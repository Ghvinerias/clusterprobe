package workload

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const selectLoadEventQuery = "SELECT payload FROM load_events WHERE created_at > $1 ORDER BY created_at DESC LIMIT 1"

// DBReadGenerator reads load_events rows and records latency.
type DBReadGenerator struct {
	latency metric.Float64Histogram
}

// Execute runs SELECT queries for the configured duration.
func (g *DBReadGenerator) Execute(ctx context.Context, params WorkloadParams) (Result, error) {
	if err := validateDuration(params); err != nil {
		return Result{}, err
	}
	if params.Store == nil {
		return Result{}, fmt.Errorf("store is required")
	}
	if params.ReadLookbackMs <= 0 {
		params.ReadLookbackMs = int64(5 * time.Minute / time.Millisecond)
	}

	ctx, span := otel.Tracer(workloadScope).Start(ctx, "workload.db_read")
	span.SetAttributes(spanAttributes(params)...)
	span.SetAttributes(attribute.String("workload.generator", "db_read"))
	defer span.End()

	if g.latency == nil {
		meter := tracer()
		hist, err := meter.Float64Histogram("db_read_latency_ms")
		if err != nil {
			return Result{}, fmt.Errorf("create latency histogram: %w", err)
		}
		g.latency = hist
	}

	rng := rand.New(rand.NewSource(time.Now().UnixNano()))
	start := time.Now()
	deadline := start.Add(time.Duration(params.DurationMs) * time.Millisecond)
	var ops int64

	for time.Now().Before(deadline) {
		jitter := time.Duration(rng.Int63n(params.ReadLookbackMs)) * time.Millisecond
		since := time.Now().Add(-jitter)

		queryStart := time.Now()
		var payload []byte
		if err := params.Store.QueryRow(ctx, selectLoadEventQuery, since).Scan(&payload); err != nil {
			result := Result{Ops: ops, Duration: time.Since(start), Error: err.Error()}
			finalizeSpan(span, result, err)
			logCompletion("db_read", result, err)
			return result, fmt.Errorf("select load event: %w", err)
		}
		elapsed := time.Since(queryStart)
		g.latency.Record(ctx, float64(elapsed.Milliseconds()))
		ops++
	}

	result := Result{Ops: ops, Duration: time.Since(start)}
	finalizeSpan(span, result, nil)
	logCompletion("db_read", result, nil)
	return result, nil
}
