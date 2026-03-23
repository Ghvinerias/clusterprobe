package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/Ghvinerias/clusterprobe/cmd/api/handlers"
	"github.com/Ghvinerias/clusterprobe/cmd/api/middleware"
	"github.com/Ghvinerias/clusterprobe/internal/config"
	"github.com/Ghvinerias/clusterprobe/internal/db"
	"github.com/Ghvinerias/clusterprobe/internal/messaging"
	"github.com/Ghvinerias/clusterprobe/internal/telemetry"
)

const (
	serviceName = "clusterprobe-api"
)

var (
	version   = "dev"
	commitSHA = "unknown"
	buildDate = "unknown"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config load failed", "error", err)
		os.Exit(1)
	}

	shutdownTelemetry, err := telemetry.Init(ctx, telemetry.Config{
		OTLPEndpoint:   cfg.OTLPEndpoint,
		ServiceName:    serviceName,
		ServiceVersion: version,
		Environment:    "unknown",
	})
	if err != nil {
		slog.Error("telemetry init failed", "error", err)
		os.Exit(1)
	}
	defer shutdownTelemetry()

	postgresClient, err := db.NewPostgres(ctx, cfg.PostgresDSN, int32(10))
	if err != nil {
		slog.Error("postgres init failed", "error", err)
		os.Exit(1)
	}
	defer postgresClient.Close()

	redisClient, err := newRedisClient(ctx, cfg.RedisDSN)
	if err != nil {
		slog.Error("redis init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = redisClient.Close()
	}()

	mongoDB := mongoDatabaseFromURI(cfg.MongoDSN)
	mongoClient, err := db.NewMongo(ctx, cfg.MongoDSN, mongoDB)
	if err != nil {
		slog.Error("mongo init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = mongoClient.Close(context.Background())
	}()

	producer, err := messaging.NewProducer(ctx, cfg.RabbitMQURL)
	if err != nil {
		slog.Error("rabbitmq init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = producer.Close()
	}()

	if err := producer.DeclareTopology(ctx); err != nil {
		slog.Error("rabbitmq topology failed", "error", err)
		os.Exit(1)
	}

	apiRouter := buildRouter(postgresClient, redisClient, producer)

	server := &http.Server{
		Addr:              cfg.APIListenAddr,
		Handler:           apiRouter,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info(
			"api server started",
			"addr", cfg.APIListenAddr,
			"version", version,
			"commit_sha", commitSHA,
			"build_date", buildDate,
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("api server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("api server shutdown failed", "error", err)
	}
}

func buildRouter(postgresClient *db.PostgresClient, redisClient *db.RedisClient, producer *messaging.Producer) http.Handler {
	adapter := &postgresAdapter{client: postgresClient}
	redisAdapter := &redisAdapter{client: redisClient}

	scenarios := handlers.NewScenarioHandler(adapter, producer)
	status := handlers.NewStatusHandler(adapter, redisAdapter)
	chaos := handlers.NewChaosHandler(adapter, producer)

	router := handlers.NewRouter(scenarios, status, chaos)

	mux := chi.NewRouter()
	mux.Use(middleware.OTelMiddleware(serviceName))
	mux.Use(middleware.Logging)
	mux.Mount("/", router.Routes())

	return mux
}

type postgresAdapter struct {
	client *db.PostgresClient
}

func (a *postgresAdapter) Exec(ctx context.Context, sql string, args ...any) error {
	_, err := a.client.Exec(ctx, sql, args...)
	if err != nil {
		return fmt.Errorf("postgres exec: %w", err)
	}
	return nil
}

func (a *postgresAdapter) Query(ctx context.Context, sql string, args ...any) (handlers.Rows, error) {
	rows, err := a.client.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres query: %w", err)
	}
	return rowsAdapter{rows: rows}, nil
}

func (a *postgresAdapter) QueryRow(ctx context.Context, sql string, args ...any) handlers.Row {
	return a.client.QueryRow(ctx, sql, args...)
}

type rowsAdapter struct {
	rows interface {
		Close()
		Err() error
		Next() bool
		Scan(dest ...any) error
	}
}

func (r rowsAdapter) Close()                 { r.rows.Close() }
func (r rowsAdapter) Err() error             { return r.rows.Err() }
func (r rowsAdapter) Next() bool             { return r.rows.Next() }
func (r rowsAdapter) Scan(dest ...any) error { return r.rows.Scan(dest...) }

type redisAdapter struct {
	client *db.RedisClient
}

func (r *redisAdapter) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key)
}

func newRedisClient(ctx context.Context, dsn string) (*db.RedisClient, error) {
	if strings.HasPrefix(dsn, "redis://") || strings.HasPrefix(dsn, "rediss://") {
		options, err := redis.ParseURL(dsn)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}
		return db.NewRedis(ctx, options.Addr, options.Password, options.DB)
	}

	return db.NewRedis(ctx, dsn, "", 0)
}

func mongoDatabaseFromURI(uri string) string {
	parsed, err := url.Parse(uri)
	if err != nil {
		return "clusterprobe"
	}
	if parsed.Path == "" || parsed.Path == "/" {
		return "clusterprobe"
	}
	return strings.TrimPrefix(parsed.Path, "/")
}
