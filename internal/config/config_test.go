package config

import (
	"strings"
	"testing"
)

func TestLoadConfigSuccess(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.PostgresDSN != "postgres://user:pass@localhost:5432/db" {
		t.Fatalf("unexpected postgres dsn: %s", cfg.PostgresDSN)
	}
	if cfg.WorkerConcurrency != 8 {
		t.Fatalf("unexpected worker concurrency: %d", cfg.WorkerConcurrency)
	}
}

func TestLoadConfigMissing(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CP_RABBITMQ_URL", "")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected missing config error")
	}
	if !strings.Contains(err.Error(), "CP_RABBITMQ_URL") {
		t.Fatalf("expected missing key in error: %v", err)
	}
}

func TestLoadConfigInvalidConcurrency(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("CP_WORKER_CONCURRENCY", "not-a-number")

	_, err := Load()
	if err == nil {
		t.Fatalf("expected concurrency error")
	}
	if !strings.Contains(err.Error(), "CP_WORKER_CONCURRENCY") {
		t.Fatalf("expected concurrency key in error: %v", err)
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("CP_POSTGRES_DSN", "postgres://user:pass@localhost:5432/db")
	t.Setenv("CP_REDIS_DSN", "redis://localhost:6379/0")
	t.Setenv("CP_MONGO_DSN", "mongodb://localhost:27017")
	t.Setenv("CP_RABBITMQ_URL", "amqp://guest:guest@localhost:5672/")
	t.Setenv("CP_OTEL_ENDPOINT", "127.0.0.1:4317")
	t.Setenv("CP_API_LISTEN_ADDR", ":8080")
	t.Setenv("CP_UI_LISTEN_ADDR", ":8081")
	t.Setenv("CP_CHAOS_LISTEN_ADDR", ":8082")
	t.Setenv("CP_WORKER_CONCURRENCY", "8")
	t.Setenv("CP_LOG_LEVEL", "info")
}
