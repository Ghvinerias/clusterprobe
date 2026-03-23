package handlers

import (
	"net/http"
	"time"

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

	data := ChaosListData{Active: "chaos", Experiments: views}
	if err := s.RenderTemplate(w, "base", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

// NewChaos renders the chaos form.
func (s *Server) NewChaos(w http.ResponseWriter, r *http.Request) {
	_, span := s.newSpan(r.Context(), "ui.chaos.new")
	defer span.End()
	defer s.logRequest(r, "chaos_new")

	data := FormData{Active: "chaos", Now: time.Now()}
	if err := s.RenderTemplate(w, "base", data); err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
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

	if _, err := s.api.CreateExperiment(ctx, req); err != nil {
		http.Error(w, "failed to create experiment", http.StatusBadGateway)
		return
	}

	http.Redirect(w, r, "/chaos", http.StatusSeeOther)
}
