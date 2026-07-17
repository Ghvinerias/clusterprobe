package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
)

const (
	insertScenarioQuery = "INSERT INTO scenario_events (scenario_id, payload) VALUES ($1, $2)"
	latestScenarioQuery = "SELECT payload FROM scenario_events WHERE scenario_id=$1 " +
		"ORDER BY created_at DESC, id DESC LIMIT 1"
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

func latestScenarioStatus(ctx context.Context, store workload.SQLStore, id string) (string, error) {
	row := store.QueryRow(ctx, latestScenarioQuery, id)
	var payload []byte
	if err := row.Scan(&payload); err != nil {
		return "", fmt.Errorf("select latest scenario status: %w", err)
	}

	var scenario workload.ScenarioResponse
	if err := json.Unmarshal(payload, &scenario); err != nil {
		return "", fmt.Errorf("decode latest scenario status: %w", err)
	}
	return scenario.Status, nil
}

func isStoppedScenarioStatus(status string) bool {
	return strings.EqualFold(strings.TrimSpace(status), "stopped")
}

func watchScenarioStop(
	ctx context.Context,
	store workload.SQLStore,
	id string,
	interval time.Duration,
	cancel context.CancelFunc,
) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		if interval <= 0 {
			interval = 500 * time.Millisecond
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				status, err := latestScenarioStatus(ctx, store, id)
				if err != nil {
					continue
				}
				if isStoppedScenarioStatus(status) {
					cancel()
					return
				}
			}
		}
	}()
	return done
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
