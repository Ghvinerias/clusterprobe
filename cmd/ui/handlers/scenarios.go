package handlers

import (
	"fmt"
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
		s.renderError(w, http.StatusBadGateway, ErrorData{
			Active:    "scenarios",
			Title:     "Scenarios Unavailable | ClusterProbe",
			Heading:   "Scenarios unavailable",
			Message:   "ClusterProbe could not load scenarios from the API. Check API health and try again.",
			Status:    "API unavailable",
			BackHref:  "/dashboard",
			BackLabel: "Back to Dashboard",
		})
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

	data := ScenarioListData{
		Active:    "scenarios",
		Title:     "Scenarios | ClusterProbe",
		Scenarios: views,
		Banner:    banner,
	}
	if err := s.RenderTemplate(w, "scenarios", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// NewScenario renders the scenario form.
func (s *Server) NewScenario(w http.ResponseWriter, r *http.Request) {
	_, span := s.newSpan(r.Context(), "ui.scenarios.new")
	defer span.End()
	defer s.logRequest(r, "scenarios_new")

	data := FormData{
		Active: "scenarios",
		Title:  "New Scenario | ClusterProbe",
		Now:    time.Now(),
	}
	if err := s.RenderTemplate(w, "scenario-new", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// ScenarioDetail renders one scenario with lifecycle history.
func (s *Server) ScenarioDetail(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.scenarios.detail")
	defer span.End()
	defer s.logRequest(r, "scenarios_detail")

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	scenario, err := s.api.GetScenario(ctx, id)
	if err != nil {
		if isNotFoundError(err) {
			s.renderNotFound(w, NotFoundData{
				Active:    "scenarios",
				Title:     "Scenario Not Found | ClusterProbe",
				Heading:   "Scenario not found",
				Message:   "The scenario may have been deleted or the ID is no longer available.",
				BackHref:  "/scenarios",
				BackLabel: "Back to Scenarios",
			})
			return
		}
		http.Error(w, "failed to load scenario", http.StatusBadGateway)
		return
	}
	events, err := s.api.ListScenarioEvents(ctx, id)
	if err != nil {
		http.Error(w, "failed to load scenario events", http.StatusBadGateway)
		return
	}

	data := ScenarioDetailData{
		Active:   "scenarios",
		Title:    "Scenario | ClusterProbe",
		Scenario: buildScenarioView(scenario),
		Events:   buildScenarioViews(events),
	}
	if err := s.RenderTemplate(w, "scenario-detail", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// ScenarioEvents renders lifecycle history for one scenario.
func (s *Server) ScenarioEvents(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.scenarios.events")
	defer span.End()
	defer s.logRequest(r, "scenarios_events")

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	scenario, err := s.api.GetScenario(ctx, id)
	if err != nil {
		http.Error(w, "failed to load scenario", http.StatusBadGateway)
		return
	}
	events, err := s.api.ListScenarioEvents(ctx, id)
	if err != nil {
		http.Error(w, "failed to load scenario events", http.StatusBadGateway)
		return
	}

	data := ScenarioEventsData{
		Scenario: buildScenarioView(scenario),
		Events:   buildScenarioViews(events),
	}
	if err := s.renderScenarioEvents(w, data); err != nil {
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

// ScenarioRow renders one live-updating scenario row partial.
func (s *Server) ScenarioRow(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.scenarios.row")
	defer span.End()
	defer s.logRequest(r, "scenarios_row")

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	scenario, err := s.api.GetScenario(ctx, id)
	if err != nil {
		http.Error(w, "failed to load scenario", http.StatusBadGateway)
		return
	}

	if err := s.renderScenarioRow(w, buildScenarioView(scenario)); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
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

	if err := s.renderScenarioRow(w, buildScenarioView(scenario)); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (s *Server) renderScenarioRow(w http.ResponseWriter, view ScenarioView) error {
	tmpl, ok := s.templates["scenarios"]
	if !ok {
		return fmt.Errorf("template not found: scenarios")
	}
	if err := tmpl.ExecuteTemplate(w, "scenario-row", view); err != nil {
		return fmt.Errorf("execute scenario row template: %w", err)
	}
	return nil
}

func (s *Server) renderScenarioEvents(w http.ResponseWriter, data ScenarioEventsData) error {
	tmpl, ok := s.templates["scenario-detail"]
	if !ok {
		return fmt.Errorf("template not found: scenario-detail")
	}
	if err := tmpl.ExecuteTemplate(w, "scenario-events", data); err != nil {
		return fmt.Errorf("execute scenario events template: %w", err)
	}
	return nil
}

func buildScenarioViews(scenarios []workload.ScenarioResponse) []ScenarioView {
	views := make([]ScenarioView, 0, len(scenarios))
	for _, scenario := range scenarios {
		views = append(views, buildScenarioView(scenario))
	}
	return views
}
