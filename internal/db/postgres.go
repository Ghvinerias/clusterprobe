package db

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	postgresDriverName = "postgresql"
	postgresScope      = "clusterprobe/db/postgres"
)

//go:embed schema/postgres.sql
var postgresSchemaFS embed.FS

// PostgresClient wraps a pgx pool.
type PostgresClient struct {
	pool   postgresPool
	tracer trace.Tracer
}

type postgresPool interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Ping(ctx context.Context) error
	Close()
}

// NewPostgres creates a new Postgres client with retry/backoff.
func NewPostgres(ctx context.Context, dsn string, maxConns int32) (*PostgresClient, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, fmt.Errorf("postgres dsn is required")
	}
	if maxConns <= 0 {
		return nil, fmt.Errorf("postgres maxConns must be greater than zero")
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse postgres dsn: %w", err)
	}
	cfg.MaxConns = maxConns

	pool, err := connectPostgresWithRetry(ctx, cfg)
	if err != nil {
		return nil, err
	}

	return &PostgresClient{
		pool:   pool,
		tracer: otel.Tracer(postgresScope),
	}, nil
}

func connectPostgresWithRetry(ctx context.Context, cfg *pgxpool.Config) (*pgxpool.Pool, error) {
	backoff := 200 * time.Millisecond
	var lastErr error
	for attempt := 1; attempt <= 10; attempt++ {
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			lastErr = err
		} else if err = pool.Ping(ctx); err != nil {
			lastErr = err
			pool.Close()
		} else {
			return pool, nil
		}

		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return nil, fmt.Errorf("postgres connect canceled: %w", ctx.Err())
		}

		if attempt < 10 {
			timer := time.NewTimer(backoff)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, fmt.Errorf("postgres connect canceled: %w", ctx.Err())
			case <-timer.C:
			}
			if backoff < 5*time.Second {
				backoff *= 2
			}
		}
	}

	return nil, fmt.Errorf("postgres connect failed after retries: %w", lastErr)
}

// Ping verifies connectivity.
func (c *PostgresClient) Ping(ctx context.Context) error {
	if err := c.pool.Ping(ctx); err != nil {
		return fmt.Errorf("postgres ping: %w", err)
	}
	return nil
}

// Exec executes a statement with an OTEL span.
func (c *PostgresClient) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	ctx, span := c.tracer.Start(ctx, "postgres.exec")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", postgresDriverName),
		attribute.String("db.statement", sql),
	)

	result, err := c.pool.Exec(ctx, sql, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "exec failed")
		return pgconn.CommandTag{}, fmt.Errorf("postgres exec: %w", err)
	}

	return result, nil
}

// Query executes a query with an OTEL span.
func (c *PostgresClient) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	ctx, span := c.tracer.Start(ctx, "postgres.query")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", postgresDriverName),
		attribute.String("db.statement", sql),
	)

	rows, err := c.pool.Query(ctx, sql, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "query failed")
		return nil, fmt.Errorf("postgres query: %w", err)
	}

	return rows, nil
}

// QueryRow executes a query row with an OTEL span.
func (c *PostgresClient) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	ctx, span := c.tracer.Start(ctx, "postgres.query_row")
	defer span.End()

	span.SetAttributes(
		attribute.String("db.system", postgresDriverName),
		attribute.String("db.statement", sql),
	)

	return c.pool.QueryRow(ctx, sql, args...)
}

// InitSchema applies the embedded schema.
func (c *PostgresClient) InitSchema(ctx context.Context) error {
	data, err := postgresSchemaFS.ReadFile("schema/postgres.sql")
	if err != nil {
		return fmt.Errorf("read postgres schema: %w", err)
	}

	statements := splitSQLStatements(string(data))
	for _, stmt := range statements {
		if _, err := c.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("apply postgres schema: %w", err)
		}
	}

	return nil
}

// Close releases pool resources.
func (c *PostgresClient) Close() {
	c.pool.Close()
}

func splitSQLStatements(sql string) []string {
	parts := strings.Split(sql, ";")
	statements := make([]string, 0, len(parts))
	for _, part := range parts {
		stmt := strings.TrimSpace(part)
		if stmt == "" {
			continue
		}
		statements = append(statements, stmt)
	}
	return statements
}
