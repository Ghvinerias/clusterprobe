package handlers

import (
	"encoding/json"
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
	for key, redisKey := range h.statusKeys {
		value, err := h.redis.Get(r.Context(), redisKey)
		if err != nil {
			counters[key] = 0
			continue
		}
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			counters[key] = 0
			continue
		}
		counters[key] = parsed
	}

	payload := map[string]any{
		"status":   "ok",
		"time":     time.Now().UTC(),
		"counters": counters,
	}

	if err := writeJSON(w, http.StatusOK, payload); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
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
	row := h.store.QueryRow(r.Context(), "SELECT snapshot, created_at FROM metrics_snapshots ORDER BY created_at DESC LIMIT 1")
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
