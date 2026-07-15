//go:build integration

package handlers

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/db"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestScenarioQueriesUseLifecycleEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "postgres:16",
			ExposedPorts: []string{"5432/tcp"},
			Env: map[string]string{
				"POSTGRES_USER":     "clusterprobe",
				"POSTGRES_PASSWORD": "clusterprobe",
				"POSTGRES_DB":       "clusterprobe",
			},
			WaitingFor: wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(60 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("start postgres container: %v", err)
	}
	defer func() {
		_ = container.Terminate(context.Background())
	}()

	host, err := container.Host(ctx)
	if err != nil {
		t.Fatalf("postgres host: %v", err)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		t.Fatalf("postgres port: %v", err)
	}

	dsn := fmt.Sprintf(
		"postgres://clusterprobe:clusterprobe@%s:%s/clusterprobe?sslmode=disable",
		host,
		port.Port(),
	)
	postgres, err := db.NewPostgres(ctx, dsn, 4)
	if err != nil {
		t.Fatalf("new postgres: %v", err)
	}
	defer postgres.Close()

	if err := postgres.InitSchema(ctx); err != nil {
		t.Fatalf("init schema: %v", err)
	}

	scenarioID := "scenario-split"
	workloadPayload := []byte(`{"scenario_id":"scenario-split","workload":"db_write"}`)
	queuedPayload := []byte(`{"id":"scenario-split","name":"split","status":"queued","created_at":"2026-07-15T00:00:00Z","profile":{"rps":1,"duration":1000000000,"payload_size_bytes":0,"concurrency":1,"target_queue":"workload.high","workload_type":"db_write"}}`)
	completedPayload := []byte(`{"id":"scenario-split","name":"split","status":"completed","created_at":"2026-07-15T00:00:00Z","profile":{"rps":1,"duration":1000000000,"payload_size_bytes":0,"concurrency":1,"target_queue":"workload.high","workload_type":"db_write"}}`)

	if _, err := postgres.Exec(ctx, insertScenarioQuery, scenarioID, queuedPayload); err != nil {
		t.Fatalf("insert queued lifecycle event: %v", err)
	}
	if _, err := postgres.Exec(ctx, "INSERT INTO load_events (scenario_id, payload) VALUES ($1, $2)", scenarioID, workloadPayload); err != nil {
		t.Fatalf("insert workload event: %v", err)
	}
	if _, err := postgres.Exec(ctx, insertScenarioQuery, scenarioID, completedPayload); err != nil {
		t.Fatalf("insert completed lifecycle event: %v", err)
	}

	var listedID string
	var listedPayload []byte
	var listedCreated time.Time
	rows, err := postgres.Query(ctx, listScenarioQuery)
	if err != nil {
		t.Fatalf("list scenarios: %v", err)
	}
	defer rows.Close()
	if !rows.Next() {
		t.Fatalf("expected listed scenario")
	}
	if err := rows.Scan(&listedID, &listedPayload, &listedCreated); err != nil {
		t.Fatalf("scan listed scenario: %v", err)
	}
	if listedID != scenarioID {
		t.Fatalf("expected listed scenario %s, got %s", scenarioID, listedID)
	}
	if got := string(listedPayload); !containsJSONStatus(got, "completed") {
		t.Fatalf("expected completed lifecycle payload, got %s", got)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("list rows: %v", err)
	}

	var gotID string
	var gotPayload []byte
	var gotCreated time.Time
	if err := postgres.QueryRow(ctx, getScenarioQuery, scenarioID).Scan(&gotID, &gotPayload, &gotCreated); err != nil {
		t.Fatalf("get scenario: %v", err)
	}
	if gotID != scenarioID {
		t.Fatalf("expected scenario %s, got %s", scenarioID, gotID)
	}
	if got := string(gotPayload); !containsJSONStatus(got, "completed") {
		t.Fatalf("expected completed lifecycle payload, got %s", got)
	}

	var running int64
	var completed int64
	if err := postgres.QueryRow(ctx, scenarioCountsQuery).Scan(&running, &completed); err != nil {
		t.Fatalf("scenario counts: %v", err)
	}
	if running != 0 || completed != 1 {
		t.Fatalf("expected running=0 completed=1, got running=%d completed=%d", running, completed)
	}
}

func containsJSONStatus(payload string, status string) bool {
	return strings.Contains(payload, fmt.Sprintf(`"status": "%s"`, status)) ||
		strings.Contains(payload, fmt.Sprintf(`"status":"%s"`, status))
}
