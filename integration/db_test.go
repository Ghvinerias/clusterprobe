//go:build integration

package integration

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/db"
	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestDatabaseIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	postgresDSN := os.Getenv("CP_POSTGRES_DSN")
	redisAddr := os.Getenv("CP_REDIS_ADDR")
	mongoURI := os.Getenv("CP_MONGO_URI")
	mongoDB := os.Getenv("CP_MONGO_DATABASE")

	if postgresDSN == "" || redisAddr == "" || mongoURI == "" || mongoDB == "" {
		t.Skip("integration env vars not set")
	}

	maxConns := int32(4)
	if env := os.Getenv("CP_POSTGRES_MAX_CONNS"); env != "" {
		value, err := strconv.Atoi(env)
		if err != nil {
			t.Fatalf("CP_POSTGRES_MAX_CONNS: %v", err)
		}
		maxConns = int32(value)
	}

	postgresClient, err := db.NewPostgres(ctx, postgresDSN, maxConns)
	if err != nil {
		t.Fatalf("postgres new: %v", err)
	}
	defer postgresClient.Close()

	if err := postgresClient.InitSchema(ctx); err != nil {
		t.Fatalf("postgres init schema: %v", err)
	}

	if _, err := postgresClient.Exec(ctx, "INSERT INTO load_events (scenario_id, payload) VALUES ($1, $2)", "test", "{}"); err != nil {
		t.Fatalf("postgres insert: %v", err)
	}

	row := postgresClient.QueryRow(ctx, "SELECT COUNT(*) FROM load_events")
	var count int
	if err := row.Scan(&count); err != nil {
		t.Fatalf("postgres count: %v", err)
	}
	if count == 0 {
		t.Fatalf("expected load_events rows")
	}

	if _, err := postgresClient.Exec(ctx, "DELETE FROM load_events WHERE scenario_id=$1", "test"); err != nil {
		t.Fatalf("postgres delete: %v", err)
	}

	redisPassword := os.Getenv("CP_REDIS_PASSWORD")
	redisDB := 0
	if env := os.Getenv("CP_REDIS_DB"); env != "" {
		value, err := strconv.Atoi(env)
		if err != nil {
			t.Fatalf("CP_REDIS_DB: %v", err)
		}
		redisDB = value
	}

	redisClient, err := db.NewRedis(ctx, redisAddr, redisPassword, redisDB)
	if err != nil {
		t.Fatalf("redis new: %v", err)
	}
	defer func() {
		_ = redisClient.Close()
	}()

	if err := redisClient.Set(ctx, "integration:key", "value", 5*time.Second); err != nil {
		t.Fatalf("redis set: %v", err)
	}

	value, err := redisClient.Get(ctx, "integration:key")
	if err != nil {
		t.Fatalf("redis get: %v", err)
	}
	if value != "value" {
		t.Fatalf("unexpected redis value: %s", value)
	}

	if _, err := redisClient.Del(ctx, "integration:key"); err != nil {
		t.Fatalf("redis del: %v", err)
	}

	mongoClient, err := db.NewMongo(ctx, mongoURI, mongoDB)
	if err != nil {
		t.Fatalf("mongo new: %v", err)
	}
	defer func() {
		_ = mongoClient.Close(context.Background())
	}()

	if _, err := mongoClient.InsertOne(ctx, "payloads", bson.M{"_id": "integration", "value": "a"}); err != nil {
		t.Fatalf("mongo insert: %v", err)
	}

	result, err := mongoClient.FindOne(ctx, "payloads", bson.M{"_id": "integration"})
	if err != nil {
		t.Fatalf("mongo find one: %v", err)
	}

	var doc bson.M
	if err := result.Decode(&doc); err != nil {
		t.Fatalf("mongo decode: %v", err)
	}

	if _, err := mongoClient.UpdateOne(ctx, "payloads", bson.M{"_id": "integration"}, bson.M{"$set": bson.M{"value": "b"}}); err != nil {
		t.Fatalf("mongo update: %v", err)
	}

	if _, err := mongoClient.Collection("payloads").DeleteOne(ctx, bson.M{"_id": "integration"}); err != nil {
		t.Fatalf("mongo delete: %v", err)
	}
}
