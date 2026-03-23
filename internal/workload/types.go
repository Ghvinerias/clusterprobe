package workload

import (
	"fmt"
	"time"
)

// WorkloadType describes the workload behavior.
type WorkloadType string

const (
	WorkloadTypeCPUBurn  WorkloadType = "cpu_burn"
	WorkloadTypeMemAlloc WorkloadType = "mem_alloc"
	WorkloadTypeDBWrite  WorkloadType = "db_write"
	WorkloadTypeDBRead   WorkloadType = "db_read"
	WorkloadTypeMixed    WorkloadType = "mixed"
)

// LoadProfile defines the workload parameters for a scenario.
type LoadProfile struct {
	RPS              int           `json:"rps"`
	Duration         time.Duration `json:"duration"`
	PayloadSizeBytes int           `json:"payload_size_bytes"`
	Concurrency      int           `json:"concurrency"`
	TargetQueue      string        `json:"target_queue"`
	WorkloadType     WorkloadType  `json:"workload_type"`
}

// Validate validates the load profile.
func (p LoadProfile) Validate() error {
	if p.RPS <= 0 {
		return fmt.Errorf("rps must be greater than zero")
	}
	if p.Duration <= 0 {
		return fmt.Errorf("duration must be greater than zero")
	}
	if p.PayloadSizeBytes < 0 {
		return fmt.Errorf("payload_size_bytes must be non-negative")
	}
	if p.Concurrency <= 0 {
		return fmt.Errorf("concurrency must be greater than zero")
	}
	if p.TargetQueue == "" {
		return fmt.Errorf("target_queue is required")
	}
	switch p.WorkloadType {
	case WorkloadTypeCPUBurn, WorkloadTypeMemAlloc, WorkloadTypeDBWrite, WorkloadTypeDBRead, WorkloadTypeMixed:
		return nil
	default:
		return fmt.Errorf("workload_type is invalid")
	}
}

// ScenarioRequest represents a scenario creation request.
type ScenarioRequest struct {
	Name        string      `json:"name"`
	Profile     LoadProfile `json:"profile"`
	StopOnError bool        `json:"stop_on_error"`
}

// Validate validates the scenario request.
func (r ScenarioRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if err := r.Profile.Validate(); err != nil {
		return fmt.Errorf("profile: %w", err)
	}
	return nil
}

// ScenarioResponse represents a scenario response.
type ScenarioResponse struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	Profile   LoadProfile `json:"profile"`
	Status    string      `json:"status"`
	CreatedAt time.Time   `json:"created_at"`
}

// Validate validates the scenario response.
func (r ScenarioResponse) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("id is required")
	}
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if err := r.Profile.Validate(); err != nil {
		return fmt.Errorf("profile: %w", err)
	}
	if r.Status == "" {
		return fmt.Errorf("status is required")
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}
	return nil
}

// ChaosExperimentRequest represents a chaos experiment request.
type ChaosExperimentRequest struct {
	Name     string            `json:"name"`
	Scenario string            `json:"scenario"`
	Config   map[string]string `json:"config"`
}

// Validate validates the chaos experiment request.
func (r ChaosExperimentRequest) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.Scenario == "" {
		return fmt.Errorf("scenario is required")
	}
	return nil
}

// ChaosExperimentResponse represents a chaos experiment response.
type ChaosExperimentResponse struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Scenario  string            `json:"scenario"`
	Config    map[string]string `json:"config"`
	Status    string            `json:"status"`
	CreatedAt time.Time         `json:"created_at"`
}

// Validate validates the chaos experiment response.
func (r ChaosExperimentResponse) Validate() error {
	if r.ID == "" {
		return fmt.Errorf("id is required")
	}
	if r.Name == "" {
		return fmt.Errorf("name is required")
	}
	if r.Scenario == "" {
		return fmt.Errorf("scenario is required")
	}
	if r.Status == "" {
		return fmt.Errorf("status is required")
	}
	if r.CreatedAt.IsZero() {
		return fmt.Errorf("created_at is required")
	}
	return nil
}
