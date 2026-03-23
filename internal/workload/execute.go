package workload

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

const workloadScope = "clusterprobe/workload"

// Row defines the minimal interface for database row scanning.
type Row interface {
	Scan(dest ...any) error
}

// Rows defines the minimal interface for iterating rows.
type Rows interface {
	Close()
	Err() error
	Next() bool
	Scan(dest ...any) error
}

// SQLStore defines the minimal database operations needed by workloads.
type SQLStore interface {
	Exec(ctx context.Context, sql string, args ...any) error
	Query(ctx context.Context, sql string, args ...any) (Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) Row
}

// MixedProfile configures workload ratios for mixed workloads.
type MixedProfile struct {
	CPUPercent     int
	DBWritePercent int
	DBReadPercent  int
}

// WorkloadParams defines shared parameters for workload generators.
type WorkloadParams struct {
	ScenarioID     string
	WorkloadType   WorkloadType
	DurationMs     int64
	AllocMB        int
	BatchSize      int
	ReadLookbackMs int64
	MixedProfile   MixedProfile
	Store          SQLStore
}

// Result captures workload execution output.
type Result struct {
	Ops      int64
	Duration time.Duration
	Error    string
}

// Generator defines the workload generator interface.
type Generator interface {
	Execute(ctx context.Context, params WorkloadParams) (Result, error)
}

func tracer() metric.Meter {
	return otel.Meter(workloadScope)
}

func spanAttributes(params WorkloadParams) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("workload.type", string(params.WorkloadType)),
		attribute.Int64("workload.duration_ms", params.DurationMs),
		attribute.Int("workload.alloc_mb", params.AllocMB),
		attribute.Int("workload.batch_size", params.BatchSize),
		attribute.Int64("workload.read_lookback_ms", params.ReadLookbackMs),
	}
}

func finalizeSpan(span trace.Span, result Result, err error) {
	span.SetAttributes(
		attribute.Int64("workload.ops", result.Ops),
		attribute.Int64("workload.elapsed_ms", result.Duration.Milliseconds()),
	)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "workload failed")
	}
}

func logCompletion(name string, result Result, err error) {
	logger := slog.With("workload", name, "ops", result.Ops, "duration_ms", result.Duration.Milliseconds())
	if err != nil {
		logger.Error("workload failed", "error", err)
		return
	}
	logger.Info("workload completed")
}

func validateDuration(params WorkloadParams) error {
	if params.DurationMs <= 0 {
		return fmt.Errorf("duration_ms must be greater than zero")
	}
	return nil
}
