package workload

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
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

	start := time.Now()
	deadline := start.Add(time.Duration(params.DurationMs) * time.Millisecond)
	var ops int64

	for time.Now().Before(deadline) {
		jitter, err := randomDuration(params.ReadLookbackMs)
		if err != nil {
			result := Result{Ops: ops, Duration: time.Since(start), Error: err.Error()}
			finalizeSpan(span, result, err)
			logCompletion("db_read", result, err)
			return result, fmt.Errorf("jitter: %w", err)
		}
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

func randomDuration(maxMs int64) (time.Duration, error) {
	if maxMs <= 0 {
		return 0, fmt.Errorf("maxMs must be positive")
	}
	n, err := rand.Int(rand.Reader, big.NewInt(maxMs))
	if err != nil {
		return 0, fmt.Errorf("rand: %w", err)
	}
	return time.Duration(n.Int64()) * time.Millisecond, nil
}
