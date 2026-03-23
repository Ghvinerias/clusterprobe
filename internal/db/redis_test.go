package db

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestRedisSetGetIncrDelExpire(t *testing.T) {
	server, err := miniredis.Run()
	require.NoError(t, err)
	defer server.Close()

	client, err := NewRedis(context.Background(), server.Addr(), "", 0)
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	require.NoError(t, client.Set(context.Background(), "key", "value", 0))

	value, err := client.Get(context.Background(), "key")
	require.NoError(t, err)
	require.Equal(t, "value", value)

	count, err := client.Incr(context.Background(), "counter")
	require.NoError(t, err)
	require.Equal(t, int64(1), count)

	ok, err := client.Expire(context.Background(), "key", 5*time.Second)
	require.NoError(t, err)
	require.True(t, ok)

	deleted, err := client.Del(context.Background(), "key")
	require.NoError(t, err)
	require.Equal(t, int64(1), deleted)
}

func TestRedisPublishSubscribe(t *testing.T) {
	server, err := miniredis.Run()
	require.NoError(t, err)
	defer server.Close()

	client, err := NewRedis(context.Background(), server.Addr(), "", 0)
	require.NoError(t, err)
	defer func() {
		_ = client.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pubsub, err := client.Subscribe(ctx, "events")
	require.NoError(t, err)
	defer func() {
		_ = pubsub.Close()
	}()

	_, err = client.Publish(ctx, "events", "payload")
	require.NoError(t, err)

	msg, err := pubsub.ReceiveMessage(ctx)
	require.NoError(t, err)
	require.Equal(t, "payload", msg.Payload)
}

func TestNewRedisValidation(t *testing.T) {
	client, err := NewRedis(context.Background(), "", "", 0)
	require.Error(t, err)
	require.Nil(t, client)
}

func TestConnectRedisWithRetryCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := connectRedisWithRetry(ctx, &redis.Options{Addr: "127.0.0.1:1"})
	require.Error(t, err)
	require.Nil(t, client)
}

func TestRedisErrorPaths(t *testing.T) {
	server, err := miniredis.Run()
	require.NoError(t, err)

	client, err := NewRedis(context.Background(), server.Addr(), "", 0)
	require.NoError(t, err)

	server.Close()

	_, err = client.Publish(context.Background(), "events", "payload")
	require.Error(t, err)

	_, err = client.Get(context.Background(), "missing")
	require.Error(t, err)

	err = client.Set(context.Background(), "key", "value", 0)
	require.Error(t, err)

	_, err = client.Incr(context.Background(), "counter")
	require.Error(t, err)

	_, err = client.Del(context.Background(), "key")
	require.Error(t, err)

	_, err = client.Expire(context.Background(), "key", time.Second)
	require.Error(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	pubsub, err := client.Subscribe(ctx, "events")
	require.Error(t, err)
	require.Nil(t, pubsub)

	require.NoError(t, client.Close())
}
