package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/messaging"
	"github.com/Ghvinerias/clusterprobe/internal/workload"
)

const (
	resultsExchange       = "clusterprobe.events"
	metricsSnapshotInsert = "INSERT INTO metrics_snapshots (snapshot) VALUES ($1)"
)

func reportResult(
	ctx context.Context,
	scenario workload.ScenarioResponse,
	result workload.Result,
	redis *redisAdapter,
	producer *messaging.Producer,
	store workload.SQLStore,
) error {
	snapshot := map[string]any{
		"scenario_id": scenario.ID,
		"workload":    scenario.Profile.WorkloadType,
		"ops":         result.Ops,
		"duration_ms": result.Duration.Milliseconds(),
		"error":       result.Error,
		"timestamp":   time.Now().UTC(),
	}

	encoded, err := json.Marshal(snapshot)
	if err != nil {
		return fmt.Errorf("marshal snapshot: %w", err)
	}

	if err := store.Exec(ctx, metricsSnapshotInsert, encoded); err != nil {
		return fmt.Errorf("insert metrics snapshot: %w", err)
	}

	if err := redis.Incr(ctx, "cp:ops:total"); err != nil {
		return fmt.Errorf("increment ops total: %w", err)
	}
	if err := redis.Incr(ctx, fmt.Sprintf("cp:ops:%s", scenario.Profile.WorkloadType)); err != nil {
		return fmt.Errorf("increment ops by type: %w", err)
	}
	if result.Error != "" {
		if err := redis.Incr(ctx, "cp:errors:total"); err != nil {
			return fmt.Errorf("increment errors total: %w", err)
		}
	}

	if err := producer.Publish(ctx, resultsExchange, fmt.Sprintf("results.%s", scenario.ID), encoded); err != nil {
		return fmt.Errorf("publish result: %w", err)
	}

	return nil
}
