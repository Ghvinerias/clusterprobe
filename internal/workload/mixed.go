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

type mixedDurations struct {
	cpuMs   int64
	writeMs int64
	readMs  int64
}

type mixedPhase struct {
	name    string
	percent int
	ms      int64
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
	durations, err := splitMixedDurations(params.DurationMs, profile)
	if err != nil {
		finalizeSpan(span, Result{Duration: time.Since(start), Error: err.Error()}, err)
		logCompletion("mixed", Result{Duration: time.Since(start), Error: err.Error()}, err)
		return Result{}, err
	}

	result := Result{}
	if durations.cpuMs > 0 {
		cpuParams := params
		cpuParams.DurationMs = durations.cpuMs
		cpuResult, err := g.CPU.Execute(ctx, cpuParams)
		result.Ops += cpuResult.Ops
		if err != nil {
			result.Duration = time.Since(start)
			result.Error = err.Error()
			finalizeSpan(span, result, err)
			logCompletion("mixed", result, err)
			return result, fmt.Errorf("cpu workload: %w", err)
		}
	}

	if durations.writeMs > 0 {
		writeParams := params
		writeParams.DurationMs = durations.writeMs
		writeResult, err := g.DBWrite.Execute(ctx, writeParams)
		result.Ops += writeResult.Ops
		if err != nil {
			result.Duration = time.Since(start)
			result.Error = err.Error()
			finalizeSpan(span, result, err)
			logCompletion("mixed", result, err)
			return result, fmt.Errorf("db write workload: %w", err)
		}
	}

	if durations.readMs > 0 {
		readParams := params
		readParams.DurationMs = durations.readMs
		readResult, err := g.DBRead.Execute(ctx, readParams)
		result.Ops += readResult.Ops
		if err != nil {
			result.Duration = time.Since(start)
			result.Error = err.Error()
			finalizeSpan(span, result, err)
			logCompletion("mixed", result, err)
			return result, fmt.Errorf("db read workload: %w", err)
		}
	}

	result.Duration = time.Since(start)
	finalizeSpan(span, result, nil)
	logCompletion("mixed", result, nil)
	return result, nil
}

func splitMixedDurations(totalMs int64, profile MixedProfile) (mixedDurations, error) {
	if totalMs <= 0 {
		return mixedDurations{}, fmt.Errorf("duration_ms must be greater than zero")
	}
	if profile.CPUPercent < 0 || profile.DBWritePercent < 0 || profile.DBReadPercent < 0 {
		return mixedDurations{}, fmt.Errorf("mixed profile percentages must be non-negative")
	}
	totalPercent := profile.CPUPercent + profile.DBWritePercent + profile.DBReadPercent
	if totalPercent == 0 {
		profile = MixedProfile{CPUPercent: 34, DBWritePercent: 33, DBReadPercent: 33}
		totalPercent = 100
	}
	if totalPercent > 100 {
		return mixedDurations{}, fmt.Errorf("mixed profile percentages must not exceed 100")
	}

	phases := []mixedPhase{
		{name: "cpu", percent: profile.CPUPercent},
		{name: "db_write", percent: profile.DBWritePercent},
		{name: "db_read", percent: profile.DBReadPercent},
	}

	active := 0
	for i := range phases {
		if phases[i].percent > 0 {
			active++
			phases[i].ms = totalMs * int64(phases[i].percent) / 100
			if phases[i].ms == 0 {
				phases[i].ms = 1
			}
		}
	}
	if active == 0 {
		return mixedDurations{}, fmt.Errorf("mixed profile must include at least one workload")
	}
	if totalMs < int64(active) {
		return mixedDurations{}, fmt.Errorf("duration_ms is too short for mixed profile")
	}

	sum := int64(0)
	remainderTarget := -1
	remainderTargetPercent := -1
	for i := range phases {
		sum += phases[i].ms
		if phases[i].percent <= 0 {
			continue
		}
		if phases[i].percent > remainderTargetPercent {
			remainderTarget = i
			remainderTargetPercent = phases[i].percent
		}
	}
	if sum < totalMs && remainderTarget >= 0 {
		phases[remainderTarget].ms += totalMs - sum
	}
	for sumDurations(phases) > totalMs {
		reduced := false
		for i := range phases {
			if phases[i].ms > 1 {
				phases[i].ms--
				reduced = true
				break
			}
		}
		if !reduced {
			return mixedDurations{}, fmt.Errorf("duration_ms is too short for mixed profile")
		}
	}

	return mixedDurations{
		cpuMs:   phases[0].ms,
		writeMs: phases[1].ms,
		readMs:  phases[2].ms,
	}, nil
}

func sumDurations(phases []mixedPhase) int64 {
	total := int64(0)
	for _, phase := range phases {
		total += phase.ms
	}
	return total
}
