package workload

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type mockStore struct {
	execCalls int64
	row       Row
	execErr   error
}

func (m *mockStore) Exec(ctx context.Context, sql string, args ...any) error {
	atomic.AddInt64(&m.execCalls, 1)
	return m.execErr
}

func (m *mockStore) Query(ctx context.Context, sql string, args ...any) (Rows, error) {
	return &mockRows{}, nil
}

func (m *mockStore) QueryRow(ctx context.Context, sql string, args ...any) Row {
	return m.row
}

type mockRow struct {
	err error
}

func (r *mockRow) Scan(dest ...any) error {
	return r.err
}

type mockRows struct{}

func (r *mockRows) Close()                 {}
func (r *mockRows) Err() error             { return nil }
func (r *mockRows) Next() bool             { return false }
func (r *mockRows) Scan(dest ...any) error { return nil }

func TestCPUGenerator(t *testing.T) {
	gen := &CPUGenerator{}
	result, err := gen.Execute(context.Background(), WorkloadParams{DurationMs: 5, WorkloadType: WorkloadTypeCPUBurn})
	if err != nil {
		t.Fatalf("cpu execute: %v", err)
	}
	if result.Ops <= 0 {
		t.Fatalf("expected ops")
	}
}

func TestMemoryGenerator(t *testing.T) {
	gen := &MemoryGenerator{}
	result, err := gen.Execute(
		context.Background(),
		WorkloadParams{DurationMs: 5, AllocMB: 1, WorkloadType: WorkloadTypeMemAlloc},
	)
	if err != nil {
		t.Fatalf("memory execute: %v", err)
	}
	if result.Duration <= 0 {
		t.Fatalf("expected duration")
	}
}

func TestDBWriteGenerator(t *testing.T) {
	store := &mockStore{}
	gen := &DBWriteGenerator{}
	params := WorkloadParams{
		ScenarioID:   "s1",
		WorkloadType: WorkloadTypeDBWrite,
		DurationMs:   5,
		BatchSize:    2,
		Store:        store,
	}
	result, err := gen.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("db write execute: %v", err)
	}
	if result.Ops <= 0 {
		t.Fatalf("expected ops")
	}
	if atomic.LoadInt64(&store.execCalls) == 0 {
		t.Fatalf("expected exec calls")
	}
}

func TestDBWriteGeneratorMissingStore(t *testing.T) {
	gen := &DBWriteGenerator{}
	_, err := gen.Execute(context.Background(), WorkloadParams{DurationMs: 1, WorkloadType: WorkloadTypeDBWrite})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestDBReadGenerator(t *testing.T) {
	store := &mockStore{row: &mockRow{}}
	gen := &DBReadGenerator{}
	params := WorkloadParams{
		ScenarioID:     "s1",
		WorkloadType:   WorkloadTypeDBRead,
		DurationMs:     5,
		ReadLookbackMs: int64(1000),
		Store:          store,
	}
	result, err := gen.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("db read execute: %v", err)
	}
	if result.Ops <= 0 {
		t.Fatalf("expected ops")
	}
}

func TestDBReadGeneratorMissingStore(t *testing.T) {
	gen := &DBReadGenerator{}
	_, err := gen.Execute(context.Background(), WorkloadParams{DurationMs: 1, WorkloadType: WorkloadTypeDBRead})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestDBReadGeneratorScanError(t *testing.T) {
	store := &mockStore{row: &mockRow{err: errors.New("scan")}}
	gen := &DBReadGenerator{}
	params := WorkloadParams{
		ScenarioID:     "s1",
		WorkloadType:   WorkloadTypeDBRead,
		DurationMs:     1,
		ReadLookbackMs: int64(1000),
		Store:          store,
	}
	_, err := gen.Execute(context.Background(), params)
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestMixedGenerator(t *testing.T) {
	gen := &MixedGenerator{
		CPU:     &CPUGenerator{},
		DBWrite: &DBWriteGenerator{},
		DBRead:  &DBReadGenerator{},
	}
	store := &mockStore{row: &mockRow{}}
	params := WorkloadParams{
		ScenarioID:     "s1",
		WorkloadType:   WorkloadTypeMixed,
		DurationMs:     5,
		ReadLookbackMs: int64(1000),
		BatchSize:      1,
		Store:          store,
		MixedProfile:   MixedProfile{CPUPercent: 50, DBWritePercent: 25, DBReadPercent: 25},
	}
	result, err := gen.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("mixed execute: %v", err)
	}
	if result.Duration <= 0 {
		t.Fatalf("expected duration")
	}
}

func TestValidateDuration(t *testing.T) {
	if err := validateDuration(WorkloadParams{DurationMs: 0}); err == nil {
		t.Fatalf("expected error")
	}
	if err := validateDuration(WorkloadParams{DurationMs: 1}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestMemoryGeneratorContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	gen := &MemoryGenerator{}
	result, err := gen.Execute(ctx, WorkloadParams{DurationMs: 10, AllocMB: 1, WorkloadType: WorkloadTypeMemAlloc})
	if err == nil {
		t.Fatalf("expected error")
	}
	if result.Duration <= 0 {
		t.Fatalf("expected duration")
	}
}

func TestDBWriteGeneratorDuration(t *testing.T) {
	store := &mockStore{}
	gen := &DBWriteGenerator{}
	params := WorkloadParams{
		ScenarioID:   "s1",
		WorkloadType: WorkloadTypeDBWrite,
		DurationMs:   int64((2 * time.Millisecond).Milliseconds()),
		BatchSize:    1,
		Store:        store,
	}
	result, err := gen.Execute(context.Background(), params)
	if err != nil {
		t.Fatalf("db write execute: %v", err)
	}
	if result.Duration <= 0 {
		t.Fatalf("expected duration")
	}
}
