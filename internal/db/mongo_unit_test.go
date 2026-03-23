//go:build !integration

package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.opentelemetry.io/otel"
)

func TestNewMongoValidation(t *testing.T) {
	t.Run("empty uri", func(t *testing.T) {
		client, err := NewMongo(context.Background(), "", "db")
		require.Error(t, err)
		require.Nil(t, client)
	})

	t.Run("empty database", func(t *testing.T) {
		client, err := NewMongo(context.Background(), "mongodb://localhost:27017", "")
		require.Error(t, err)
		require.Nil(t, client)
	})
}

func TestConnectMongoWithRetryCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	client, err := connectMongoWithRetry(ctx, "mongodb://localhost:27017")
	require.Error(t, err)
	require.Nil(t, client)
}

func TestMongoClientOperationsErrorPaths(t *testing.T) {
	raw, err := mongo.Connect(
		options.Client().
			ApplyURI("mongodb://127.0.0.1:1").
			SetServerSelectionTimeout(50*time.Millisecond),
	)
	require.NoError(t, err)

	client := &MongoClient{
		client:   raw,
		database: "clusterprobe",
		tracer:   otel.Tracer("test"),
	}

	t.Run("collection", func(t *testing.T) {
		require.NotNil(t, client.Collection("payloads"))
	})

	t.Run("insert one", func(t *testing.T) {
		_, err := client.InsertOne(context.Background(), "payloads", map[string]any{"a": 1})
		require.Error(t, err)
	})

	t.Run("insert many", func(t *testing.T) {
		_, err := client.InsertMany(context.Background(), "payloads", []any{map[string]any{"a": 1}})
		require.Error(t, err)
	})

	t.Run("find", func(t *testing.T) {
		_, err := client.Find(context.Background(), "payloads", map[string]any{})
		require.Error(t, err)
	})

	t.Run("find one", func(t *testing.T) {
		_, err := client.FindOne(context.Background(), "payloads", map[string]any{})
		require.Error(t, err)
	})

	t.Run("update one", func(t *testing.T) {
		_, err := client.UpdateOne(context.Background(), "payloads", map[string]any{}, map[string]any{"$set": map[string]any{"a": 1}})
		require.Error(t, err)
	})

	t.Run("close", func(t *testing.T) {
		require.NoError(t, client.Close(context.Background()))
	})
}
