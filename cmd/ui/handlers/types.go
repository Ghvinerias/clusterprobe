package handlers

import (
	"context"
	"io"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
)

// APIClient abstracts the backend API.
type APIClient interface {
	ListScenarios(ctx context.Context) ([]workload.ScenarioResponse, error)
	CreateScenario(ctx context.Context, req workload.ScenarioRequest) (workload.ScenarioResponse, error)
	StopScenario(ctx context.Context, id string) (workload.ScenarioResponse, error)
	ListExperiments(ctx context.Context) ([]workload.ChaosExperimentResponse, error)
	CreateExperiment(ctx context.Context, req workload.ChaosExperimentRequest) (workload.ChaosExperimentResponse, error)
	LogsStream(ctx context.Context) (io.ReadCloser, error)
}

// CounterStore reads counters for dashboard stats.
type CounterStore interface {
	Get(ctx context.Context, key string) (string, error)
}

// Stat describes a dashboard stat block.
type Stat struct {
	Label       string
	Value       string
	Status      string
	StatusClass string
}

// Banner describes a notification banner.
type Banner struct {
	Status      string
	StatusClass string
	Message     string
}

// ScenarioView wraps scenario data for templates.
type ScenarioView struct {
	workload.ScenarioResponse
	StatusClass string
}

// ExperimentView wraps chaos experiment data for templates.
type ExperimentView struct {
	workload.ChaosExperimentResponse
	StatusClass string
}

// DashboardData renders the dashboard template.
type DashboardData struct {
	Active string
	Stats  []Stat
}

// ScenarioListData renders scenarios list.
type ScenarioListData struct {
	Active    string
	Scenarios []ScenarioView
	Banner    *Banner
}

// ChaosListData renders chaos list.
type ChaosListData struct {
	Active      string
	Experiments []ExperimentView
}

// FormData renders forms.
type FormData struct {
	Active string
	Now    time.Time
}

// LogsData renders logs page.
type LogsData struct {
	Active string
}
