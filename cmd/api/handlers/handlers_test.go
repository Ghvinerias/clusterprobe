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
	"github.com/Ghvinerias/clusterprobe/internal/chaos"
	"github.com/Ghvinerias/clusterprobe/internal/workload"
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

type mockChaosEngine struct {
	applyFn  func(ctx context.Context, spec chaos.ExperimentSpec) (string, error)
	statusFn func(ctx context.Context, id string) (chaos.ExperimentStatus, error)
	deleteFn func(ctx context.Context, id string) error
	listFn   func(ctx context.Context) ([]chaos.ExperimentStatus, error)
}

func (m *mockChaosEngine) Apply(ctx context.Context, spec chaos.ExperimentSpec) (string, error) {
	return m.applyFn(ctx, spec)
}

func (m *mockChaosEngine) Status(ctx context.Context, id string) (chaos.ExperimentStatus, error) {
	return m.statusFn(ctx, id)
}

func (m *mockChaosEngine) Delete(ctx context.Context, id string) error {
	return m.deleteFn(ctx, id)
}

func (m *mockChaosEngine) List(ctx context.Context) ([]chaos.ExperimentStatus, error) {
	return m.listFn(ctx)
}

type mockMongo struct {
	insertFn func(ctx context.Context, collection string, doc any) error
	findFn   func(ctx context.Context, collection string, filter any) (MongoCursor, error)
}

func (m *mockMongo) InsertOne(ctx context.Context, collection string, doc any) error {
	return m.insertFn(ctx, collection, doc)
}

func (m *mockMongo) Find(ctx context.Context, collection string, filter any) (MongoCursor, error) {
	return m.findFn(ctx, collection, filter)
}

type mockCursor struct {
	rows []any
	idx  int
	err  error
}

func (m *mockCursor) Next(ctx context.Context) bool {
	m.idx++
	return m.idx <= len(m.rows)
}

func (m *mockCursor) Decode(val any) error {
	if m.err != nil {
		return m.err
	}
	switch dest := val.(type) {
	case *chaosRecord:
		*dest = m.rows[m.idx-1].(chaosRecord)
	case *workload.ChaosExperimentResponse:
		*dest = m.rows[m.idx-1].(workload.ChaosExperimentResponse)
	default:
		return errors.New("unsupported decode type")
	}
	return nil
}

func (m *mockCursor) Err() error { return m.err }

func (m *mockCursor) Close(ctx context.Context) error { return nil }

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

func buildTestServer(
	store PostgresStore,
	redis RedisStore,
	mongo MongoStore,
	publisher Publisher,
	chaosEngine ChaosEngine,
) http.Handler {
	scenarios := NewScenarioHandler(store, publisher)
	status := NewStatusHandler(store, redis)
	chaos := NewChaosHandler(mongo, chaosEngine)

	router := NewRouter(scenarios, status, chaos)

	mux := chi.NewRouter()
	mux.Use(middleware.OTelMiddleware("clusterprobe-api"))
	mux.Use(middleware.Logging)
	mux.Mount("/", router.Routes())
	return mux
}

func defaultMongo() *mockMongo {
	return &mockMongo{
		insertFn: func(ctx context.Context, collection string, doc any) error { return nil },
		findFn: func(ctx context.Context, collection string, filter any) (MongoCursor, error) {
			return &mockCursor{}, nil
		},
	}
}

func defaultChaosEngine() *mockChaosEngine {
	return &mockChaosEngine{
		applyFn: func(ctx context.Context, spec chaos.ExperimentSpec) (string, error) {
			return "chaos-id", nil
		},
		statusFn: func(ctx context.Context, id string) (chaos.ExperimentStatus, error) {
			return chaos.ExperimentStatus{ID: id, Name: id, Phase: "Pending"}, nil
		},
		deleteFn: func(ctx context.Context, id string) error { return nil },
		listFn: func(ctx context.Context) ([]chaos.ExperimentStatus, error) {
			return nil, nil
		},
	}
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

	buildTestServer(store, redis, defaultMongo(), publisher, defaultChaosEngine()).ServeHTTP(rec, req)

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

	buildTestServer(store, redis, defaultMongo(), publisher, defaultChaosEngine()).ServeHTTP(rec, req)

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

	buildTestServer(store, redis, defaultMongo(), publisher, defaultChaosEngine()).ServeHTTP(rec, req)

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

	buildTestServer(store, redis, defaultMongo(), publisher, defaultChaosEngine()).ServeHTTP(rec, req)

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

	buildTestServer(store, redis, defaultMongo(), publisher, defaultChaosEngine()).ServeHTTP(rec, req)

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

	buildTestServer(store, redis, defaultMongo(), publisher, defaultChaosEngine()).ServeHTTP(rec, req)

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

	buildTestServer(store, redis, defaultMongo(), publisher, defaultChaosEngine()).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestChaosCreate(t *testing.T) {
	store := &mockPostgres{execFn: func(ctx context.Context, sql string, args ...any) error { return nil }}
	publisher := &mockPublisher{publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error { return nil }}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) { return "0", nil },
	}
	mongo := &mockMongo{
		insertFn: func(ctx context.Context, collection string, doc any) error { return nil },
		findFn: func(ctx context.Context, collection string, filter any) (MongoCursor, error) {
			return &mockCursor{}, nil
		},
	}
	chaosEngine := &mockChaosEngine{
		applyFn: func(ctx context.Context, spec chaos.ExperimentSpec) (string, error) {
			return "exp-id", nil
		},
		statusFn: func(ctx context.Context, id string) (chaos.ExperimentStatus, error) {
			return chaos.ExperimentStatus{ID: id, Name: id, Phase: "Pending"}, nil
		},
		deleteFn: func(ctx context.Context, id string) error { return nil },
		listFn:   func(ctx context.Context) ([]chaos.ExperimentStatus, error) { return nil, nil },
	}

	payload := []byte(`{"name":"exp","scenario":"s1","config":{"type":"pod","target":"app.kubernetes.io/component=api","duration":"5m"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/chaos/experiments", bytes.NewReader(payload))
	rec := httptest.NewRecorder()

	buildTestServer(store, redis, mongo, publisher, chaosEngine).ServeHTTP(rec, req)

	if rec.Code != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", rec.Code)
	}
}

func TestChaosList(t *testing.T) {
	created := time.Now().UTC()
	store := &mockPostgres{execFn: func(ctx context.Context, sql string, args ...any) error { return nil }}
	publisher := &mockPublisher{publishFn: func(ctx context.Context, exchange, routingKey string, body []byte) error { return nil }}
	redis := &mockRedis{
		getFn: func(ctx context.Context, key string) (string, error) { return "0", nil },
	}
	mongo := &mockMongo{
		insertFn: func(ctx context.Context, collection string, doc any) error { return nil },
		findFn: func(ctx context.Context, collection string, filter any) (MongoCursor, error) {
			record := chaosRecord{
				ID:        "id",
				Name:      "exp",
				Scenario:  "s1",
				Status:    "queued",
				CreatedAt: created,
			}
			return &mockCursor{rows: []any{record}}, nil
		},
	}
	chaosEngine := defaultChaosEngine()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/chaos/experiments", nil)
	rec := httptest.NewRecorder()

	buildTestServer(store, redis, mongo, publisher, chaosEngine).ServeHTTP(rec, req)

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

	handler := buildTestServer(store, redis, defaultMongo(), publisher, defaultChaosEngine())
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
