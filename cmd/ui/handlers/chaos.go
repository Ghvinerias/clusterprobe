package handlers

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/attribute"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
)

// ListChaos renders the chaos list.
func (s *Server) ListChaos(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.chaos")
	defer span.End()
	span.SetAttributes(attribute.String("ui.page", "chaos"))
	defer s.logRequest(r, "chaos")

	experiments, err := s.api.ListExperiments(ctx)
	if err != nil {
		http.Error(w, "failed to load experiments", http.StatusBadGateway)
		return
	}

	views := make([]ExperimentView, 0, len(experiments))
	for _, exp := range experiments {
		views = append(views, buildExperimentView(exp))
	}

	data := ChaosListData{
		Active:      "chaos",
		Title:       "Chaos Experiments | ClusterProbe",
		Experiments: views,
	}
	if err := s.RenderTemplate(w, "chaos", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// NewChaos renders the chaos form.
func (s *Server) NewChaos(w http.ResponseWriter, r *http.Request) {
	_, span := s.newSpan(r.Context(), "ui.chaos.new")
	defer span.End()
	defer s.logRequest(r, "chaos_new")

	data := FormData{
		Active: "chaos",
		Title:  "New Chaos Experiment | ClusterProbe",
		Now:    time.Now(),
	}
	if err := s.RenderTemplate(w, "chaos-new", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// ChaosStatus renders a live status badge for one chaos experiment.
func (s *Server) ChaosStatus(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.chaos.status")
	defer span.End()
	defer s.logRequest(r, "chaos_status")

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}

	experiment, err := s.api.GetExperiment(ctx, id)
	if err != nil {
		http.Error(w, "failed to load experiment", http.StatusBadGateway)
		return
	}

	tmpl, ok := s.templates["chaos"]
	if !ok {
		http.Error(w, "template error", http.StatusInternalServerError)
		return
	}
	data := ExperimentStatusData{
		ID:           experiment.ID,
		ExperimentID: experiment.ID,
		Status:       experiment.Status,
		StatusClass:  statusClass(experiment.Status),
	}
	if err := tmpl.ExecuteTemplate(w, "experiment-status", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// DeleteChaos deletes a chaos experiment and removes its table row.
func (s *Server) DeleteChaos(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.chaos.delete")
	defer span.End()
	defer s.logRequest(r, "chaos_delete")

	id := chi.URLParam(r, "id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	if err := s.api.DeleteExperiment(ctx, id); err != nil {
		http.Error(w, "failed to delete experiment", http.StatusBadGateway)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// CreateChaos submits a chaos experiment to the API.
func (s *Server) CreateChaos(w http.ResponseWriter, r *http.Request) {
	ctx, span := s.newSpan(r.Context(), "ui.chaos.create")
	defer span.End()
	defer s.logRequest(r, "chaos_create")

	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}

	config := parseChaosParameters(r.FormValue("parameters"))
	config["type"] = r.FormValue("type")
	config["target"] = r.FormValue("target")
	config["duration"] = r.FormValue("duration")

	req := workload.ChaosExperimentRequest{
		Name:     r.FormValue("name"),
		Scenario: r.FormValue("scenario"),
		Config:   config,
	}

	resp, err := s.api.CreateExperiment(ctx, req)
	if err != nil {
		http.Error(w, "failed to create experiment", http.StatusBadGateway)
		return
	}

	data := FormData{
		Active:       "chaos",
		Title:        "New Chaos Experiment | ClusterProbe",
		Now:          time.Now(),
		ID:           resp.ID,
		ExperimentID: resp.ID,
		Status:       resp.Status,
		StatusClass:  statusClass(resp.Status),
	}
	if err := s.RenderTemplate(w, "chaos-new", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}
