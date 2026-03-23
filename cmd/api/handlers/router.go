package handlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"
)

// Router wires API routes.
type Router struct {
	scenarios *ScenarioHandler
	status    *StatusHandler
	chaos     *ChaosHandler
}

// NewRouter builds the router dependencies.
func NewRouter(scenarios *ScenarioHandler, status *StatusHandler, chaos *ChaosHandler) *Router {
	return &Router{
		scenarios: scenarios,
		status:    status,
		chaos:     chaos,
	}
}

// Routes registers API routes.
func (r *Router) Routes() http.Handler {
	mux := chi.NewRouter()

	mux.Get("/healthz", r.status.Healthz)

	mux.Route("/api/v1", func(router chi.Router) {
		router.Get("/status", r.status.Status)
		router.Get("/metrics/snapshot", r.status.MetricsSnapshot)

		router.Route("/scenarios", func(router chi.Router) {
			router.Post("/", r.scenarios.CreateScenario)
			router.Get("/", r.scenarios.ListScenarios)
			router.Get("/{id}", r.scenarios.GetScenario)
			router.Put("/{id}/stop", r.scenarios.StopScenario)
		})

		router.Route("/chaos/experiments", func(router chi.Router) {
			router.Post("/", r.chaos.CreateExperiment)
			router.Get("/", r.chaos.ListExperiments)
			router.Get("/{id}", r.chaos.GetExperiment)
			router.Delete("/{id}", r.chaos.DeleteExperiment)
		})
	})

	return mux
}
