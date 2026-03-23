package handlers

import (
	"bytes"
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Ghvinerias/clusterprobe/cmd/api/middleware"
	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/trace"
	collectortest "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	"google.golang.org/grpc"
)

type mockPostgres struct {
	execFn     func(ctx context.Context, sql string, args ...any) error
	queryFn    func(ctx context.Context, sql string, args ...any) (Rows, error)
	queryRowFn func(ctx context.Context, sql string, args ...any) Row
}

func (m *mockPostgres) Exec(ctx context.Context, sql string, args ...any) error {
	return m.execFn(ctx, sql, args...)
}

func (m *mockPostgres) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return m.queryFn(ctx, sql, args...)
}

func (m *mockPostgres) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return m.queryRowFn(ctx, sql, args...)
}

type mockRedis struct {
	getFn func(ctx context.Context, key string) (string, error)
}

func (m *mockRedis) Get(ctx context.Context, key string) (string, error) {
	return m.getFn(ctx, key)
}

type mockPublisher struct {
	publishFn func(ctx context.Context, exchange, routingKey string, body []byte) error
}

func (m *mockPublisher) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	return m.publishFn(ctx, exchange, routingKey, body)
}

type rowStub struct {
	values []any
	err    error
}

func (r *rowStub) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	return assign(dest, r.values)
}

type rowsStub struct {
	rows [][]any
	idx  int
	err  error
}

func (r *rowsStub) Close()                 {}
func (r *rowsStub) Err() error             { return r.err }
func (r *rowsStub) Next() bool             { r.idx++; return r.idx <= len(r.rows) }
func (r *rowsStub) Scan(dest ...any) error { return assign(dest, r.rows[r.idx-1]) }

func assign(dest []any, values []any) error {
	if len(dest) != len(values) {
		return errors.New("scan mismatch")
	}
	for i, v := range values {
		switch d := dest[i].(type) {
		case *string:
			*d = v.(string)
		case *[]byte:
			*d = v.([]byte)
		case *time.Time:
			*d = v.(time.Time)
		case *int:
			*d = v.(int)
		case *int64:
			*d = v.(int64)
		default:
			return errors.New("unsupported scan type")
		}
	}
	return nil
}

func buildTestServer(store PostgresStore, redis RedisStore, publisher Publisher) http.Handler {
	scenarios := NewScenarioHandler(store, publisher)
	status := NewStatusHandler(store, redis)
	chaos := NewChaosHandler(store, publisher)

	router := NewRouter(scenarios, status, chaos)

	mux := chi.NewRouter()
	mux.Use(middleware.OTelMiddleware("clusterprobe-api"))
	mux.Use(middleware.Logging)
	mux.Mount("/", router.Routes())
	return mux
}

func scenarioPayloadJSON() []byte {
	return []byte(`{
  "name": "test",
  "profile": {
    "rps": 1,
    "duration": 1000000000,
    "payload_size_bytes": 0,
    "concurrency": 1,
    "target_queue": "workload.high",
    "workload_type": "cpu_burn"
  }
}`)
}

func TestScenarioCreate(t *testing.T) {
	store := &mockPostgres{
		execFn: func(ctx context.Context, sql string, args ...any) error {
			return nil
		},
	}
	publisher := &mockPublisher{
		publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error {
			return nil
		},
	}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) { return "0", nil },
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scenarios", bytes.NewReader(scenarioPayloadJSON()))
	rec := httptest.NewRecorder()

	buildTestServer(store, redis, publisher).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestScenarioCreateError(t *testing.T) {
	store := &mockPostgres{
		execFn: func(ctx context.Context, sql string, args ...any) error {
			return errors.New("db error")
		},
	}
	publisher := &mockPublisher{
		publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error {
			return nil
		},
	}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) { return "0", nil },
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/scenarios", bytes.NewReader(scenarioPayloadJSON()))
	rec := httptest.NewRecorder()

	buildTestServer(store, redis, publisher).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestScenarioList(t *testing.T) {
	created := time.Now().UTC()
	rowPayload := []byte(`{
  "id": "id",
  "name": "test",
  "profile": {
    "rps": 1,
    "duration": 1000000000,
    "payload_size_bytes": 0,
    "concurrency": 1,
    "target_queue": "workload.high",
    "workload_type": "cpu_burn"
  },
  "status": "queued",
  "created_at": "` + created.Format(time.RFC3339Nano) + `"
}`)
	row := []any{"id", rowPayload, created}

	store := &mockPostgres{
		queryFn: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &rowsStub{rows: [][]any{row}}, nil
		},
	}
	publisher := &mockPublisher{
		publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error {
			return nil
		},
	}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) { return "0", nil },
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scenarios", nil)
	rec := httptest.NewRecorder()

	buildTestServer(store, redis, publisher).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestScenarioGetNotFound(t *testing.T) {
	store := &mockPostgres{
		queryRowFn: func(ctx context.Context, sql string, args ...any) Row {
			return &rowStub{err: errors.New("not found")}
		},
	}
	publisher := &mockPublisher{
		publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error {
			return nil
		},
	}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) { return "0", nil },
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/scenarios/abc", nil)
	rec := httptest.NewRecorder()

	buildTestServer(store, redis, publisher).ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestScenarioStopPublishError(t *testing.T) {
	store := &mockPostgres{
		execFn: func(ctx context.Context, sql string, args ...any) error { return nil },
	}
	publisher := &mockPublisher{
		publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error {
			return errors.New("publish")
		},
	}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) { return "0", nil },
	}

	req := httptest.NewRequest(http.MethodPut, "/api/v1/scenarios/abc/stop", nil)
	rec := httptest.NewRecorder()

	buildTestServer(store, redis, publisher).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}

func TestStatus(t *testing.T) {
	store := &mockPostgres{
		queryRowFn: func(ctx context.Context, sql string, args ...any) Row { return &rowStub{} },
	}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) {
			return "1", nil
		},
	}
	publisher := &mockPublisher{
		publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error {
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/status", nil)
	rec := httptest.NewRecorder()

	buildTestServer(store, redis, publisher).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestMetricsSnapshot(t *testing.T) {
	payload := []byte(`{"cpu":1}`)
	created := time.Now().UTC()
	store := &mockPostgres{
		queryRowFn: func(ctx context.Context, sql string, args ...any) Row {
			return &rowStub{values: []any{payload, created}}
		},
	}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) { return "0", nil },
	}
	publisher := &mockPublisher{
		publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error {
			return nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/metrics/snapshot", nil)
	rec := httptest.NewRecorder()

	buildTestServer(store, redis, publisher).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestChaosCreate(t *testing.T) {
	store := &mockPostgres{execFn: func(ctx context.Context, sql string, args ...any) error { return nil }}
	publisher := &mockPublisher{
		publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error {
			return nil
		},
	}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) { return "0", nil },
	}

	payload := []byte(`{"name":"exp","scenario":"s1","config":{"mode":"stress"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chaos/experiments", bytes.NewReader(payload))
	rec := httptest.NewRecorder()

	buildTestServer(store, redis, publisher).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestChaosList(t *testing.T) {
	created := time.Now().UTC()
	rowPayload := []byte(`{
  "id": "id",
  "name": "exp",
  "scenario": "s1",
  "status": "queued",
  "created_at": "` + created.Format(time.RFC3339Nano) + `"
}`)
	row := []any{"exp", rowPayload, created}

	store := &mockPostgres{
		queryFn: func(ctx context.Context, sql string, args ...any) (Rows, error) {
			return &rowsStub{rows: [][]any{row}}, nil
		},
	}
	publisher := &mockPublisher{
		publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error {
			return nil
		},
	}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) { return "0", nil },
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chaos/experiments", nil)
	rec := httptest.NewRecorder()

	buildTestServer(store, redis, publisher).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestOTELSpansCreated(t *testing.T) {
	collector, shutdown := startOTLPCollector(t)
	defer shutdown()

	exporter, err := otlptracegrpc.New(
		context.Background(),
		otlptracegrpc.WithEndpoint(collector.addr),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		t.Fatalf("exporter: %v", err)
	}

	provider := trace.NewTracerProvider(trace.WithBatcher(exporter))
	otel.SetTracerProvider(provider)

	store := &mockPostgres{
		execFn: func(ctx context.Context, sql string, args ...any) error { return nil },
	}
	publisher := &mockPublisher{
		publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error {
			return nil
		},
	}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) { return "0", nil },
	}

	handler := buildTestServer(store, redis, publisher)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/scenarios", bytes.NewReader(scenarioPayloadJSON()))
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}

	if err := provider.Shutdown(context.Background()); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	if collector.count() == 0 {
		t.Fatalf("expected spans exported")
	}
}

type otlpCollector struct {
	addr       string
	grpcServer *grpc.Server
	traceCount int64
}

type traceService struct {
	collectortest.UnimplementedTraceServiceServer
	parent *otlpCollector
}

func (s *traceService) Export(
	ctx context.Context,
	req *collectortest.ExportTraceServiceRequest,
) (*collectortest.ExportTraceServiceResponse, error) {
	atomic.AddInt64(&s.parent.traceCount, 1)
	return &collectortest.ExportTraceServiceResponse{}, nil
}

func startOTLPCollector(t *testing.T) (*otlpCollector, func()) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	collector := &otlpCollector{addr: listener.Addr().String()}
	collector.grpcServer = grpc.NewServer()
	collectortest.RegisterTraceServiceServer(collector.grpcServer, &traceService{parent: collector})

	go func() {
		_ = collector.grpcServer.Serve(listener)
	}()

	shutdown := func() {
		collector.grpcServer.Stop()
		_ = listener.Close()
	}

	return collector, shutdown
}

func (c *otlpCollector) count() int64 {
	return atomic.LoadInt64(&c.traceCount)
}
