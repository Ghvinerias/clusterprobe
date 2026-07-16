package main

import (
	"context"
	"encoding/json"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
)

type fakeGenerator struct {
	mu     sync.Mutex
	called bool
	params workload.WorkloadParams
	result workload.Result
	err    error
}

func (g *fakeGenerator) Execute(ctx context.Context, params workload.WorkloadParams) (workload.Result, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.called = true
	g.params = params
	return g.result, g.err
}

func (g *fakeGenerator) Called() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.called
}

type fakeStore struct {
	execCalls      int
	execSQL        []string
	execArgs       [][]any
	latestStatus   string
	latestStatuses []string
}

func (s *fakeStore) Exec(ctx context.Context, sql string, args ...any) error {
	s.execCalls++
	s.execSQL = append(s.execSQL, sql)
	s.execArgs = append(s.execArgs, args)
	return nil
}

func (s *fakeStore) Query(ctx context.Context, sql string, args ...any) (workload.Rows, error) {
	return &mockRows{}, nil
}

func (s *fakeStore) QueryRow(ctx context.Context, sql string, args ...any) workload.Row {
	status := s.latestStatus
	if len(s.latestStatuses) > 0 {
		status = s.latestStatuses[0]
		s.latestStatuses = s.latestStatuses[1:]
	}
	if status == "" {
		status = "running"
	}
	payload, _ := json.Marshal(workload.ScenarioResponse{Status: status})
	return &mockRow{values: []any{payload}}
}

type mockRow struct {
	values []any
}

func (r *mockRow) Scan(dest ...any) error {
	if len(r.values) == 0 {
		return nil
	}
	for i, value := range r.values {
		switch d := dest[i].(type) {
		case *[]byte:
			*d = value.([]byte)
		}
	}
	return nil
}

type mockRows struct{}

func (r *mockRows) Close()                 {}
func (r *mockRows) Err() error             { return nil }
func (r *mockRows) Next() bool             { return false }
func (r *mockRows) Scan(dest ...any) error { return nil }

type fakeRedis struct {
	counters map[string]int
}

func (r *fakeRedis) Incr(ctx context.Context, key string) error {
	if r.counters == nil {
		r.counters = make(map[string]int)
	}
	r.counters[key]++
	return nil
}

type fakePublisher struct {
	exchange   string
	routingKey string
	payload    []byte
}

func (p *fakePublisher) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	p.exchange = exchange
	p.routingKey = routingKey
	p.payload = body
	return nil
}

func TestHandleMessageDispatch(t *testing.T) {
	gen := &fakeGenerator{result: workload.Result{Ops: 1, Duration: 1 * time.Millisecond}}
	gens := map[workload.WorkloadType]workload.Generator{
		workload.WorkloadTypeCPUBurn: gen,
	}
	store := &fakeStore{}
	redis := &fakeRedis{}
	publisher := &fakePublisher{}

	scenario := workload.ScenarioResponse{
		ID: "scenario-1",
		Profile: workload.LoadProfile{
			WorkloadType:     workload.WorkloadTypeCPUBurn,
			Duration:         time.Millisecond,
			PayloadSizeBytes: 1024 * 1024,
			Concurrency:      2,
		},
	}
	body, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	msg := amqp.Delivery{Body: body}
	if err := handleMessage(context.Background(), msg, gens, store, redis, publisher); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	if !gen.Called() {
		t.Fatalf("expected generator to be called")
	}
	if store.execCalls == 0 {
		t.Fatalf("expected snapshot insert")
	}
	if !hasScenarioEventInsert(store) {
		t.Fatalf("expected scenario status inserts to use scenario_events")
	}
	if redis.counters["cp:ops:total"] == 0 {
		t.Fatalf("expected ops counter")
	}
	if publisher.routingKey == "" {
		t.Fatalf("expected publish")
	}

	statuses := scenarioStatuses(t, store)
	if len(statuses) != 2 {
		t.Fatalf("expected running and completed status events, got %v", statuses)
	}
	if statuses[0] != "running" {
		t.Fatalf("expected first status running, got %s", statuses[0])
	}
	if statuses[1] != "completed" {
		t.Fatalf("expected second status completed, got %s", statuses[1])
	}
}

func TestHandleMessageMarksScenarioFailed(t *testing.T) {
	gen := &fakeGenerator{
		result: workload.Result{Ops: 1, Duration: time.Millisecond},
		err:    assertError("generator failed"),
	}
	gens := map[workload.WorkloadType]workload.Generator{
		workload.WorkloadTypeCPUBurn: gen,
	}
	store := &fakeStore{}
	redis := &fakeRedis{}
	publisher := &fakePublisher{}

	scenario := workload.ScenarioResponse{
		ID:        "scenario-1",
		Name:      "scenario",
		Status:    "queued",
		CreatedAt: time.Now().UTC(),
		Profile: workload.LoadProfile{
			WorkloadType:     workload.WorkloadTypeCPUBurn,
			Duration:         time.Millisecond,
			PayloadSizeBytes: 1024 * 1024,
			Concurrency:      2,
			RPS:              1,
			TargetQueue:      "workload.high",
		},
	}
	body, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	msg := amqp.Delivery{Body: body}
	if err := handleMessage(context.Background(), msg, gens, store, redis, publisher); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	statuses := scenarioStatuses(t, store)
	if len(statuses) != 2 {
		t.Fatalf("expected running and failed status events, got %v", statuses)
	}
	if statuses[1] != "failed" {
		t.Fatalf("expected terminal status failed, got %s", statuses[1])
	}
	if redis.counters["cp:errors:total"] == 0 {
		t.Fatalf("expected errors counter")
	}
}

func TestHandleMessagePreservesStoppedScenario(t *testing.T) {
	gen := &fakeGenerator{result: workload.Result{Ops: 1, Duration: time.Millisecond}}
	gens := map[workload.WorkloadType]workload.Generator{
		workload.WorkloadTypeCPUBurn: gen,
	}
	store := &fakeStore{latestStatuses: []string{"running", "stopped"}}
	redis := &fakeRedis{}
	publisher := &fakePublisher{}

	scenario := workload.ScenarioResponse{
		ID:        "scenario-1",
		Name:      "scenario",
		Status:    "queued",
		CreatedAt: time.Now().UTC(),
		Profile: workload.LoadProfile{
			WorkloadType:     workload.WorkloadTypeCPUBurn,
			Duration:         time.Millisecond,
			PayloadSizeBytes: 1024 * 1024,
			Concurrency:      2,
			RPS:              1,
			TargetQueue:      "workload.high",
		},
	}
	body, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	msg := amqp.Delivery{Body: body}
	if err := handleMessage(context.Background(), msg, gens, store, redis, publisher); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}

	statuses := scenarioStatuses(t, store)
	if len(statuses) != 1 {
		t.Fatalf("expected only running status event, got %v", statuses)
	}
	if statuses[0] != "running" {
		t.Fatalf("expected running status, got %s", statuses[0])
	}
}

func TestHandleMessageSkipsPreStoppedScenario(t *testing.T) {
	gen := &fakeGenerator{result: workload.Result{Ops: 1, Duration: time.Millisecond}}
	gens := map[workload.WorkloadType]workload.Generator{
		workload.WorkloadTypeCPUBurn: gen,
	}
	store := &fakeStore{latestStatus: "stopped"}
	redis := &fakeRedis{}
	publisher := &fakePublisher{}

	scenario := workload.ScenarioResponse{
		ID:        "scenario-1",
		Name:      "scenario",
		Status:    "queued",
		CreatedAt: time.Now().UTC(),
		Profile: workload.LoadProfile{
			WorkloadType:     workload.WorkloadTypeCPUBurn,
			Duration:         time.Millisecond,
			PayloadSizeBytes: 1024 * 1024,
			Concurrency:      2,
			RPS:              1,
			TargetQueue:      "workload.high",
		},
	}
	body, err := json.Marshal(scenario)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	msg := amqp.Delivery{Body: body}
	if err := handleMessage(context.Background(), msg, gens, store, redis, publisher); err != nil {
		t.Fatalf("handleMessage: %v", err)
	}
	if gen.Called() {
		t.Fatalf("expected generator not to run")
	}
	if store.execCalls != 0 {
		t.Fatalf("expected no status or snapshot inserts, got %d", store.execCalls)
	}
}

type assertError string

func (e assertError) Error() string { return string(e) }

func scenarioStatuses(t *testing.T, store *fakeStore) []string {
	t.Helper()

	statuses := make([]string, 0, len(store.execArgs))
	for _, args := range store.execArgs {
		if len(args) < 2 {
			continue
		}

		payload, ok := args[1].([]byte)
		if !ok {
			continue
		}

		var scenario workload.ScenarioResponse
		if err := json.Unmarshal(payload, &scenario); err != nil {
			continue
		}
		if scenario.Status == "running" || scenario.Status == "completed" || scenario.Status == "failed" {
			statuses = append(statuses, scenario.Status)
		}
	}

	return statuses
}

func hasScenarioEventInsert(store *fakeStore) bool {
	for _, sql := range store.execSQL {
		if sql == insertScenarioQuery {
			return true
		}
	}
	return false
}

type fakeConsumer struct {
	consumeFn func(ctx context.Context, queue string, handler func(context.Context, amqp.Delivery) error) error
}

func (c *fakeConsumer) Consume(
	ctx context.Context,
	queue string,
	handler func(context.Context, amqp.Delivery) error,
) error {
	return c.consumeFn(ctx, queue, handler)
}

func TestStartConsumersGracefulShutdown(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	started := make(chan struct{})
	finish := make(chan struct{})

	consumerFactory := func() (consumer, error) {
		return &fakeConsumer{consumeFn: func(
			ctx context.Context,
			queue string,
			handler func(context.Context, amqp.Delivery) error,
		) error {
			close(started)
			if err := handler(ctx, amqp.Delivery{Body: []byte("{}")}); err != nil {
				return err
			}
			<-ctx.Done()
			<-finish
			return nil
		}}, nil
	}

	queuePicker := func(i int) string { return "queue" }
	called := make(chan struct{})
	handler := func(ctx context.Context, msg amqp.Delivery) error {
		close(called)
		return nil
	}

	go func() {
		<-called
		cancel()
		close(finish)
	}()

	errCh := make(chan error, 1)
	go func() {
		errCh <- startConsumers(ctx, 1, queuePicker, consumerFactory, handler, nil)
	}()

	<-started
	if err := <-errCh; err != nil {
		t.Fatalf("startConsumers: %v", err)
	}
}
