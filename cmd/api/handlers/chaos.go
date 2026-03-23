package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
	"github.com/google/uuid"
)

const chaosExchange = "clusterprobe.events"
const chaosRoutingKey = "workload.chaos"

// ChaosHandler handles chaos endpoints.
type ChaosHandler struct {
	store     PostgresStore
	publisher Publisher
}

// NewChaosHandler builds a ChaosHandler.
func NewChaosHandler(store PostgresStore, publisher Publisher) *ChaosHandler {
	return &ChaosHandler{store: store, publisher: publisher}
}

// CreateExperiment handles POST /api/v1/chaos/experiments.
func (h *ChaosHandler) CreateExperiment(w http.ResponseWriter, r *http.Request) {
	var req workload.ChaosExperimentRequest
	if err := decodeJSON(r, &req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := req.Validate(); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := workload.ChaosExperimentResponse{
		ID:        uuid.NewString(),
		Name:      req.Name,
		Scenario:  req.Scenario,
		Config:    req.Config,
		Status:    "queued",
		CreatedAt: time.Now().UTC(),
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "marshal experiment")
		return
	}

	if err := h.store.Exec(r.Context(), "INSERT INTO chaos_events (experiment_name, payload) VALUES ($1, $2)", resp.Name, payload); err != nil {
		errorResponse(w, http.StatusInternalServerError, "store experiment")
		return
	}

	if err := h.publisher.Publish(r.Context(), chaosExchange, chaosRoutingKey, payload); err != nil {
		errorResponse(w, http.StatusBadGateway, "publish experiment")
		return
	}

	if err := writeJSON(w, http.StatusAccepted, resp); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

// ListExperiments handles GET /api/v1/chaos/experiments.
func (h *ChaosHandler) ListExperiments(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.Query(r.Context(), "SELECT experiment_name, payload, created_at FROM chaos_events ORDER BY created_at DESC LIMIT 50")
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list experiments")
		return
	}
	defer rows.Close()

	responses := make([]workload.ChaosExperimentResponse, 0)
	for rows.Next() {
		var name string
		var payload []byte
		var created time.Time
		if err := rows.Scan(&name, &payload, &created); err != nil {
			errorResponse(w, http.StatusInternalServerError, "scan experiment")
			return
		}
		var resp workload.ChaosExperimentResponse
		if err := json.Unmarshal(payload, &resp); err != nil {
			errorResponse(w, http.StatusInternalServerError, "decode experiment")
			return
		}
		if resp.Name == "" {
			resp.Name = name
		}
		if resp.CreatedAt.IsZero() {
			resp.CreatedAt = created
		}
		responses = append(responses, resp)
	}
	if err := rows.Err(); err != nil {
		errorResponse(w, http.StatusInternalServerError, "list experiments")
		return
	}

	if err := writeJSON(w, http.StatusOK, responses); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}
