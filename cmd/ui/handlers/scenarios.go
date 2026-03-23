package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
)

// Root redirects to the scenarios list.
func (s *Server) Root(w http.ResponseWriter, r *http.Request) {
	_, span := s.newSpan(r.Context(), "ui.root")
	defer span.End()
	defer s.logRequest(r, "root")

	http.Redirect(w, r, "/scenarios", http.StatusFound)
}

// ListScenarios renders the scenario list.
func (s *Server) ListScenarios(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.scenarios")
	defer span.End()
	span.SetAttributes(attribute.String("ui.page", "scenarios"))
	defer s.logRequest(r, "scenarios")

	scenarios, err := s.api.ListScenarios(ctx)
	if err != nil {
		http.Error(w, "failed to load scenarios", http.StatusBadGateway)
		return
	}

	views := make([]ScenarioView, 0, len(scenarios))
	for _, scenario := range scenarios {
		views = append(views, buildScenarioView(scenario))
	}

	var banner *Banner
	if r.URL.Query().Get("success") != "" {
		banner = &Banner{Status: "success", StatusClass: "success", Message: "Scenario queued"}
	}

	data := ScenarioListData{Active: "scenarios", Scenarios: views, Banner: banner}
	if err := s.RenderTemplate(w, "base", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// NewScenario renders the scenario form.
func (s *Server) NewScenario(w http.ResponseWriter, r *http.Request) {
	_, span := s.newSpan(r.Context(), "ui.scenarios.new")
	defer span.End()
	defer s.logRequest(r, "scenarios_new")

	data := FormData{Active: "scenarios", Now: time.Now()}
	if err := s.RenderTemplate(w, "base", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// CreateScenario submits a new scenario to the API.
func (s *Server) CreateScenario(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.scenarios.create")
	defer span.End()
	defer s.logRequest(r, "scenarios_create")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	rps, err := parseIntField(r.FormValue("rps"), "rps")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	payloadSize, err := parseIntField(r.FormValue("payload_size"), "payload_size")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	concurrency, err := parseIntField(r.FormValue("concurrency"), "concurrency")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	duration, err := parseDurationSeconds(r.FormValue("duration"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	req := workload.ScenarioRequest{
		Name: r.FormValue("name"),
		Profile: workload.LoadProfile{
			RPS:              rps,
			Duration:         duration,
			PayloadSizeBytes: payloadSize,
			Concurrency:      concurrency,
			TargetQueue:      r.FormValue("target_queue"),
			WorkloadType:     workload.WorkloadType(r.FormValue("workload_type")),
		},
	}

	if _, err := s.api.CreateScenario(ctx, req); err != nil {
		http.Error(w, "failed to create scenario", http.StatusBadGateway)
		return
	}

	http.Redirect(w, r, "/scenarios?success=1", http.StatusSeeOther)
}

// StopScenario stops a scenario and returns a row partial.
func (s *Server) StopScenario(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.scenarios.stop")
	defer span.End()
	defer s.logRequest(r, "scenarios_stop")

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	scenario, err := s.api.StopScenario(ctx, id)
	if err != nil {
		http.Error(w, "failed to stop scenario", http.StatusBadGateway)
		return
	}

	view := buildScenarioView(scenario)
	if err := s.templates.ExecuteTemplate(w, "scenario-row", view); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
