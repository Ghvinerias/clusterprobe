package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	"go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
)

// Config defines the telemetry bootstrap configuration.
type Config struct {
	OTLPEndpoint   string
	ServiceName    string
	ServiceVersion string
	Environment    string
}

// Init configures OTEL tracing, metrics, and logging, returning a shutdown function.
func Init(ctx context.Context, cfg Config) (func(), error) {
	if cfg.OTLPEndpoint == "" {
		return nil, fmt.Errorf("otel endpoint is required")
	}

	res, err := resource.New(
		ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.ServiceVersionKey.String(cfg.ServiceVersion),
			semconv.DeploymentEnvironmentKey.String(cfg.Environment),
		),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	traceExporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create trace exporter: %w", err)
	}

	metricExporter, err := otlpmetricgrpc.New(
		ctx,
		otlpmetricgrpc.WithEndpoint(cfg.OTLPEndpoint),
		otlpmetricgrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("create metric exporter: %w", err)
	}

	tracerProvider := trace.NewTracerProvider(
		trace.WithBatcher(traceExporter),
		trace.WithResource(res),
	)

	meterProvider := metric.NewMeterProvider(
		metric.WithReader(metric.NewPeriodicReader(metricExporter)),
		metric.WithResource(res),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetMeterProvider(meterProvider)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	handler := otelslog.NewHandler(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))
	logger := slog.New(handler).With(
		"service", cfg.ServiceName,
		"phase", "unknown",
	)
	slog.SetDefault(logger)

	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := tracerProvider.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shutdown tracer provider", "error", err)
		}
		if err := meterProvider.Shutdown(shutdownCtx); err != nil {
			slog.Error("failed to shutdown meter provider", "error", err)
		}
	}, nil
}
