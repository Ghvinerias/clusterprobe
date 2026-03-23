package workload

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// CPUGenerator executes CPU-bound workloads.
type CPUGenerator struct{}

// Execute runs a tight loop for the configured duration.
func (g *CPUGenerator) Execute(ctx context.Context, params WorkloadParams) (Result, error) {
	if err := validateDuration(params); err != nil {
		return Result{}, err
	}

	_, span := otel.Tracer(workloadScope).Start(ctx, "workload.cpu")
	span.SetAttributes(spanAttributes(params)...)
	span.SetAttributes(attribute.String("workload.generator", "cpu"))
	defer span.End()

	start := time.Now()
	deadline := start.Add(time.Duration(params.DurationMs) * time.Millisecond)
	var ops int64
	var sink uint64

	for time.Now().Before(deadline) {
		for i := uint64(0); i < 1000; i++ {
			sink += i
			ops++
		}
	}

	result := Result{Ops: ops, Duration: time.Since(start)}
	if sink == 0 {
		result.Error = "cpu loop produced no work"
		err := fmt.Errorf("%s", result.Error)
		finalizeSpan(span, result, err)
		logCompletion("cpu", result, err)
		return result, err
	}

	finalizeSpan(span, result, nil)
	logCompletion("cpu", result, nil)
	return result, nil
}
