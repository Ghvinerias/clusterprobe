package handlers

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

const (
	uiScope          = "clusterprobe/ui"
	reportingBackoff = 2 * time.Second
)

// Server holds UI handler dependencies.
type Server struct {
	templates map[string]*template.Template
	api       APIClient
	counters  CounterStore
	logger    *slog.Logger
}

// NewServer constructs a UI server.
func NewServer(
	templates map[string]*template.Template,
	api APIClient,
	counters CounterStore,
	logger *slog.Logger,
) *Server {
	return &Server{
		templates: templates,
		api:       api,
		counters:  counters,
		logger:    logger,
	}
}

// RenderTemplate renders a named template.
func (s *Server) RenderTemplate(w http.ResponseWriter, page string, data any) error {
	tmpl, ok := s.templates[page]
	if !ok {
		return fmt.Errorf("template not found: %s", page)
	}
	buf := &bytes.Buffer{}
	if err := tmpl.ExecuteTemplate(buf, "base", data); err != nil {
		return fmt.Errorf("execute template %s: %w", page, err)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, err := w.Write(buf.Bytes())
	if err != nil {
		return fmt.Errorf("write template response: %w", err)
	}
	return nil
}

func statusClass(status string) string {
	lower := strings.ToLower(status)
	switch {
	case strings.Contains(lower, "error"):
		return "error"
	case strings.Contains(lower, "fail"):
		return "error"
	case strings.Contains(lower, "stop"):
		return "warn"
	case strings.Contains(lower, "queue"):
		return "warn"
	case strings.Contains(lower, "running"):
		return "success"
	default:
		return ""
	}
}

func parseInt(value string) int64 {
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0
	}
	return parsed
}

func (s *Server) buildStats(ctx context.Context, prevOps int64, prevTime time.Time) ([]Stat, int64, time.Time) {
	opsRaw, _ := s.counters.Get(ctx, "cp:ops:total")
	errorsRaw, _ := s.counters.Get(ctx, "cp:errors:total")
	scenariosRaw, _ := s.counters.Get(ctx, "scenarios.running")
	queueRaw, _ := s.counters.Get(ctx, "queue.depth")

	opCount := parseInt(opsRaw)
	errorCount := parseInt(errorsRaw)
	activeScenarios := parseInt(scenariosRaw)
	queueDepth := parseInt(queueRaw)

	now := time.Now()
	deltaSeconds := now.Sub(prevTime).Seconds()
	opRate := int64(0)
	if deltaSeconds > 0 && prevTime.After(time.Time{}) {
		opRate = int64(float64(opCount-prevOps) / deltaSeconds)
	}

	errorRate := "0%"
	if opCount > 0 {
		errorRate = fmt.Sprintf("%.2f%%", (float64(errorCount)/float64(opCount))*100)
	}

	stats := []Stat{
		{Label: "Ops/sec", Value: fmt.Sprintf("%d", opRate), Status: "active", StatusClass: "success"},
		{Label: "Error Rate", Value: errorRate, Status: "monitor", StatusClass: "warn"},
		{Label: "Active Scenarios", Value: fmt.Sprintf("%d", activeScenarios), Status: "live", StatusClass: "success"},
		{Label: "Queue Depth", Value: fmt.Sprintf("%d", queueDepth), Status: "signal", StatusClass: ""},
	}

	return stats, opCount, now
}

func parseDurationSeconds(value string) (time.Duration, error) {
	seconds, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("duration must be integer seconds")
	}
	if seconds <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return time.Duration(seconds) * time.Second, nil
}

func parseIntField(value string, field string) (int, error) {
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be integer", field)
	}
	return parsed, nil
}

func parseChaosParameters(value string) map[string]string {
	params := map[string]string{}
	lines := strings.Split(value, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		params[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	return params
}

func (s *Server) logRequest(r *http.Request, name string) {
	s.logger.Info(
		"ui_request",
		"handler", name,
		"path", r.URL.Path,
	)
}

func (s *Server) newSpan(ctx context.Context, name string) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(uiScope).Start(ctx, name)
	return ctx, span
}

func buildScenarioView(scenario workload.ScenarioResponse) ScenarioView {
	return ScenarioView{ScenarioResponse: scenario, StatusClass: statusClass(scenario.Status)}
}

func buildExperimentView(exp workload.ChaosExperimentResponse) ExperimentView {
	return ExperimentView{ChaosExperimentResponse: exp, StatusClass: statusClass(exp.Status)}
}
