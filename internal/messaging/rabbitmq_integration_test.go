//go:build integration

package messaging

import (
	"context"
	"fmt"
	"testing"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestRabbitMQProducerConsumerIntegration(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "rabbitmq:3.13-management",
			ExposedPorts: []string{"5672/tcp"},
			WaitingFor: wait.ForAll(
				wait.ForListeningPort("5672/tcp"),
				wait.ForLog("Server startup complete"),
			).WithStartupTimeout(90 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start rabbitmq container: %v", err)
	}
	t.Cleanup(func() {
		_ = container.Terminate(context.Background())
	})

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("container host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5672/tcp")
	if err != nil {
		t.Fatalf("container port: %v", err)
	}
	rabbitURL := fmt.Sprintf("amqp://guest:guest@%s:%s/", host, port.Port())

	producer, err := NewProducer(ctx, rabbitURL)
	if err != nil {
		t.Fatalf("new producer: %v", err)
	}
	t.Cleanup(func() {
		_ = producer.Close()
	})
	if err := producer.DeclareTopology(ctx); err != nil {
		t.Fatalf("declare topology: %v", err)
	}

	consumer, err := NewConsumer(ctx, rabbitURL, 1)
	if err != nil {
		t.Fatalf("new consumer: %v", err)
	}

	received := make(chan []byte, 1)
	consumeCtx, stopConsume := context.WithCancel(ctx)
	defer stopConsume()
	go func() {
		_ = consumer.Consume(consumeCtx, queueHigh, func(ctx context.Context, delivery amqp.Delivery) error {
			received <- delivery.Body
			stopConsume()
			return nil
		})
	}()

	if err := producer.Publish(ctx, exchangeName, routingHigh, []byte(`{"message":"hello"}`)); err != nil {
		t.Fatalf("publish: %v", err)
	}

	select {
	case body := <-received:
		if string(body) != `{"message":"hello"}` {
			t.Fatalf("unexpected body: %s", string(body))
		}
	case <-time.After(30 * time.Second):
		t.Fatalf("timed out waiting for rabbitmq delivery")
	}
}
