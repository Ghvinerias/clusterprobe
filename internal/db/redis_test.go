package db

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
)

func TestRedisSetGetIncrDelExpire(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer server.Close()

	client, err := NewRedis(context.Background(), server.Addr(), "", 0)
	if err != nil {
		t.Fatalf("new redis: %v", err)
	}
	defer func() {
		_ = client.Close()
	}()

	if err := client.Set(context.Background(), "key", "value", 0); err != nil {
		t.Fatalf("set: %v", err)
	}

	value, err := client.Get(context.Background(), "key")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if value != "value" {
		t.Fatalf("unexpected value: %s", value)
	}

	count, err := client.Incr(context.Background(), "counter")
	if err != nil {
		t.Fatalf("incr: %v", err)
	}
	if count != 1 {
		t.Fatalf("unexpected count: %d", count)
	}

	ok, err := client.Expire(context.Background(), "key", 5*time.Second)
	if err != nil {
		t.Fatalf("expire: %v", err)
	}
	if !ok {
		t.Fatalf("expected expire true")
	}

	deleted, err := client.Del(context.Background(), "key")
	if err != nil {
		t.Fatalf("del: %v", err)
	}
	if deleted != 1 {
		t.Fatalf("unexpected deleted count: %d", deleted)
	}
}

func TestRedisPublishSubscribe(t *testing.T) {
	server, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer server.Close()

	client, err := NewRedis(context.Background(), server.Addr(), "", 0)
	if err != nil {
		t.Fatalf("new redis: %v", err)
	}
	defer func() {
		_ = client.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pubsub, err := client.Subscribe(ctx, "events")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	defer func() {
		_ = pubsub.Close()
	}()

	if _, err := client.Publish(ctx, "events", "payload"); err != nil {
		t.Fatalf("publish: %v", err)
	}

	msg, err := pubsub.ReceiveMessage(ctx)
	if err != nil {
		t.Fatalf("receive: %v", err)
	}
	if msg.Payload != "payload" {
		t.Fatalf("unexpected payload: %s", msg.Payload)
	}
}
