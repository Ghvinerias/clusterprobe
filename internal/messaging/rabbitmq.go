package messaging

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const (
	exchangeName   = "clusterprobe.events"
	dlxName        = "clusterprobe.events.dlx"
	queueHigh      = "workload.high"
	queueLow       = "workload.low"
	queueChaos     = "workload.chaos"
	routingHigh    = "workload.high"
	routingLow     = "workload.low"
	routingChaos   = "workload.chaos"
	messagingScope = "clusterprobe/messaging/rabbitmq"
)

// Producer publishes events to RabbitMQ with auto-reconnect.
type Producer struct {
	mu          sync.RWMutex
	conn        amqpConn
	channel     amqpChannel
	dial        func(ctx context.Context) (amqpConn, error)
	reconnectCh chan struct{}
	tracer      trace.Tracer
}

type amqpConn interface {
	Channel() (amqpChannel, error)
	NotifyClose(c chan *amqp.Error) chan *amqp.Error
	Close() error
}

type amqpChannel interface {
	Publish(exchange, key string, mandatory, immediate bool, msg amqp.Publishing) error
	ExchangeDeclare(name, kind string, durable, autoDelete, internal, noWait bool, args amqp.Table) error
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	QueueBind(name, key, exchange string, noWait bool, args amqp.Table) error
	Qos(prefetchCount, prefetchSize int, global bool) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	Close() error
}

type realConn struct {
	conn *amqp.Connection
}

func (c realConn) Channel() (amqpChannel, error) {
	ch, err := c.conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("rabbitmq channel: %w", err)
	}
	return ch, nil
}

func (c realConn) NotifyClose(ch chan *amqp.Error) chan *amqp.Error {
	return c.conn.NotifyClose(ch)
}

func (c realConn) Close() error {
	if err := c.conn.Close(); err != nil {
		return fmt.Errorf("rabbitmq close: %w", err)
	}
	return nil
}

// NewProducer creates a producer and starts reconnect monitoring.
func NewProducer(ctx context.Context, url string) (*Producer, error) {
	if url == "" {
		return nil, fmt.Errorf("rabbitmq url is required")
	}

	dialer := func(ctx context.Context) (amqpConn, error) {
		conn, err := amqp.Dial(url)
		if err != nil {
			return nil, fmt.Errorf("rabbitmq dial: %w", err)
		}
		return realConn{conn: conn}, nil
	}

	producer := &Producer{
		dial:        dialer,
		reconnectCh: make(chan struct{}, 1),
		tracer:      otel.Tracer(messagingScope),
	}

	if err := producer.connect(ctx); err != nil {
		return nil, err
	}

	go producer.monitorReconnect(ctx)

	return producer, nil
}

func (p *Producer) connect(ctx context.Context) error {
	conn, err := p.dial(ctx)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("rabbitmq channel: %w", err)
	}

	p.mu.Lock()
	if p.channel != nil {
		_ = p.channel.Close()
	}
	if p.conn != nil {
		_ = p.conn.Close()
	}
	p.conn = conn
	p.channel = ch
	p.mu.Unlock()

	select {
	case p.reconnectCh <- struct{}{}:
	default:
	}

	return nil
}

func (p *Producer) monitorReconnect(ctx context.Context) {
	for {
		p.mu.RLock()
		conn := p.conn
		p.mu.RUnlock()

		if conn == nil {
			return
		}

		notify := make(chan *amqp.Error, 1)
		conn.NotifyClose(notify)

		select {
		case <-ctx.Done():
			return
		case <-notify:
			p.reconnect(ctx)
		}
	}
}

func (p *Producer) reconnect(ctx context.Context) {
	backoff := 500 * time.Millisecond
	for attempt := 1; attempt <= 10; attempt++ {
		if err := p.connect(ctx); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
}

// Publish sends a message and injects W3C trace context into headers.
func (p *Producer) Publish(ctx context.Context, exchange, routingKey string, body []byte) error {
	ctx, span := p.tracer.Start(ctx, "rabbitmq.publish")
	defer span.End()

	span.SetAttributes(
		attribute.String("messaging.system", "rabbitmq"),
		attribute.String("messaging.destination", exchange),
		attribute.String("messaging.rabbitmq.routing_key", routingKey),
	)

	p.mu.RLock()
	ch := p.channel
	p.mu.RUnlock()

	if ch == nil {
		err := fmt.Errorf("rabbitmq channel not available")
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish failed")
		return err
	}

	headers := amqp.Table{}
	carrier := amqpHeaderCarrier(headers)
	otel.GetTextMapPropagator().Inject(ctx, carrier)

	msg := amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		Headers:      headers,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
	}

	if err := ch.Publish(exchange, routingKey, false, false, msg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish failed")
		return fmt.Errorf("rabbitmq publish: %w", err)
	}

	return nil
}

// DeclareTopology sets up exchanges, queues, and bindings for ClusterProbe.
func (p *Producer) DeclareTopology(ctx context.Context) error {
	p.mu.RLock()
	ch := p.channel
	p.mu.RUnlock()

	if ch == nil {
		return fmt.Errorf("rabbitmq channel not available")
	}

	if err := ch.ExchangeDeclare(exchangeName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare exchange: %w", err)
	}
	if err := ch.ExchangeDeclare(dlxName, "topic", true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare dlx: %w", err)
	}

	queues := []struct {
		name       string
		routingKey string
	}{
		{name: queueHigh, routingKey: routingHigh},
		{name: queueLow, routingKey: routingLow},
		{name: queueChaos, routingKey: routingChaos},
	}

	for _, q := range queues {
		dlqName := q.name + ".dlq"
		if _, err := ch.QueueDeclare(dlqName, true, false, false, false, nil); err != nil {
			return fmt.Errorf("declare dlq %s: %w", dlqName, err)
		}
		if err := ch.QueueBind(dlqName, q.routingKey, dlxName, false, nil); err != nil {
			return fmt.Errorf("bind dlq %s: %w", dlqName, err)
		}

		args := amqp.Table{
			"x-dead-letter-exchange":    dlxName,
			"x-dead-letter-routing-key": q.routingKey,
		}

		if _, err := ch.QueueDeclare(q.name, true, false, false, false, args); err != nil {
			return fmt.Errorf("declare queue %s: %w", q.name, err)
		}
		if err := ch.QueueBind(q.name, q.routingKey, exchangeName, false, nil); err != nil {
			return fmt.Errorf("bind queue %s: %w", q.name, err)
		}
	}

	return nil
}

// Close shuts down the producer.
func (p *Producer) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var errs []error
	if p.channel != nil {
		if err := p.channel.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
			errs = append(errs, err)
		}
		p.channel = nil
	}
	if p.conn != nil {
		if err := p.conn.Close(); err != nil && !errors.Is(err, amqp.ErrClosed) {
			errs = append(errs, err)
		}
		p.conn = nil
	}
	if len(errs) > 0 {
		return fmt.Errorf("rabbitmq close: %w", errs[0])
	}
	return nil
}

type amqpHeaderCarrier amqp.Table

func (c amqpHeaderCarrier) Get(key string) string {
	value, ok := amqp.Table(c)[key]
	if !ok {
		return ""
	}
	str, ok := value.(string)
	if !ok {
		return ""
	}
	return str
}

func (c amqpHeaderCarrier) Set(key string, value string) {
	amqp.Table(c)[key] = value
}

func (c amqpHeaderCarrier) Keys() []string {
	keys := make([]string, 0, len(c))
	for key := range c {
		keys = append(keys, key)
	}
	return keys
}

func init() {
	otel.SetTextMapPropagator(propagation.TraceContext{})
}

// UnrecoverableError signals message processing failures that should be dead-lettered.
type UnrecoverableError struct {
	Err error
}

// Error returns the wrapped error message.
func (e UnrecoverableError) Error() string {
	return e.Err.Error()
}

// Unwrap returns the wrapped error.
func (e UnrecoverableError) Unwrap() error {
	return e.Err
}

// Consumer handles message consumption with auto-reconnect.
type Consumer struct {
	mu       sync.RWMutex
	conn     amqpConn
	channel  amqpChannel
	queue    string
	prefetch int
	dial     func(ctx context.Context) (amqpConn, error)
	tracer   trace.Tracer
}

// NewConsumer creates a new RabbitMQ consumer.
func NewConsumer(ctx context.Context, url string, prefetch int) (*Consumer, error) {
	if url == "" {
		return nil, fmt.Errorf("rabbitmq url is required")
	}
	if prefetch <= 0 {
		prefetch = 1
	}

	dialer := func(ctx context.Context) (amqpConn, error) {
		conn, err := amqp.Dial(url)
		if err != nil {
			return nil, fmt.Errorf("rabbitmq dial: %w", err)
		}
		return realConn{conn: conn}, nil
	}

	consumer := &Consumer{
		dial:     dialer,
		prefetch: prefetch,
		tracer:   otel.Tracer(messagingScope),
	}

	if err := consumer.connect(ctx); err != nil {
		return nil, err
	}

	go consumer.monitorReconnect(ctx)

	return consumer, nil
}

// Consume starts consuming messages from a queue and invokes handler for each delivery.
func (c *Consumer) Consume(ctx context.Context, queue string, handler func(context.Context, amqp.Delivery) error) error {
	if queue == "" {
		return fmt.Errorf("queue is required")
	}
	if handler == nil {
		return fmt.Errorf("handler is required")
	}

	c.mu.Lock()
	c.queue = queue
	c.mu.Unlock()

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		deliveries, err := c.consumeOnce(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(500 * time.Millisecond):
				continue
			}
		}

		for delivery := range deliveries {
			msgCtx := c.injectTraceContext(ctx, delivery)
			msgCtx, span := c.tracer.Start(msgCtx, "rabbitmq.consume")
			span.SetAttributes(
				attribute.String("messaging.system", "rabbitmq"),
				attribute.String("messaging.destination", c.queue),
				attribute.String("messaging.rabbitmq.routing_key", delivery.RoutingKey),
			)

			if err := handler(msgCtx, delivery); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "handler error")
				if errors.As(err, &UnrecoverableError{}) {
					if nackErr := delivery.Nack(false, false); nackErr != nil {
						return fmt.Errorf("nack message: %w", nackErr)
					}
				} else {
					if nackErr := delivery.Nack(false, true); nackErr != nil {
						return fmt.Errorf("nack message: %w", nackErr)
					}
				}
				span.End()
				continue
			}

			if err := delivery.Ack(false); err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, "ack error")
				span.End()
				return fmt.Errorf("ack message: %w", err)
			}
			span.End()
		}
	}
}

func (c *Consumer) consumeOnce(ctx context.Context) (<-chan amqp.Delivery, error) {
	c.mu.RLock()
	ch := c.channel
	queue := c.queue
	prefetch := c.prefetch
	c.mu.RUnlock()

	if ch == nil {
		return nil, fmt.Errorf("rabbitmq channel not available")
	}
	if err := ch.Qos(prefetch, 0, false); err != nil {
		return nil, fmt.Errorf("rabbitmq qos: %w", err)
	}

	deliveries, err := ch.Consume(queue, "", false, false, false, false, nil)
	if err != nil {
		return nil, fmt.Errorf("rabbitmq consume: %w", err)
	}
	return deliveries, nil
}

func (c *Consumer) connect(ctx context.Context) error {
	conn, err := c.dial(ctx)
	if err != nil {
		return err
	}

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("rabbitmq channel: %w", err)
	}

	c.mu.Lock()
	if c.channel != nil {
		_ = c.channel.Close()
	}
	if c.conn != nil {
		_ = c.conn.Close()
	}
	c.conn = conn
	c.channel = ch
	c.mu.Unlock()

	return nil
}

func (c *Consumer) monitorReconnect(ctx context.Context) {
	for {
		c.mu.RLock()
		conn := c.conn
		c.mu.RUnlock()

		if conn == nil {
			return
		}

		notify := make(chan *amqp.Error, 1)
		conn.NotifyClose(notify)

		select {
		case <-ctx.Done():
			return
		case <-notify:
			c.reconnect(ctx)
		}
	}
}

func (c *Consumer) reconnect(ctx context.Context) {
	backoff := 500 * time.Millisecond
	for attempt := 1; attempt <= 10; attempt++ {
		if err := c.connect(ctx); err == nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 5*time.Second {
			backoff *= 2
		}
	}
}

func (c *Consumer) injectTraceContext(ctx context.Context, msg amqp.Delivery) context.Context {
	carrier := amqpHeaderCarrier(msg.Headers)
	return otel.GetTextMapPropagator().Extract(ctx, carrier)
}
