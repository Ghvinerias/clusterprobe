package db

import (
	"context"
	"testing"

	"github.com/pashagolub/pgxmock/v2"
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
	if _, err := client.Exec(context.Background(), "INSERT INTO load_events (scenario_id, payload) VALUES ($1, $2)", "s", "{}"); err != nil {
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

func TestPostgresInitSchema(t *testing.T) {
	mock, err := pgxmock.NewPool()
	if err != nil {
		t.Fatalf("pgxmock: %v", err)
	}
	defer mock.Close()

	client := &PostgresClient{pool: mock, tracer: otel.Tracer("test")}

	mock.ExpectExec("CREATE TABLE IF NOT EXISTS load_events").WillReturnResult(pgxmock.NewResult("CREATE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS chaos_events").WillReturnResult(pgxmock.NewResult("CREATE", 0))
	mock.ExpectExec("CREATE TABLE IF NOT EXISTS metrics_snapshots").WillReturnResult(pgxmock.NewResult("CREATE", 0))

	if err := client.InitSchema(context.Background()); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("expectations: %v", err)
	}
}
