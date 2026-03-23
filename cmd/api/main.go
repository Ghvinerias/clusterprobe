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
	"github.com/Ghvinerias/clusterprobe/internal/chaos"
	"github.com/Ghvinerias/clusterprobe/internal/config"
	"github.com/Ghvinerias/clusterprobe/internal/db"
	"github.com/Ghvinerias/clusterprobe/internal/messaging"
	"github.com/Ghvinerias/clusterprobe/internal/telemetry"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
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

	chaosClient, err := newChaosClient()
	if err != nil {
		slog.Error("chaos client init failed", "error", err)
		os.Exit(1)
	}

	apiRouter := buildRouter(postgresClient, redisClient, mongoClient, producer, chaosClient)

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

func buildRouter(
	postgresClient *db.PostgresClient,
	redisClient *db.RedisClient,
	mongoClient *db.MongoClient,
	producer *messaging.Producer,
	chaosClient *chaos.ChaosClient,
) http.Handler {
	adapter := &postgresAdapter{client: postgresClient}
	redisAdapter := &redisAdapter{client: redisClient}
	mongoAdapter := &mongoAdapter{client: mongoClient}

	scenarios := handlers.NewScenarioHandler(adapter, producer)
	status := handlers.NewStatusHandler(adapter, redisAdapter)
	chaos := handlers.NewChaosHandler(mongoAdapter, chaosClient)

	router := handlers.NewRouter(scenarios, status, chaos)

	mux := chi.NewRouter()
	mux.Use(middleware.OTelMiddleware(serviceName))
	mux.Use(middleware.Logging)
	mux.Mount("/", router.Routes())

	return mux
}

func newChaosClient() (*chaos.ChaosClient, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in cluster config: %w", err)
	}
	client, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}
	return chaos.NewChaosClient(client, "cluster-probe"), nil
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

func (r rowsAdapter) Close() { r.rows.Close() }
func (r rowsAdapter) Err() error {
	if err := r.rows.Err(); err != nil {
		return fmt.Errorf("rows err: %w", err)
	}
	return nil
}

func (r rowsAdapter) Next() bool { return r.rows.Next() }

func (r rowsAdapter) Scan(dest ...any) error {
	if err := r.rows.Scan(dest...); err != nil {
		return fmt.Errorf("rows scan: %w", err)
	}
	return nil
}

type redisAdapter struct {
	client *db.RedisClient
}

func (r *redisAdapter) Get(ctx context.Context, key string) (string, error) {
	value, err := r.client.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("redis get: %w", err)
	}
	return value, nil
}

func newRedisClient(ctx context.Context, dsn string) (*db.RedisClient, error) {
	if strings.HasPrefix(dsn, "redis://") || strings.HasPrefix(dsn, "rediss://") {
		options, err := redis.ParseURL(dsn)
		if err != nil {
			return nil, fmt.Errorf("parse redis url: %w", err)
		}
		client, err := db.NewRedis(ctx, options.Addr, options.Password, options.DB)
		if err != nil {
			return nil, fmt.Errorf("redis client: %w", err)
		}
		return client, nil
	}

	client, err := db.NewRedis(ctx, dsn, "", 0)
	if err != nil {
		return nil, fmt.Errorf("redis client: %w", err)
	}
	return client, nil
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

type mongoAdapter struct {
	client *db.MongoClient
}

func (m *mongoAdapter) InsertOne(ctx context.Context, collection string, doc any) error {
	if _, err := m.client.InsertOne(ctx, collection, doc); err != nil {
		return fmt.Errorf("mongo insert: %w", err)
	}
	return nil
}

func (m *mongoAdapter) Find(ctx context.Context, collection string, filter any) (handlers.MongoCursor, error) {
	cursor, err := m.client.Find(ctx, collection, filter)
	if err != nil {
		return nil, fmt.Errorf("mongo find: %w", err)
	}
	return mongoCursor{cursor: cursor}, nil
}

type mongoCursor struct {
	cursor interface {
		Next(ctx context.Context) bool
		Decode(val any) error
		Err() error
		Close(ctx context.Context) error
	}
}

func (m mongoCursor) Next(ctx context.Context) bool { return m.cursor.Next(ctx) }

func (m mongoCursor) Decode(val any) error {
	if err := m.cursor.Decode(val); err != nil {
		return fmt.Errorf("mongo decode: %w", err)
	}
	return nil
}

func (m mongoCursor) Err() error {
	if err := m.cursor.Err(); err != nil {
		return fmt.Errorf("mongo cursor err: %w", err)
	}
	return nil
}

func (m mongoCursor) Close(ctx context.Context) error {
	if err := m.cursor.Close(ctx); err != nil {
		return fmt.Errorf("mongo cursor close: %w", err)
	}
	return nil
}
