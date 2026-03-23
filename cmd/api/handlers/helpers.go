package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/Ghvinerias/clusterprobe/internal/chaos"
)

// Row abstracts database scanning.
type Row interface {
	Scan(dest ...any) error
}

// Rows abstracts database row iteration.
type Rows interface {
	Close()
	Err() error
	Next() bool
	Scan(dest ...any) error
}

// PostgresStore defines minimal DB operations.
type PostgresStore interface {
	Exec(ctx context.Context, sql string, args ...any) error
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// RedisStore defines minimal Redis operations.
type RedisStore interface {
	Get(ctx context.Context, key string) (string, error)
}

// Publisher publishes messages to RabbitMQ.
type Publisher interface {
	Publish(ctx context.Context, exchange, routingKey string, body []byte) error
}

// ChaosEngine manages Chaos Mesh experiments.
type ChaosEngine interface {
	Apply(ctx context.Context, spec chaos.ExperimentSpec) (string, error)
	Status(ctx context.Context, id string) (chaos.ExperimentStatus, error)
	Delete(ctx context.Context, id string) error
	List(ctx context.Context) ([]chaos.ExperimentStatus, error)
}

// MongoCursor abstracts Mongo cursor iteration.
type MongoCursor interface {
	Next(ctx context.Context) bool
	Decode(val any) error
	Err() error
	Close(ctx context.Context) error
}

// MongoStore defines minimal MongoDB operations.
type MongoStore interface {
	InsertOne(ctx context.Context, collection string, doc any) error
	Find(ctx context.Context, collection string, filter any) (MongoCursor, error)
}

func writeJSON(w http.ResponseWriter, status int, payload any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(payload); err != nil {
		return fmt.Errorf("encode json: %w", err)
	}
	return nil
}

func decodeJSON(r *http.Request, dest any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dest); err != nil {
		return fmt.Errorf("decode json: %w", err)
	}
	return nil
}

func errorResponse(w http.ResponseWriter, status int, message string) {
	_ = writeJSON(w, status, map[string]string{"error": message})
}
