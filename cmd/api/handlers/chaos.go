package handlers

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/chaos"
	"github.com/Ghvinerias/clusterprobe/internal/workload"
	"github.com/go-chi/chi/v5"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const chaosCollection = "chaos_experiments"

// ChaosHandler handles chaos endpoints.
type ChaosHandler struct {
	store  MongoStore
	client ChaosEngine
}

// NewChaosHandler builds a ChaosHandler.
func NewChaosHandler(store MongoStore, client ChaosEngine) *ChaosHandler {
	return &ChaosHandler{store: store, client: client}
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

	spec, err := buildExperimentSpec(req)
	if err != nil {
		errorResponse(w, http.StatusBadRequest, err.Error())
		return
	}

	id, err := h.client.Apply(r.Context(), spec)
	if err != nil {
		errorResponse(w, http.StatusBadGateway, "apply chaos experiment")
		return
	}

	record := chaosRecord{
		ID:        id,
		Name:      req.Name,
		Scenario:  req.Scenario,
		Config:    req.Config,
		Status:    "pending",
		CreatedAt: time.Now().UTC(),
	}

	if err := h.store.InsertOne(r.Context(), chaosCollection, record); err != nil {
		errorResponse(w, http.StatusInternalServerError, "store experiment")
		return
	}

	resp := record.toResponse()
	if err := writeJSON(w, http.StatusAccepted, resp); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

// ListExperiments handles GET /api/v1/chaos/experiments.
func (h *ChaosHandler) ListExperiments(w http.ResponseWriter, r *http.Request) {
	cursor, err := h.store.Find(r.Context(), chaosCollection, bson.M{})
	if err != nil {
		errorResponse(w, http.StatusInternalServerError, "list experiments")
		return
	}
	defer func() {
		_ = cursor.Close(r.Context())
	}()

	responses := make([]workload.ChaosExperimentResponse, 0)
	for cursor.Next(r.Context()) {
		var record chaosRecord
		if err := cursor.Decode(&record); err != nil {
			errorResponse(w, http.StatusInternalServerError, "decode experiment")
			return
		}
		responses = append(responses, record.toResponse())
	}
	if err := cursor.Err(); err != nil {
		errorResponse(w, http.StatusInternalServerError, "list experiments")
		return
	}

	if err := writeJSON(w, http.StatusOK, responses); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

// GetExperiment handles GET /api/v1/chaos/experiments/{id}.
func (h *ChaosHandler) GetExperiment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "missing id")
		return
	}

	status, err := h.client.Status(r.Context(), id)
	if err != nil {
		errorResponse(w, http.StatusNotFound, "experiment not found")
		return
	}

	response := workload.ChaosExperimentResponse{
		ID:     status.ID,
		Name:   status.Name,
		Status: status.Phase,
	}

	if record, ok := h.findRecord(r.Context(), id); ok {
		response.Name = record.Name
		response.Scenario = record.Scenario
		response.Config = record.Config
		response.CreatedAt = record.CreatedAt
	}

	if r.URL.Query().Get("view") == "badge" {
		badge := renderStatusBadge(id, response.Status)
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := w.Write([]byte(badge)); err != nil {
			errorResponse(w, http.StatusInternalServerError, "render badge")
		}
		return
	}

	if err := writeJSON(w, http.StatusOK, response); err != nil {
		errorResponse(w, http.StatusInternalServerError, err.Error())
	}
}

// DeleteExperiment handles DELETE /api/v1/chaos/experiments/{id}.
func (h *ChaosHandler) DeleteExperiment(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		errorResponse(w, http.StatusBadRequest, "missing id")
		return
	}

	if err := h.client.Delete(r.Context(), id); err != nil {
		errorResponse(w, http.StatusBadGateway, "delete experiment")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h *ChaosHandler) findRecord(ctx context.Context, id string) (chaosRecord, bool) {
	cursor, err := h.store.Find(ctx, chaosCollection, bson.M{"_id": id})
	if err != nil {
		return chaosRecord{}, false
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()
	if !cursor.Next(ctx) {
		return chaosRecord{}, false
	}
	var record chaosRecord
	if err := cursor.Decode(&record); err != nil {
		return chaosRecord{}, false
	}
	return record, true
}

func buildExperimentSpec(req workload.ChaosExperimentRequest) (chaos.ExperimentSpec, error) {
	kind := chaosKindFromConfig(req.Config)
	if kind == "" {
		return chaos.ExperimentSpec{}, fmt.Errorf("chaos type is required")
	}

	spec := chaosSpecFromConfig(kind, req.Config)
	if spec == nil {
		return chaos.ExperimentSpec{}, fmt.Errorf("invalid chaos config")
	}

	return chaos.ExperimentSpec{
		Name:      req.Name,
		Namespace: "cluster-probe",
		Kind:      kind,
		Spec:      spec,
	}, nil
}

func chaosKindFromConfig(config map[string]string) string {
	switch config["type"] {
	case "pod":
		return "PodChaos"
	case "network":
		return "NetworkChaos"
	case "io":
		return "IOChaos"
	case "stress":
		return "StressChaos"
	case "time":
		return "TimeChaos"
	default:
		return ""
	}
}

func chaosSpecFromConfig(kind string, config map[string]string) map[string]any {
	selector := map[string]any{
		"namespaces": []string{"cluster-probe"},
	}
	if target := config["target"]; target != "" {
		if labels := parseLabelSelector(target); len(labels) > 0 {
			selector["labelSelectors"] = labels
		}
	}

	duration := config["duration"]
	if duration == "" {
		duration = "3m"
	}

	switch kind {
	case "PodChaos":
		return map[string]any{
			"action":   "pod-kill",
			"mode":     "one",
			"selector": selector,
			"duration": duration,
		}
	case "NetworkChaos":
		return map[string]any{
			"action":   "delay",
			"mode":     "all",
			"selector": selector,
			"delay": map[string]any{
				"latency":     valueOr(config["latency"], "100ms"),
				"jitter":      valueOr(config["jitter"], "20ms"),
				"correlation": valueOr(config["correlation"], "0"),
			},
			"duration": duration,
		}
	case "IOChaos":
		return map[string]any{
			"action":     "latency",
			"mode":       "one",
			"selector":   selector,
			"volumePath": valueOr(config["volumePath"], "/var/lib/postgresql/data"),
			"path":       valueOr(config["path"], "/var/lib/postgresql/data/**"),
			"delay":      valueOr(config["delay"], "50ms"),
			"percent":    intValueOr(config["percent"], 100),
			"duration":   duration,
		}
	case "StressChaos":
		return map[string]any{
			"mode":     "all",
			"selector": selector,
			"stressors": map[string]any{
				"cpu": map[string]any{
					"workers": intValueOr(config["workers"], 2),
					"load":    intValueOr(config["load"], 100),
				},
			},
			"duration": duration,
		}
	case "TimeChaos":
		return map[string]any{
			"mode":       "one",
			"selector":   selector,
			"timeOffset": valueOr(config["offset"], "5m"),
			"duration":   duration,
		}
	default:
		return nil
	}
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func intValueOr(value string, fallback int) int {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func parseLabelSelector(input string) map[string]string {
	parts := strings.SplitN(input, "=", 2)
	if len(parts) != 2 {
		return nil
	}
	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	if key == "" || value == "" {
		return nil
	}
	return map[string]string{key: value}
}

func renderStatusBadge(id string, status string) string {
	class := statusBadgeClass(status)
	if isTerminalStatus(status) {
		return fmt.Sprintf(`<span class="badge %s">%s</span>`, class, status)
	}
	return fmt.Sprintf(
		`<span class="badge %s" hx-get="/api/v1/chaos/experiments/%s?view=badge" hx-trigger="every 3s" hx-swap="outerHTML">%s</span>`,
		class,
		id,
		status,
	)
}

func statusBadgeClass(status string) string {
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "fail"), strings.Contains(lower, "error"):
		return "error"
	case strings.Contains(lower, "running"), strings.Contains(lower, "finish"), strings.Contains(lower, "complete"):
		return "success"
	default:
		return "warn"
	}
}

func isTerminalStatus(status string) bool {
	lower := strings.ToLower(status)
	return strings.Contains(lower, "running") || strings.Contains(lower, "fail")
}

type chaosRecord struct {
	ID        string            `bson:"_id"`
	Name      string            `bson:"name"`
	Scenario  string            `bson:"scenario"`
	Config    map[string]string `bson:"config"`
	Status    string            `bson:"status"`
	CreatedAt time.Time         `bson:"created_at"`
}

func (r chaosRecord) toResponse() workload.ChaosExperimentResponse {
	return workload.ChaosExperimentResponse{
		ID:        r.ID,
		Name:      r.Name,
		Scenario:  r.Scenario,
		Config:    r.Config,
		Status:    r.Status,
		CreatedAt: r.CreatedAt,
	}
}
