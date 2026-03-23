package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"

	"github.com/Ghvinerias/clusterprobe/internal/config"
	"github.com/Ghvinerias/clusterprobe/internal/db"
	"github.com/Ghvinerias/clusterprobe/internal/messaging"
	"github.com/Ghvinerias/clusterprobe/internal/telemetry"
	"github.com/Ghvinerias/clusterprobe/internal/workload"
)

const (
	serviceName = "clusterprobe-worker"
	queueHigh   = "workload.high"
	queueLow    = "workload.low"
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

	metricsHandler, shutdownMetrics, err := setupPrometheusMetrics(ctx, cfg)
	if err != nil {
		slog.Error("metrics setup failed", "error", err)
		os.Exit(1)
	}
	defer shutdownMetrics()

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

	producer, err := messaging.NewProducer(ctx, cfg.RabbitMQURL)
	if err != nil {
		slog.Error("rabbitmq producer init failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		_ = producer.Close()
	}()

	workerCount := cfg.WorkerConcurrency
	if workerCount <= 0 {
		workerCount = 10
	}

	store := &postgresAdapter{client: postgresClient}
	redis := &redisAdapter{client: redisClient}

	generators := buildGenerators(store)

	server := &http.Server{
		Addr:              cfg.ChaosListenAddr,
		Handler:           buildHTTPServer(metricsHandler),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		slog.Info(
			"worker metrics server started",
			"addr", cfg.ChaosListenAddr,
			"version", version,
			"commit_sha", commitSHA,
			"build_date", buildDate,
		)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("worker metrics server error", "error", err)
			stop()
		}
	}()

	consumerErrCh := make(chan error, 1)
	go func() {
		queuePicker := func(i int) string {
			if i%2 == 0 {
				return queueHigh
			}
			return queueLow
		}
		consumerFactory := func() (consumer, error) {
			return messaging.NewConsumer(ctx, cfg.RabbitMQURL, 1)
		}
		handler := func(msgCtx context.Context, msg amqp.Delivery) error {
			workCtx := context.WithoutCancel(msgCtx)
			return handleMessage(workCtx, msg, generators, store, redis, producer)
		}
		consumerErrCh <- startConsumers(ctx, workerCount, queuePicker, consumerFactory, handler)
	}()

	<-ctx.Done()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("metrics server shutdown failed", "error", err)
	}

	if err := <-consumerErrCh; err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("consumer pool stopped", "error", err)
	}
}

func buildHTTPServer(metricsHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/metrics", metricsHandler)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return mux
}

func setupPrometheusMetrics(ctx context.Context, cfg config.Config) (http.Handler, func(), error) {
	metricExporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("create otlp metric exporter: %w", err)
	}

	promExporter, err := prometheus.New()
	if err != nil {
		return nil, nil, fmt.Errorf("create prometheus exporter: %w", err)
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(serviceName),
			semconv.ServiceVersionKey.String(version),
		),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("build resource: %w", err)
	}

	provider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
		metric.WithReader(promExporter),
		metric.WithResource(res),
	)
	otel.SetMeterProvider(provider)

	shutdown := func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := provider.Shutdown(shutdownCtx); err != nil {
			slog.Error("meter provider shutdown failed", "error", err)
		}
	}

	return promhttp.Handler(), shutdown, nil
}

func buildGenerators(store workload.SQLStore) map[workload.WorkloadType]workload.Generator {
	cpuGen := &workload.CPUGenerator{}
	memGen := &workload.MemoryGenerator{}
	writeGen := &workload.DBWriteGenerator{}
	readGen := &workload.DBReadGenerator{}
	mixedGen := &workload.MixedGenerator{CPU: cpuGen, DBWrite: writeGen, DBRead: readGen}

	return map[workload.WorkloadType]workload.Generator{
		workload.WorkloadTypeCPUBurn:  cpuGen,
		workload.WorkloadTypeMemAlloc: memGen,
		workload.WorkloadTypeDBWrite:  writeGen,
		workload.WorkloadTypeDBRead:   readGen,
		workload.WorkloadTypeMixed:    mixedGen,
	}
}

func handleMessage(
	ctx context.Context,
	msg amqp.Delivery,
	gens map[workload.WorkloadType]workload.Generator,
	store workload.SQLStore,
	redis redisCounter,
	producer messagePublisher,
) error {
	var scenario workload.ScenarioResponse
	if err := json.Unmarshal(msg.Body, &scenario); err != nil {
		return messaging.UnrecoverableError{Err: fmt.Errorf("decode scenario: %w", err)}
	}

	gen, ok := gens[scenario.Profile.WorkloadType]
	if !ok {
		return messaging.UnrecoverableError{Err: fmt.Errorf("unknown workload type: %s", scenario.Profile.WorkloadType)}
	}

	allocMB := scenario.Profile.PayloadSizeBytes / (1024 * 1024)
	if allocMB <= 0 {
		allocMB = 1
	}

	params := workload.WorkloadParams{
		ScenarioID:     scenario.ID,
		WorkloadType:   scenario.Profile.WorkloadType,
		DurationMs:     scenario.Profile.Duration.Milliseconds(),
		AllocMB:        allocMB,
		BatchSize:      scenario.Profile.Concurrency,
		ReadLookbackMs: int64(5 * time.Minute / time.Millisecond),
		Store:          store,
	}

	result, err := gen.Execute(ctx, params)
	if err != nil {
		result.Error = err.Error()
	}

	if reportErr := reportResult(ctx, scenario, result, redis, producer, store); reportErr != nil {
		return fmt.Errorf("report result: %w", reportErr)
	}

	return nil
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

func (a *postgresAdapter) Query(ctx context.Context, sql string, args ...any) (workload.Rows, error) {
	rows, err := a.client.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("postgres query: %w", err)
	}
	return rowsAdapter{rows: rows}, nil
}

func (a *postgresAdapter) QueryRow(ctx context.Context, sql string, args ...any) workload.Row {
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

func (r *redisAdapter) Incr(ctx context.Context, key string) error {
	if _, err := r.client.Incr(ctx, key); err != nil {
		return fmt.Errorf("redis incr: %w", err)
	}
	return nil
}

type redisCounter interface {
	Incr(ctx context.Context, key string) error
}

type messagePublisher interface {
	Publish(ctx context.Context, exchange, routingKey string, body []byte) error
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

type consumer interface {
	Consume(ctx context.Context, queue string, handler func(context.Context, amqp.Delivery) error) error
}

func startConsumers(
	ctx context.Context,
	workerCount int,
	queuePicker func(int) string,
	consumerFactory func() (consumer, error),
	handler func(context.Context, amqp.Delivery) error,
) error {
	var wg sync.WaitGroup
	errCh := make(chan error, workerCount)

	for i := 0; i < workerCount; i++ {
		queue := queuePicker(i)
		c, err := consumerFactory()
		if err != nil {
			return fmt.Errorf("create consumer: %w", err)
		}

		wg.Add(1)
		go func(queue string, c consumer) {
			defer wg.Done()
			if err := c.Consume(ctx, queue, handler); err != nil && !errors.Is(err, context.Canceled) {
				errCh <- fmt.Errorf("consume %s: %w", queue, err)
			}
		}(queue, c)
	}

	<-ctx.Done()
	wg.Wait()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}
