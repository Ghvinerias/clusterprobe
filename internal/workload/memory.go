package workload

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// MemoryGenerator executes memory-bound workloads.
type MemoryGenerator struct{}

// Execute allocates memory for the configured duration.
func (g *MemoryGenerator) Execute(ctx context.Context, params WorkloadParams) (Result, error) {
	if err := validateDuration(params); err != nil {
		return Result{}, err
	}
	if params.AllocMB <= 0 {
		params.AllocMB = 1
	}

	ctx, span := otel.Tracer(workloadScope).Start(ctx, "workload.memory")
	span.SetAttributes(spanAttributes(params)...)
	span.SetAttributes(attribute.String("workload.generator", "memory"))
	defer span.End()

	start := time.Now()
	buf := make([]byte, params.AllocMB*1024*1024)
	for i := 0; i < len(buf); i += 4096 {
		buf[i] = byte(i % 256)
	}

	select {
	case <-ctx.Done():
	case <-time.After(time.Duration(params.DurationMs) * time.Millisecond):
	}

	runtime.KeepAlive(buf)
	result := Result{Ops: int64(len(buf)), Duration: time.Since(start)}
	finalizeSpan(span, result, ctx.Err())
	logCompletion("memory", result, ctx.Err())
	if err := ctx.Err(); err != nil {
		return result, fmt.Errorf("context: %w", err)
	}
	return result, nil
}
