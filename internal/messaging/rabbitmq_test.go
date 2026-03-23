package messaging

import (
	"context"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type mockConn struct {
	ch       *mockChannel
	mu       sync.Mutex
	notifyCh chan *amqp.Error
	closed   bool
}

func (m *mockConn) Channel() (amqpChannel, error) {
	return m.ch, nil
}

func (m *mockConn) NotifyClose(c chan *amqp.Error) chan *amqp.Error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.notifyCh = c
	return c
}

func (m *mockConn) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.closed = true
	return nil
}

func (m *mockConn) notifyChannel() chan *amqp.Error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.notifyCh
}

type mockChannel struct {
	publishCalls []amqp.Publishing
	mu           sync.Mutex
	queues       []string
	exchanges    []string
	bindings     []string
	consumeCh    chan amqp.Delivery
	qosCalls     int
}

func (m *mockChannel) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishCalls = append(m.publishCalls, msg)
	return nil
}

func (m *mockChannel) ExchangeDeclare(
	name string,
	kind string,
	durable bool,
	autoDelete bool,
	internal bool,
	noWait bool,
	args amqp.Table,
) error {
	m.exchanges = append(m.exchanges, name)
	return nil
}

func (m *mockChannel) QueueDeclare(
	name string,
	durable bool,
	autoDelete bool,
	exclusive bool,
	noWait bool,
	args amqp.Table,
) (amqp.Queue, error) {
	m.queues = append(m.queues, name)
	return amqp.Queue{Name: name}, nil
}

func (m *mockChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	m.bindings = append(m.bindings, name+":"+key+":"+exchange)
	return nil
}

func (m *mockChannel) Qos(prefetchCount, prefetchSize int, global bool) error {
	m.qosCalls++
	return nil
}

func (m *mockChannel) Consume(
	queue string,
	consumer string,
	autoAck bool,
	exclusive bool,
	noLocal bool,
	noWait bool,
	args amqp.Table,
) (<-chan amqp.Delivery, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.consumeCh == nil {
		m.consumeCh = make(chan amqp.Delivery)
	}
	return m.consumeCh, nil
}

func (m *mockChannel) Close() error {
	return nil
}

func TestPublishInjectsTraceHeaders(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
	})

	ch := &mockChannel{}
	conn := &mockConn{ch: ch}
	producer := &Producer{
		conn:    conn,
		channel: ch,
		tracer:  otel.Tracer("test"),
	}

	ctx, span := producer.tracer.Start(context.Background(), "span")
	defer span.End()

	if err := producer.Publish(ctx, exchangeName, routingHigh, []byte("{}")); err != nil {
		t.Fatalf("publish: %v", err)
	}

	if len(ch.publishCalls) != 1 {
		t.Fatalf("expected publish call")
	}

	msg := ch.publishCalls[0]
	if _, ok := msg.Headers["traceparent"]; !ok {
		t.Fatalf("expected traceparent header")
	}
}

func TestDeclareTopology(t *testing.T) {
	ch := &mockChannel{}
	producer := &Producer{channel: ch}

	if err := producer.DeclareTopology(context.Background()); err != nil {
		t.Fatalf("declare topology: %v", err)
	}

	if len(ch.exchanges) != 2 {
		t.Fatalf("expected exchanges declared")
	}
	if len(ch.queues) != 6 {
		t.Fatalf("expected queues declared")
	}
	if len(ch.bindings) != 6 {
		t.Fatalf("expected bindings declared")
	}
}

func TestReconnectOnClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var dialCount int
	var mu sync.Mutex

	dialer := func(ctx context.Context) (amqpConn, error) {
		mu.Lock()
		dialCount++
		mu.Unlock()
		return &mockConn{ch: &mockChannel{}}, nil
	}

	producer := &Producer{
		dial:        dialer,
		reconnectCh: make(chan struct{}, 1),
		tracer:      otel.Tracer("test"),
	}

	if err := producer.connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}

	go producer.monitorReconnect(ctx)

	producer.mu.RLock()
	conn := producer.conn.(*mockConn)
	producer.mu.RUnlock()

	notifyDeadline := time.Now().Add(500 * time.Millisecond)
	for {
		notifyCh := conn.notifyChannel()
		if notifyCh != nil {
			notifyCh <- &amqp.Error{Reason: "closed"}
			break
		}
		if time.Now().After(notifyDeadline) {
			t.Fatalf("notify channel not set")
		}
		time.Sleep(10 * time.Millisecond)
	}

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		count := dialCount
		mu.Unlock()
		if count >= 2 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	mu.Lock()
	count := dialCount
	mu.Unlock()
	if count < 2 {
		t.Fatalf("expected reconnect, dial count: %d", count)
	}
}

func TestHeaderCarrier(t *testing.T) {
	carrier := amqpHeaderCarrier(amqp.Table{})
	carrier.Set("traceparent", "value")
	if carrier.Get("traceparent") != "value" {
		t.Fatalf("expected value")
	}
	if len(carrier.Keys()) != 1 {
		t.Fatalf("expected keys")
	}
}

func TestProducerCloseNoError(t *testing.T) {
	producer := &Producer{
		conn:    &mockConn{ch: &mockChannel{}},
		channel: &mockChannel{},
	}

	if err := producer.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

func TestPublishNoChannel(t *testing.T) {
	producer := &Producer{tracer: noop.NewTracerProvider().Tracer("test")}
	if err := producer.Publish(context.Background(), exchangeName, routingHigh, []byte("{}")); err == nil {
		t.Fatalf("expected error")
	}
}

func TestConsumerReconnect(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var dialCount int
	var dialMu sync.Mutex

	dialer := func(ctx context.Context) (amqpConn, error) {
		dialMu.Lock()
		dialCount++
		dialMu.Unlock()
		return &mockConn{ch: &mockChannel{}}, nil
	}

	consumer := &Consumer{
		dial:     dialer,
		prefetch: 1,
		tracer:   noop.NewTracerProvider().Tracer("test"),
	}

	if err := consumer.connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}

	go consumer.monitorReconnect(ctx)

	consumer.mu.RLock()
	conn := consumer.conn.(*mockConn)
	consumer.mu.RUnlock()

	notifyDeadline := time.Now().Add(500 * time.Millisecond)
	for {
		notifyCh := conn.notifyChannel()
		if notifyCh != nil {
			notifyCh <- &amqp.Error{Reason: "closed"}
			break
		}
		if time.Now().After(notifyDeadline) {
			t.Fatalf("notify channel not set")
		}
		time.Sleep(10 * time.Millisecond)
	}

	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		dialMu.Lock()
		count := dialCount
		dialMu.Unlock()
		if count >= 2 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}

	dialMu.Lock()
	count := dialCount
	dialMu.Unlock()
	if count < 2 {
		t.Fatalf("expected reconnect, dial count: %d", count)
	}
}
