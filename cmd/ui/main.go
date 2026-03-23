package main

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	"github.com/Ghvinerias/clusterprobe/cmd/ui/handlers"
	"github.com/Ghvinerias/clusterprobe/cmd/ui/middleware"
	"github.com/Ghvinerias/clusterprobe/internal/config"
	"github.com/Ghvinerias/clusterprobe/internal/db"
	"github.com/Ghvinerias/clusterprobe/internal/telemetry"
	"github.com/Ghvinerias/clusterprobe/web"
)

const serviceName = "clusterprobe-ui"

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

	redisClient, err := newRedisClient(ctx, cfg.RedisDSN)
	if err != nil {
		slog.Error("redis init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = redisClient.Close()
	}()

	apiBaseURL := os.Getenv("CP_API_BASE_URL")
	api := newAPIClient(apiBaseURL)

	tmpls, err := parseTemplates()
	if err != nil {
		slog.Error("template load failed", "error", err)
		os.Exit(1)
	}

	server := handlers.NewServer(tmpls, api, &redisAdapter{client: redisClient}, slog.Default())

	router := chi.NewRouter()
	router.Use(middleware.OTelMiddleware(serviceName))
	router.Use(middleware.Logging)

	router.Get("/", server.Root)
	router.Get("/dashboard", server.Dashboard)
	router.Get("/dashboard/stream", server.DashboardStream)
	router.Get("/scenarios", server.ListScenarios)
	router.Get("/scenarios/new", server.NewScenario)
	router.Post("/scenarios", server.CreateScenario)
	router.Post("/scenarios/{id}/stop", server.StopScenario)
	router.Get("/chaos", server.ListChaos)
	router.Get("/chaos/new", server.NewChaos)
	router.Post("/chaos", server.CreateChaos)
	router.Get("/logs", server.Logs)
	router.Get("/logs/stream", server.LogsStream)

	staticFS, err := fs.Sub(web.FS, "static")
	if err != nil {
		slog.Error("static assets missing", "error", err)
		os.Exit(1)
	}
	router.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	httpServer := &http.Server{
		Addr:              cfg.UIListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info(
			"ui server started",
			"addr", cfg.UIListenAddr,
			"version", version,
			"commit_sha", commitSHA,
			"build_date", buildDate,
		)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("ui server error", "error", err)
			stop()
		}
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		slog.Error("ui server shutdown failed", "error", err)
	}
}

func parseTemplates() (*template.Template, error) {
	return template.ParseFS(
		web.FS,
		"templates/base.html",
		"templates/pages/*.html",
		"templates/partials/*.html",
	)
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
