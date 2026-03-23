package handlers

import (
	"context"
	"html/template"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
	"github.com/Ghvinerias/clusterprobe/web"
)

type mockAPI struct {
	scenarios   []workload.ScenarioResponse
	experiments []workload.ChaosExperimentResponse
	logStream   io.ReadCloser
}

func (m *mockAPI) ListScenarios(ctx context.Context) ([]workload.ScenarioResponse, error) {
	return m.scenarios, nil
}

func (m *mockAPI) CreateScenario(ctx context.Context, req workload.ScenarioRequest) (workload.ScenarioResponse, error) {
	return workload.ScenarioResponse{ID: "s1", Name: req.Name, Profile: req.Profile, Status: "queued", CreatedAt: time.Now()}, nil
}

func (m *mockAPI) StopScenario(ctx context.Context, id string) (workload.ScenarioResponse, error) {
	return workload.ScenarioResponse{ID: id, Status: "stopped", CreatedAt: time.Now()}, nil
}

func (m *mockAPI) ListExperiments(ctx context.Context) ([]workload.ChaosExperimentResponse, error) {
	return m.experiments, nil
}

func (m *mockAPI) CreateExperiment(ctx context.Context, req workload.ChaosExperimentRequest) (workload.ChaosExperimentResponse, error) {
	return workload.ChaosExperimentResponse{ID: "e1", Name: req.Name, Scenario: req.Scenario, Status: "queued", CreatedAt: time.Now()}, nil
}

func (m *mockAPI) LogsStream(ctx context.Context) (io.ReadCloser, error) {
	if m.logStream == nil {
		return io.NopCloser(strings.NewReader("event: logs\ndata: ready\n\n")), nil
	}
	return m.logStream, nil
}

type mockCounters struct {
	values map[string]string
}

func (m *mockCounters) Get(ctx context.Context, key string) (string, error) {
	if m.values == nil {
		return "0", nil
	}
	value, ok := m.values[key]
	if !ok {
		return "0", nil
	}
	return value, nil
}

func newTestServer(t *testing.T) *Server {
	tmpls := parseTemplatesForTest(t)
	api := &mockAPI{}
	counters := &mockCounters{}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return NewServer(tmpls, api, counters, logger)
}

func parseTemplatesForTest(t *testing.T) map[string]*template.Template {
	base, err := template.ParseFS(
		web.FS,
		"templates/base.html",
		"templates/partials/*.html",
	)
	if err != nil {
		t.Fatalf("parse templates: %v", err)
	}
	pageFiles, err := fs.Glob(web.FS, "templates/pages/*.html")
	if err != nil {
		t.Fatalf("glob templates: %v", err)
	}
	templates := make(map[string]*template.Template, len(pageFiles))
	for _, file := range pageFiles {
		cloned, err := base.Clone()
		if err != nil {
			t.Fatalf("clone templates: %v", err)
		}
		if _, err := cloned.ParseFS(web.FS, file); err != nil {
			t.Fatalf("parse page template: %v", err)
		}
		name := strings.TrimSuffix(path.Base(file), ".html")
		templates[name] = cloned
	}
	return templates
}

func TestDashboardHandler(t *testing.T) {
	server := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/dashboard", nil)
	rec := httptest.NewRecorder()

	server.Dashboard(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected html content type")
	}
}

func TestScenariosHandler(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/scenarios", nil)
	rec := httptest.NewRecorder()

	server.ListScenarios(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected html content type")
	}
}

func TestScenarioNewHandler(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/scenarios/new", nil)
	rec := httptest.NewRecorder()

	server.NewScenario(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected html content type")
	}
}

func TestChaosHandler(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/chaos", nil)
	rec := httptest.NewRecorder()

	server.ListChaos(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected html content type")
	}
}

func TestChaosNewHandler(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/chaos/new", nil)
	rec := httptest.NewRecorder()

	server.NewChaos(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected html content type")
	}
}

func TestLogsHandler(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/logs", nil)
	rec := httptest.NewRecorder()

	server.Logs(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if ct := res.Header.Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("expected html content type")
	}
}

func TestDashboardStream(t *testing.T) {
	server := newTestServer(t)

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/dashboard/stream", nil).WithContext(ctx)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		server.DashboardStream(rec, req)
		close(done)
	}()

	time.Sleep(3 * time.Second)
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected stream shutdown")
	}

	if !strings.Contains(rec.Body.String(), "event: stats") {
		t.Fatalf("expected stats event")
	}
}

func TestLogsStream(t *testing.T) {
	server := newTestServer(t)
	req := httptest.NewRequest(http.MethodGet, "/logs/stream", nil)
	rec := httptest.NewRecorder()

	server.LogsStream(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if !strings.Contains(rec.Body.String(), "event: logs") {
		t.Fatalf("expected log stream")
	}
}
