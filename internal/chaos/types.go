package chaos

import "time"

// ExperimentSpec describes a Chaos Mesh experiment request.
type ExperimentSpec struct {
	Name      string
	Namespace string
	Kind      string
	Spec      map[string]any
}

// ExperimentStatus captures the lifecycle state of a Chaos Mesh experiment.
type ExperimentStatus struct {
	ID        string
	Name      string
	Namespace string
	Kind      string
	Phase     string
	Message   string
	StartTime *time.Time
	EndTime   *time.Time
}
