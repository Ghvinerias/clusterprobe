package db

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/pashagolub/pgxmock/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
)

func TestPostgresExecQuery(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()

	client := &PostgresClient{pool: mock, tracer: otel.Tracer("test")}

	mock.ExpectExec("INSERT INTO load_events").WithArgs("s", "{}").WillReturnResult(pgxmock.NewResult("INSERT", 1))
	if _, err := client.Exec(
		context.Background(),
		"INSERT INTO load_events (scenario_id, payload) VALUES ($1, $2)",
		"s",
		"{}",
	); err != nil {
		t.Fatalf("exec: %v", err)
	}

	rows := pgxmock.NewRows([]string{"id"}).AddRow(int64(1))
	mock.ExpectQuery("SELECT id FROM load_events").WillReturnRows(rows)
	r, err := client.Query(context.Background(), "SELECT id FROM load_events")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	r.Close()

	row := pgxmock.NewRows([]string{"id"}).AddRow(int64(2))
	mock.ExpectQuery("SELECT id FROM chaos_events").WillReturnRows(row)
	var id int64
	if err := client.QueryRow(context.Background(), "SELECT id FROM chaos_events").Scan(&id); err != nil {
		t.Fatalf("query row: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}

func TestPostgresExecQueryErrors(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	client := &PostgresClient{pool: mock, tracer: otel.Tracer("test")}

	mock.ExpectExec("INSERT INTO load_events").WithArgs("s", "{}").WillReturnError(errors.New("exec failed"))
	_, err = client.Exec(context.Background(), "INSERT INTO load_events (scenario_id, payload) VALUES ($1, $2)", "s", "{}")
	require.Error(t, err)

	mock.ExpectQuery("SELECT id FROM load_events").WillReturnError(errors.New("query failed"))
	_, err = client.Query(context.Background(), "SELECT id FROM load_events")
	require.Error(t, err)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestPostgresInitSchema(t *testing.T) {
	mock, err := pgxmock.NewPool()
	require.NoError(t, err)
	defer mock.Close()

	client := &PostgresClient{pool: mock, tracer: otel.Tracer("test")}

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS load_events").WillReturnResult(pgxmock.NewResult("CREATE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS chaos_events").WillReturnResult(pgxmock.NewResult("CREATE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS metrics_snapshots").WillReturnResult(pgxmock.NewResult("CREATE", 0))

	require.NoError(t, client.InitSchema(context.Background()))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewPostgresValidation(t *testing.T) {
	_, err := NewPostgres(context.Background(), "", 1)
	require.Error(t, err)

	_, err = NewPostgres(context.Background(), "postgres://user:pass@localhost:5432/db", 0)
	require.Error(t, err)
}

func TestConnectPostgresWithRetryCanceled(t *testing.T) {
	cfg, err := pgxpool.ParseConfig("postgres://user:pass@localhost:5432/db")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	pool, err := connectPostgresWithRetry(ctx, cfg)
	require.Error(t, err)
	require.Nil(t, pool)
}

func TestPostgresPingError(t *testing.T) {
	pool := &stubPostgresPool{pingErr: errors.New("ping failed")}
	client := &PostgresClient{pool: pool, tracer: otel.Tracer("test")}

	require.Error(t, client.Ping(context.Background()))
}

func TestPostgresClose(t *testing.T) {
	pool := &stubPostgresPool{}
	client := &PostgresClient{pool: pool, tracer: otel.Tracer("test")}

	client.Close()
	require.True(t, pool.closed)
}

type stubPostgresPool struct {
	closed  bool
	pingErr error
}

func (s *stubPostgresPool) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, nil
}

func (s *stubPostgresPool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return nil, nil
}

func (s *stubPostgresPool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return nil
}

func (s *stubPostgresPool) Ping(ctx context.Context) error {
	return s.pingErr
}

func (s *stubPostgresPool) Close() {
	s.closed = true
}
