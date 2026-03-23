package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

const scenariosExchange = "clusterprobe.events"

const (
	insertScenarioQuery = "INSERT INTO load_events (scenario_id, payload) VALUES ($1, $2)"
	listScenarioQuery   = "SELECT scenario_id, payload, created_at FROM load_events ORDER BY created_at DESC LIMIT 50"
	getScenarioQuery    = "SELECT scenario_id, payload, created_at FROM load_events " +
		"WHERE scenario_id=$1 ORDER BY created_at DESC LIMIT 1"
)

// ScenarioHandler handles scenario endpoints.
type ScenarioHandler struct {
	store     PostgresStore
	publisher Publisher
}

// NewScenarioHandler builds a ScenarioHandler.
func NewScenarioHandler(store PostgresStore, publisher Publisher) *ScenarioHandler {
	return &ScenarioHandler{store: store, publisher: publisher}
}

// CreateScenario handles POST /api/v1/scenarios.
func (h *ScenarioHandler) CreateScenario(w http.ResponseWriter, r *http.Request) {
	var req workload.ScenarioRequest
	if err := decodeJSON(r, &req); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := req.Validate(); err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	resp := workload.ScenarioResponse{
		ID:        uuid.NewString(),
		Name:      req.Name,
		Profile:   req.Profile,
		Status:    "queued",
		CreatedAt: time.Now().UTC(),
	}

	payload, err := json.Marshal(resp)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "marshal scenario")
		return
	}

	if err := h.store.Exec(r.Context(), insertScenarioQuery, resp.ID, payload); err != nil {
		errorResponse(w, http.StatusInternalServerError, "store scenario")
		return
	}

	if err := h.publisher.Publish(r.Context(), scenariosExchange, req.Profile.TargetQueue, payload); err != nil {
		errorResponse(w, http.StatusBadGateway, "publish scenario")
		return
	}

	if err := writeJSON(w, http.StatusAccepted, resp); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

// ListScenarios handles GET /api/v1/scenarios.
func (h *ScenarioHandler) ListScenarios(w http.ResponseWriter, r *http.Request) {
	rows, err := h.store.Query(r.Context(), listScenarioQuery)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list scenarios")
		return
	}
	defer rows.Close()

	responses := make([]workload.ScenarioResponse, 0)
	for rows.Next() {
		var id string
		var payload []byte
		var created time.Time
		if err := rows.Scan(&id, &payload, &created); err != nil {
			errorResponse(w, http.StatusInternalServerError, "scan scenario")
			return
		}
		var resp workload.ScenarioResponse
		if err := json.Unmarshal(payload, &resp); err != nil {
			errorResponse(w, http.StatusInternalServerError, "decode scenario")
			return
		}
		if resp.ID == "" {
			resp.ID = id
		}
		if resp.CreatedAt.IsZero() {
			resp.CreatedAt = created
		}
		responses = append(responses, resp)
	}
	if err := rows.Err(); err != nil {
		errorResponse(w, http.StatusInternalServerError, "list scenarios")
		return
	}

	if err := writeJSON(w, http.StatusOK, responses); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

// GetScenario handles GET /api/v1/scenarios/{id}.
func (h *ScenarioHandler) GetScenario(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "id is required")
		return
	}

	row := h.store.QueryRow(r.Context(), getScenarioQuery, id)
	var scenarioID string
	var payload []byte
	var created time.Time
	if err := row.Scan(&scenarioID, &payload, &created); err != nil {
		errorResponse(w, http.StatusNotFound, "scenario not found")
		return
	}

	var resp workload.ScenarioResponse
	if err := json.Unmarshal(payload, &resp); err != nil {
		errorResponse(w, http.StatusInternalServerError, "decode scenario")
		return
	}
	if resp.ID == "" {
		resp.ID = scenarioID
	}
	if resp.CreatedAt.IsZero() {
		resp.CreatedAt = created
	}

	if err := writeJSON(w, http.StatusOK, resp); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

// StopScenario handles PUT /api/v1/scenarios/{id}/stop.
func (h *ScenarioHandler) StopScenario(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "id is required")
		return
	}

	resp := workload.ScenarioResponse{
		ID:        id,
		Status:    "stopped",
		CreatedAt: time.Now().UTC(),
	}
	payload, err := json.Marshal(resp)
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "marshal scenario")
		return
	}

	if err := h.store.Exec(r.Context(), insertScenarioQuery, id, payload); err != nil {
		errorResponse(w, http.StatusInternalServerError, "store scenario")
		return
	}

	if err := h.publisher.Publish(r.Context(), scenariosExchange, "scenario.stop", payload); err != nil {
		errorResponse(w, http.StatusBadGateway, "publish stop")
		return
	}

	if err := writeJSON(w, http.StatusOK, resp); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}
