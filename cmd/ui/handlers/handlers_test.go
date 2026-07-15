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

	"github.com/go-chi/chi/v5"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
	"github.com/Ghvinerias/clusterprobe/web"
)

type mockAPI struct {
	scenarios   []workload.ScenarioResponse
	experiments []workload.ChaosExperimentResponse
	logStream   io.ReadCloser
	deleted     string
}

func (m *mockAPI) ListScenarios(ctx context.Context) ([]workload.ScenarioResponse, error) {
	return m.scenarios, nil
}

func (m *mockAPI) CreateScenario(ctx context.Context, req workload.ScenarioRequest) (workload.ScenarioResponse, error) {
	return workload.ScenarioResponse{
		ID:        "s1",
		Name:      req.Name,
		Profile:   req.Profile,
		Status:    "queued",
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockAPI) GetScenario(ctx context.Context, id string) (workload.ScenarioResponse, error) {
	for _, scenario := range m.scenarios {
		if scenario.ID == id {
			return scenario, nil
		}
	}
	return workload.ScenarioResponse{
		ID:        id,
		Name:      "scenario",
		Status:    "running",
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockAPI) StopScenario(ctx context.Context, id string) (workload.ScenarioResponse, error) {
	return workload.ScenarioResponse{ID: id, Status: "stopped", CreatedAt: time.Now()}, nil
}

func (m *mockAPI) ListExperiments(ctx context.Context) ([]workload.ChaosExperimentResponse, error) {
	return m.experiments, nil
}

func (m *mockAPI) CreateExperiment(
	ctx context.Context,
	req workload.ChaosExperimentRequest,
) (workload.ChaosExperimentResponse, error) {
	return workload.ChaosExperimentResponse{
		ID:        "e1",
		Name:      req.Name,
		Scenario:  req.Scenario,
		Status:    "queued",
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockAPI) GetExperiment(ctx context.Context, id string) (workload.ChaosExperimentResponse, error) {
	return workload.ChaosExperimentResponse{
		ID:        id,
		Name:      "pod-kill",
		Scenario:  "s1",
		Status:    "running",
		CreatedAt: time.Now(),
	}, nil
}

func (m *mockAPI) DeleteExperiment(ctx context.Context, id string) error {
	m.deleted = id
	return nil
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
	server.api = &mockAPI{
		scenarios: []workload.ScenarioResponse{
			{
				ID:        "s1",
				Name:      "running",
				Status:    "running",
				CreatedAt: time.Now(),
				Profile: workload.LoadProfile{
					WorkloadType: workload.WorkloadTypeDBWrite,
				},
			},
			{
				ID:        "s2",
				Name:      "completed",
				Status:    "completed",
				CreatedAt: time.Now(),
				Profile: workload.LoadProfile{
					WorkloadType: workload.WorkloadTypeDBRead,
				},
			},
		},
	}
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
	body := rec.Body.String()
	if !strings.Contains(body, `hx-get="/scenarios/s1/row"`) {
		t.Fatalf("expected running scenario row to poll")
	}
	if strings.Contains(body, `hx-get="/scenarios/s2/row"`) {
		t.Fatalf("expected completed scenario row not to poll")
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
	server.api = &mockAPI{
		experiments: []workload.ChaosExperimentResponse{
			{
				ID:        "e1",
				Name:      "pod-kill",
				Scenario:  "s1",
				Status:    "running",
				CreatedAt: time.Now(),
			},
		},
	}
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
	if !strings.Contains(rec.Body.String(), `hx-get="/chaos/e1/status"`) {
		t.Fatalf("expected UI status route in chaos table")
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

func TestChaosStatusHandler(t *testing.T) {
	server := newTestServer(t)
	req := requestWithRouteParam(http.MethodGet, "/chaos/e1/status", "id", "e1")
	rec := httptest.NewRecorder()

	server.ChaosStatus(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `hx-get="/chaos/e1/status"`) {
		t.Fatalf("expected self-refreshing status badge")
	}
	if !strings.Contains(body, "running") {
		t.Fatalf("expected running status")
	}
}

func TestDeleteChaosHandler(t *testing.T) {
	api := &mockAPI{}
	server := newTestServer(t)
	server.api = api
	req := requestWithRouteParam(http.MethodDelete, "/chaos/e1", "id", "e1")
	rec := httptest.NewRecorder()

	server.DeleteChaos(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	if api.deleted != "e1" {
		t.Fatalf("expected delete call for e1, got %q", api.deleted)
	}
}

func TestScenarioRowHandler(t *testing.T) {
	server := newTestServer(t)
	server.api = &mockAPI{
		scenarios: []workload.ScenarioResponse{
			{
				ID:        "s1",
				Name:      "scenario",
				Status:    "running",
				CreatedAt: time.Now(),
				Profile: workload.LoadProfile{
					WorkloadType: workload.WorkloadTypeDBWrite,
				},
			},
		},
	}
	req := requestWithRouteParam(http.MethodGet, "/scenarios/s1/row", "id", "s1")
	rec := httptest.NewRecorder()

	server.ScenarioRow(rec, req)

	res := rec.Result()
	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `hx-get="/scenarios/s1/row"`) {
		t.Fatalf("expected self-refreshing scenario row")
	}
	if !strings.Contains(body, "running") {
		t.Fatalf("expected running status")
	}
}

func requestWithRouteParam(method string, target string, key string, value string) *http.Request {
	req := httptest.NewRequest(method, target, nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add(key, value)
	return req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
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
