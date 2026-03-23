package workload

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

const insertLoadEventQuery = "INSERT INTO load_events (scenario_id, payload) VALUES ($1, $2)"

// DBWriteGenerator inserts synthetic rows into Postgres.
type DBWriteGenerator struct{}

// Execute writes load_events rows in batches for the configured duration.
func (g *DBWriteGenerator) Execute(ctx context.Context, params WorkloadParams) (Result, error) {
	if err := validateDuration(params); err != nil {
		return Result{}, err
	}
	if params.Store == nil {
		return Result{}, fmt.Errorf("store is required")
	}
	if params.BatchSize <= 0 {
		params.BatchSize = 10
	}

	ctx, span := otel.Tracer(workloadScope).Start(ctx, "workload.db_write")
	span.SetAttributes(spanAttributes(params)...)
	span.SetAttributes(attribute.String("workload.generator", "db_write"))
	defer span.End()

	start := time.Now()
	deadline := start.Add(time.Duration(params.DurationMs) * time.Millisecond)
	var ops int64

	for time.Now().Before(deadline) {
		payload := map[string]any{
			"id":          uuid.NewString(),
			"scenario_id": params.ScenarioID,
			"workload":    params.WorkloadType,
			"created_at":  time.Now().UTC(),
		}
		payloadBytes, err := json.Marshal(payload)
		if err != nil {
			result := Result{Ops: ops, Duration: time.Since(start), Error: err.Error()}
			finalizeSpan(span, result, err)
			logCompletion("db_write", result, err)
			return result, fmt.Errorf("marshal payload: %w", err)
		}

		for i := 0; i < params.BatchSize; i++ {
			if err := params.Store.Exec(ctx, insertLoadEventQuery, params.ScenarioID, payloadBytes); err != nil {
				result := Result{Ops: ops, Duration: time.Since(start), Error: err.Error()}
				finalizeSpan(span, result, err)
				logCompletion("db_write", result, err)
				return result, fmt.Errorf("insert load event: %w", err)
			}
			ops++
		}
	}

	result := Result{Ops: ops, Duration: time.Since(start)}
	finalizeSpan(span, result, nil)
	logCompletion("db_write", result, nil)
	return result, nil
}
