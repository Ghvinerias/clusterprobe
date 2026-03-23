package messaging

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

type mockConn struct {
	ch         amqpChannel
	channelErr error
	mu         sync.Mutex
	notifyCh   chan *amqp.Error
	closed     bool
}

func (m *mockConn) Channel() (amqpChannel, error) {
	if m.channelErr != nil {
		return nil, m.channelErr
	}
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
	publishErr   error
	exchangeErr  error
	exchangeName string
	queueErr     error
	queueName    string
	bindErr      error
	qosErr       error
	consumeErr   error
}

func (m *mockChannel) Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.publishCalls = append(m.publishCalls, msg)
	if m.publishErr != nil {
		return m.publishErr
	}
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
	if m.exchangeErr != nil && (m.exchangeName == "" || m.exchangeName == name) {
		return m.exchangeErr
	}
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
	if m.queueErr != nil && (m.queueName == "" || m.queueName == name) {
		return amqp.Queue{}, m.queueErr
	}
	return amqp.Queue{Name: name}, nil
}

func (m *mockChannel) QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error {
	m.bindings = append(m.bindings, name+":"+key+":"+exchange)
	if m.bindErr != nil {
		return m.bindErr
	}
	return nil
}

func (m *mockChannel) Qos(prefetchCount, prefetchSize int, global bool) error {
	m.qosCalls++
	if m.qosErr != nil {
		return m.qosErr
	}
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
	if m.consumeErr != nil {
		return nil, m.consumeErr
	}
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

func TestProducerValidationAndConnectErrors(t *testing.T) {
	_, err := NewProducer(context.Background(), "")
	require.Error(t, err)

	producer := &Producer{
		dial: func(ctx context.Context) (amqpConn, error) {
			return nil, errors.New("dial failed")
		},
		reconnectCh: make(chan struct{}, 1),
		tracer:      noop.NewTracerProvider().Tracer("test"),
	}

	err = producer.connect(context.Background())
	require.Error(t, err)
}

func TestPublishErrorFromChannel(t *testing.T) {
	ch := &mockChannel{publishErr: errors.New("publish failed")}
	producer := &Producer{
		conn:    &mockConn{ch: ch},
		channel: ch,
		tracer:  noop.NewTracerProvider().Tracer("test"),
	}

	err := producer.Publish(context.Background(), exchangeName, routingHigh, []byte("{}"))
	require.Error(t, err)
}

func TestDeclareTopologyErrors(t *testing.T) {
	t.Run("no channel", func(t *testing.T) {
		producer := &Producer{}
		require.Error(t, producer.DeclareTopology(context.Background()))
	})

	t.Run("exchange error", func(t *testing.T) {
		ch := &mockChannel{exchangeErr: errors.New("exchange failed"), exchangeName: exchangeName}
		producer := &Producer{channel: ch}
		require.Error(t, producer.DeclareTopology(context.Background()))
	})

	t.Run("dlx error", func(t *testing.T) {
		ch := &mockChannel{exchangeErr: errors.New("dlx failed"), exchangeName: dlxName}
		producer := &Producer{channel: ch}
		require.Error(t, producer.DeclareTopology(context.Background()))
	})

	t.Run("queue declare error", func(t *testing.T) {
		ch := &mockChannel{queueErr: errors.New("queue failed")}
		producer := &Producer{channel: ch}
		require.Error(t, producer.DeclareTopology(context.Background()))
	})

	t.Run("queue bind error", func(t *testing.T) {
		ch := &mockChannel{bindErr: errors.New("bind failed")}
		producer := &Producer{channel: ch}
		require.Error(t, producer.DeclareTopology(context.Background()))
	})
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

func TestConsumerValidation(t *testing.T) {
	_, err := NewConsumer(context.Background(), "", 1)
	require.Error(t, err)
}

func TestConsumeOnceErrors(t *testing.T) {
	consumer := &Consumer{tracer: noop.NewTracerProvider().Tracer("test")}

	_, err := consumer.consumeOnce(context.Background())
	require.Error(t, err)

	consumer.channel = &mockChannel{qosErr: errors.New("qos failed")}
	consumer.queue = "queue"
	_, err = consumer.consumeOnce(context.Background())
	require.Error(t, err)

	consumer.channel = &mockChannel{consumeErr: errors.New("consume failed")}
	_, err = consumer.consumeOnce(context.Background())
	require.Error(t, err)
}

func TestConsumeValidation(t *testing.T) {
	consumer := &Consumer{}
	err := consumer.Consume(context.Background(), "", func(context.Context, amqp.Delivery) error { return nil })
	require.Error(t, err)

	err = consumer.Consume(context.Background(), "queue", nil)
	require.Error(t, err)
}

type ackTracker struct {
	acked   bool
	nacked  bool
	requeue bool
	ackErr  error
	nackErr error
}

func (a *ackTracker) Ack(tag uint64, multiple bool) error {
	a.acked = true
	return a.ackErr
}

func (a *ackTracker) Nack(tag uint64, multiple bool, requeue bool) error {
	a.nacked = true
	a.requeue = requeue
	return a.nackErr
}

func (a *ackTracker) Reject(tag uint64, requeue bool) error {
	a.nacked = true
	a.requeue = requeue
	return a.nackErr
}

func TestConsumeAckAndNackPaths(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deliveries := make(chan amqp.Delivery, 2)
	ch := &mockChannel{consumeCh: deliveries}
	consumer := &Consumer{
		channel: ch,
		queue:   "queue",
		tracer:  noop.NewTracerProvider().Tracer("test"),
	}

	ackErrTracker := &ackTracker{ackErr: errors.New("ack failed")}
	deliveries <- amqp.Delivery{Acknowledger: ackErrTracker, DeliveryTag: 1, RoutingKey: "rk"}

	errCh := make(chan error, 1)
	go func() {
		errCh <- consumer.Consume(ctx, "queue", func(context.Context, amqp.Delivery) error {
			return nil
		})
	}()

	err := <-errCh
	require.Error(t, err)
	require.True(t, ackErrTracker.acked)
}

func TestConsumeHandlerErrors(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	deliveries := make(chan amqp.Delivery, 2)
	ch := &mockChannel{consumeCh: deliveries}
	consumer := &Consumer{
		channel: ch,
		queue:   "queue",
		tracer:  noop.NewTracerProvider().Tracer("test"),
	}

	unrecoverable := &ackTracker{}
	recoverable := &ackTracker{}

	deliveries <- amqp.Delivery{Acknowledger: unrecoverable, DeliveryTag: 1, RoutingKey: "rk"}
	deliveries <- amqp.Delivery{Acknowledger: recoverable, DeliveryTag: 2, RoutingKey: "rk"}
	close(deliveries)

	handlerCalls := 0
	err := consumer.Consume(ctx, "queue", func(context.Context, amqp.Delivery) error {
		handlerCalls++
		if handlerCalls == 1 {
			return UnrecoverableError{Err: errors.New("bad")}
		}
		cancel()
		return errors.New("retryable")
	})
	require.NoError(t, err)
	require.True(t, unrecoverable.nacked)
	require.False(t, unrecoverable.requeue)
	require.True(t, recoverable.nacked)
	require.True(t, recoverable.requeue)
}

func TestUnrecoverableError(t *testing.T) {
	baseErr := errors.New("boom")
	err := UnrecoverableError{Err: baseErr}
	require.Equal(t, "boom", err.Error())
	require.ErrorIs(t, err, baseErr)
}

func TestInjectTraceContext(t *testing.T) {
	provider := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		_ = provider.Shutdown(context.Background())
	})

	tracer := otel.Tracer("test")
	ctx, span := tracer.Start(context.Background(), "span")
	defer span.End()

	headers := amqp.Table{}
	otel.GetTextMapPropagator().Inject(ctx, amqpHeaderCarrier(headers))

	consumer := &Consumer{tracer: tracer}
	msgCtx := consumer.injectTraceContext(context.Background(), amqp.Delivery{Headers: headers})
	spanCtx := trace.SpanContextFromContext(msgCtx)
	require.True(t, spanCtx.IsValid())
	require.Equal(t, span.SpanContext().TraceID(), spanCtx.TraceID())
}
