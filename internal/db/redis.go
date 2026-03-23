package db

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	redisDriverName = "redis"
	redisScope      = "clusterprobe/db/redis"
)

// RedisClient wraps a Redis client.
type RedisClient struct {
	client *redis.Client
	tracer trace.Tracer
}

// NewRedis creates a Redis client with retry/backoff.
func NewRedis(ctx context.Context, addr, password string, db int) (*RedisClient, error) {
	if strings.TrimSpace(addr) == "" {
		return nil, fmt.Errorf("redis addr is required")
	}

	options := &redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	}

	client, err := connectRedisWithRetry(ctx, options)
	if err != nil {
		return nil, err
	}

	return &RedisClient{
		client: client,
		tracer: otel.Tracer(redisScope),
	}, nil
}

func connectRedisWithRetry(ctx context.Context, options *redis.Options) (*redis.Client, error) {
	backoff := 200 * time.Millisecond
	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		client := redis.NewClient(options)
		if err := client.Ping(ctx).Err(); err != nil {
			lastErr = err
			_ = client.Close()
		} else {
			return client, nil
		}

		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("redis connect canceled: %w", ctx.Err())
		}

		if attempt < 10 {
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, fmt.Errorf("redis connect canceled: %w", ctx.Err())
			case <-timer.C:
			}
			if backoff < 5*time.Second {
				backoff *= 2
			}
		}
	}

	return nil, fmt.Errorf("redis connect failed after retries: %w", lastErr)
}

// Set sets a key with expiration.
func (c *RedisClient) Set(ctx context.Context, key string, value any, ttl time.Duration) error {
	ctx, span := c.tracer.Start(ctx, "redis.set")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", redisDriverName),
		attribute.String("db.operation", "SET"),
	)

	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "set failed")
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

// Get returns a value.
func (c *RedisClient) Get(ctx context.Context, key string) (string, error) {
	ctx, span := c.tracer.Start(ctx, "redis.get")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", redisDriverName),
		attribute.String("db.operation", "GET"),
	)

	value, err := c.client.Get(ctx, key).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "get failed")
		return "", fmt.Errorf("redis get: %w", err)
	}

	return value, nil
}

// Del deletes keys.
func (c *RedisClient) Del(ctx context.Context, keys ...string) (int64, error) {
	ctx, span := c.tracer.Start(ctx, "redis.del")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", redisDriverName),
		attribute.String("db.operation", "DEL"),
	)

	count, err := c.client.Del(ctx, keys...).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "del failed")
		return 0, fmt.Errorf("redis del: %w", err)
	}

	return count, nil
}

// Incr increments a key.
func (c *RedisClient) Incr(ctx context.Context, key string) (int64, error) {
	ctx, span := c.tracer.Start(ctx, "redis.incr")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", redisDriverName),
		attribute.String("db.operation", "INCR"),
	)

	value, err := c.client.Incr(ctx, key).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "incr failed")
		return 0, fmt.Errorf("redis incr: %w", err)
	}

	return value, nil
}

// Expire sets expiration.
func (c *RedisClient) Expire(ctx context.Context, key string, ttl time.Duration) (bool, error) {
	ctx, span := c.tracer.Start(ctx, "redis.expire")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", redisDriverName),
		attribute.String("db.operation", "EXPIRE"),
	)

	ok, err := c.client.Expire(ctx, key, ttl).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "expire failed")
		return false, fmt.Errorf("redis expire: %w", err)
	}

	return ok, nil
}

// Publish publishes a message.
func (c *RedisClient) Publish(ctx context.Context, channel string, payload any) (int64, error) {
	ctx, span := c.tracer.Start(ctx, "redis.publish")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", redisDriverName),
		attribute.String("db.operation", "PUBLISH"),
	)

	result, err := c.client.Publish(ctx, channel, payload).Result()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish failed")
		return 0, fmt.Errorf("redis publish: %w", err)
	}

	return result, nil
}

// Subscribe subscribes to a channel.
func (c *RedisClient) Subscribe(ctx context.Context, channel string) (*redis.PubSub, error) {
	ctx, span := c.tracer.Start(ctx, "redis.subscribe")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", redisDriverName),
		attribute.String("db.operation", "SUBSCRIBE"),
	)

	pubsub := c.client.Subscribe(ctx, channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "subscribe failed")
		_ = pubsub.Close()
		return nil, fmt.Errorf("redis subscribe: %w", err)
	}

	return pubsub, nil
}

// Close closes the client.
func (c *RedisClient) Close() error {
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("redis close: %w", err)
	}
	return nil
}
