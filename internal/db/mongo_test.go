//go:build integration

package db

import (
	"context"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoClientIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := mongodb.RunContainer(ctx)
	if err != nil {
		t.Fatalf("start mongo container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	uri, err := container.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("connection string: %v", err)
	}

	client, err := NewMongo(ctx, uri, "clusterprobe")
	if err != nil {
		t.Fatalf("new mongo: %v", err)
	}
	defer func() {
		_ = client.Close(context.Background())
	}()

	if _, err := client.InsertOne(ctx, "payloads", bson.M{"_id": "1", "value": "a"}); err != nil {
		t.Fatalf("insert one: %v", err)
	}

	result, err := client.FindOne(ctx, "payloads", bson.M{"_id": "1"})
	if err != nil {
		t.Fatalf("find one: %v", err)
	}

	var doc bson.M
	if err := result.Decode(&doc); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if _, err := client.UpdateOne(ctx, "payloads", bson.M{"_id": "1"}, bson.M{"$set": bson.M{"value": "b"}}); err != nil {
		t.Fatalf("update one: %v", err)
	}

	cursor, err := client.Find(ctx, "payloads", bson.M{})
	if err != nil {
		t.Fatalf("find: %v", err)
	}
	if err := cursor.Close(ctx); err != nil {
		t.Fatalf("close cursor: %v", err)
	}
}
