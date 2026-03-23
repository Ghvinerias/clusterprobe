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
	execCalls int
	lastArgs  []any
}

func (s *fakeStore) Exec(ctx context.Context, sql string, args ...any) error {
	s.execCalls++
	s.lastArgs = args
	return nil
}

func (s *fakeStore) Query(ctx context.Context, sql string, args ...any) (workload.Rows, error) {
	return &mockRows{}, nil
}

func (s *fakeStore) QueryRow(ctx context.Context, sql string, args ...any) workload.Row {
	return &mockRow{}
}

type mockRow struct{}

func (r *mockRow) Scan(dest ...any) error { return nil }

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
	if redis.counters["cp:ops:total"] == 0 {
		t.Fatalf("expected ops counter")
	}
	if publisher.routingKey == "" {
		t.Fatalf("expected publish")
	}
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
		errCh <- startConsumers(ctx, 1, queuePicker, consumerFactory, handler)
	}()

	<-started
	if err := <-errCh; err != nil {
		t.Fatalf("startConsumers: %v", err)
	}
}
