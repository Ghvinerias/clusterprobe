package main

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
)

const (
	insertScenarioQuery   = "INSERT INTO scenario_events (scenario_id, payload) VALUES ($1, $2)"
	resultsExchange       = "clusterprobe.events"
	metricsSnapshotInsert = "INSERT INTO metrics_snapshots (snapshot) VALUES ($1)"
)

func appendScenarioStatus(
	ctx context.Context,
	store workload.SQLStore,
	scenario workload.ScenarioResponse,
	status string,
) error {
	scenario.Status = status
	if scenario.CreatedAt.IsZero() {
		scenario.CreatedAt = time.Now().UTC()
	}

	encoded, err := json.Marshal(scenario)
	if err != nil {
		return fmt.Errorf("marshal scenario status: %w", err)
	}

	if err := store.Exec(ctx, insertScenarioQuery, scenario.ID, encoded); err != nil {
		return fmt.Errorf("insert scenario status: %w", err)
	}

	return nil
}

func reportResult(
	ctx context.Context,
	scenario workload.ScenarioResponse,
	result workload.Result,
	redis redisCounter,
	producer messagePublisher,
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
