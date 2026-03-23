package workload

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
)

// MixedGenerator orchestrates CPU and DB workloads.
type MixedGenerator struct {
	CPU     Generator
	DBWrite Generator
	DBRead  Generator
}

// Execute runs CPU, DB write, and DB read workloads based on ratio.
func (g *MixedGenerator) Execute(ctx context.Context, params WorkloadParams) (Result, error) {
	if err := validateDuration(params); err != nil {
		return Result{}, err
	}
	if g.CPU == nil || g.DBWrite == nil || g.DBRead == nil {
		return Result{}, fmt.Errorf("mixed generator requires cpu, db_write, and db_read generators")
	}

	ctx, span := otel.Tracer(workloadScope).Start(ctx, "workload.mixed")
	span.SetAttributes(spanAttributes(params)...)
	span.SetAttributes(attribute.String("workload.generator", "mixed"))
	defer span.End()

	start := time.Now()
	profile := params.MixedProfile
	if profile.CPUPercent+profile.DBWritePercent+profile.DBReadPercent == 0 {
		profile = MixedProfile{CPUPercent: 34, DBWritePercent: 33, DBReadPercent: 33}
	}

	base := time.Duration(params.DurationMs) * time.Millisecond
	cpud := base * time.Duration(profile.CPUPercent) / 100
	writeD := base * time.Duration(profile.DBWritePercent) / 100
	readD := base * time.Duration(profile.DBReadPercent) / 100
	remaining := base - (cpud + writeD + readD)
	cpud += remaining

	result := Result{}
	cpuParams := params
	cpuParams.DurationMs = cpud.Milliseconds()
	cpuResult, err := g.CPU.Execute(ctx, cpuParams)
	result.Ops += cpuResult.Ops
	if err != nil {
		result.Duration = time.Since(start)
		result.Error = err.Error()
		finalizeSpan(span, result, err)
		logCompletion("mixed", result, err)
		return result, err
	}

	writeParams := params
	writeParams.DurationMs = writeD.Milliseconds()
	writeResult, err := g.DBWrite.Execute(ctx, writeParams)
	result.Ops += writeResult.Ops
	if err != nil {
		result.Duration = time.Since(start)
		result.Error = err.Error()
		finalizeSpan(span, result, err)
		logCompletion("mixed", result, err)
		return result, err
	}

	readParams := params
	readParams.DurationMs = readD.Milliseconds()
	readResult, err := g.DBRead.Execute(ctx, readParams)
	result.Ops += readResult.Ops
	if err != nil {
		result.Duration = time.Since(start)
		result.Error = err.Error()
		finalizeSpan(span, result, err)
		logCompletion("mixed", result, err)
		return result, err
	}

	result.Duration = time.Since(start)
	finalizeSpan(span, result, nil)
	logCompletion("mixed", result, nil)
	return result, nil
}
