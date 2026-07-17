package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// StatusHandler handles status endpoints.
type StatusHandler struct {
	store      PostgresStore
	redis      RedisStore
	statusKeys map[string]string
}

const (
	metricsSnapshotQuery = "SELECT snapshot, created_at FROM metrics_snapshots ORDER BY created_at DESC LIMIT 1"
	scenarioCountsQuery  = "SELECT " +
		"COUNT(*) FILTER (WHERE payload->>'status' = 'running'), " +
		"COUNT(*) FILTER (WHERE payload->>'status' = 'completed') " +
		"FROM (" +
		"SELECT DISTINCT ON (scenario_id) payload FROM scenario_events " +
		"ORDER BY scenario_id, created_at DESC, id DESC" +
		") latest"
)

// NewStatusHandler builds a StatusHandler.
func NewStatusHandler(store PostgresStore, redis RedisStore) *StatusHandler {
	return &StatusHandler{
		store: store,
		redis: redis,
		statusKeys: map[string]string{
			"scenarios_running":   "scenarios.running",
			"scenarios_completed": "scenarios.completed",
			"chaos_running":       "chaos.running",
		},
	}
}

// Status handles GET /api/v1/status.
func (h *StatusHandler) Status(w http.ResponseWriter, r *http.Request) {
	counters := make(map[string]int64)
	running, completed, err := h.scenarioCounts(r.Context())
	if err == nil {
		counters["scenarios_running"] = running
		counters["scenarios_completed"] = completed
	} else {
		counters["scenarios_running"] = h.redisCounter(r.Context(), "scenarios_running")
		counters["scenarios_completed"] = h.redisCounter(r.Context(), "scenarios_completed")
	}
	counters["chaos_running"] = h.redisCounter(r.Context(), "chaos_running")

	payload := map[string]any{
		"status":   "ok",
		"time":     time.Now().UTC(),
		"counters": counters,
	}

	if err := writeJSON(w, http.StatusOK, payload); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

func (h *StatusHandler) scenarioCounts(ctx context.Context) (int64, int64, error) {
	var running int64
	var completed int64
	if err := h.store.QueryRow(ctx, scenarioCountsQuery).Scan(&running, &completed); err != nil {
		return 0, 0, fmt.Errorf("scan scenario counts: %w", err)
	}
	return running, completed, nil
}

func (h *StatusHandler) redisCounter(ctx context.Context, key string) int64 {
	redisKey := h.statusKeys[key]
	value, err := h.redis.Get(ctx, redisKey)
	if err != nil {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

// Healthz handles GET /healthz.
func (h *StatusHandler) Healthz(w http.ResponseWriter, r *http.Request) {
	payload := map[string]string{
		"status": "ok",
	}

	if err := writeJSON(w, http.StatusOK, payload); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

// MetricsSnapshot handles GET /api/v1/metrics/snapshot.
func (h *StatusHandler) MetricsSnapshot(w http.ResponseWriter, r *http.Request) {
	row := h.store.QueryRow(r.Context(), metricsSnapshotQuery)
	var payload []byte
	var created time.Time
	if err := row.Scan(&payload, &created); err != nil {
		errorResponse(w, http.StatusNotFound, "no metrics snapshot")
		return
	}

	var decoded any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		errorResponse(w, http.StatusInternalServerError, "decode snapshot")
		return
	}

	response := map[string]any{
		"snapshot":   decoded,
		"created_at": created,
	}

	if err := writeJSON(w, http.StatusOK, response); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}
