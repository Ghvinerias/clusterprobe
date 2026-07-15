package main

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/chaos"
)

type fakeChaosClient struct {
	applied chaos.ExperimentSpec
	deleted string
	status  chaos.ExperimentStatus
	list    []chaos.ExperimentStatus
	err     error
}

func (f *fakeChaosClient) Apply(ctx context.Context, spec chaos.ExperimentSpec) (string, error) {
	f.applied = spec
	if f.err != nil {
		return "", f.err
	}
	return spec.Name, nil
}

func (f *fakeChaosClient) Status(ctx context.Context, id string) (chaos.ExperimentStatus, error) {
	if f.err != nil {
		return chaos.ExperimentStatus{}, f.err
	}
	f.status.Name = id
	return f.status, nil
}

func (f *fakeChaosClient) Delete(ctx context.Context, id string) error {
	f.deleted = id
	return f.err
}

func (f *fakeChaosClient) List(ctx context.Context) ([]chaos.ExperimentStatus, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.list, nil
}

func TestRunApply(t *testing.T) {
	client := &fakeChaosClient{}
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	err := runWithClient(
		context.Background(),
		[]string{"--namespace", "cluster-probe", "apply", "-f", "experiment.yaml"},
		stdout,
		stderr,
		client,
		func(path string) ([]byte, error) {
			return []byte(`
apiVersion: chaos-mesh.org/v1alpha1
kind: PodChaos
metadata:
  name: kill-worker
spec:
  action: pod-kill
`), nil
		},
	)
	if err != nil {
		t.Fatalf("run apply: %v", err)
	}
	if client.applied.Name != "kill-worker" {
		t.Fatalf("expected manifest name, got %s", client.applied.Name)
	}
	if client.applied.Namespace != "cluster-probe" {
		t.Fatalf("expected default namespace, got %s", client.applied.Namespace)
	}
	if !strings.Contains(stdout.String(), "id=kill-worker") {
		t.Fatalf("expected applied output, got %s", stdout.String())
	}
}

func TestRunStatusJSON(t *testing.T) {
	now := time.Now().UTC()
	client := &fakeChaosClient{
		status: chaos.ExperimentStatus{
			Namespace: "cluster-probe",
			Kind:      "PodChaos",
			Phase:     "Running",
			StartTime: &now,
		},
	}
	stdout := &bytes.Buffer{}

	err := runWithClient(
		context.Background(),
		[]string{"-o", "json", "status", "kill-worker"},
		stdout,
		&bytes.Buffer{},
		client,
		nil,
	)
	if err != nil {
		t.Fatalf("run status: %v", err)
	}
	if !strings.Contains(stdout.String(), `"Name": "kill-worker"`) {
		t.Fatalf("expected json status, got %s", stdout.String())
	}
}

func TestRunDelete(t *testing.T) {
	client := &fakeChaosClient{}
	stdout := &bytes.Buffer{}

	err := runWithClient(
		context.Background(),
		[]string{"delete", "kill-worker"},
		stdout,
		&bytes.Buffer{},
		client,
		nil,
	)
	if err != nil {
		t.Fatalf("run delete: %v", err)
	}
	if client.deleted != "kill-worker" {
		t.Fatalf("expected deleted id")
	}
}

func TestRunList(t *testing.T) {
	client := &fakeChaosClient{
		list: []chaos.ExperimentStatus{{
			Name:      "kill-worker",
			Namespace: "cluster-probe",
			Kind:      "PodChaos",
			Phase:     "Running",
		}},
	}
	stdout := &bytes.Buffer{}

	err := runWithClient(
		context.Background(),
		[]string{"list"},
		stdout,
		&bytes.Buffer{},
		client,
		nil,
	)
	if err != nil {
		t.Fatalf("run list: %v", err)
	}
	if !strings.Contains(stdout.String(), "kill-worker") {
		t.Fatalf("expected list output, got %s", stdout.String())
	}
}

func TestRunErrors(t *testing.T) {
	client := &fakeChaosClient{err: errors.New("boom")}
	err := runWithClient(
		context.Background(),
		[]string{"status", "missing"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		client,
		nil,
	)
	if err == nil {
		t.Fatalf("expected error")
	}

	err = runWithClient(
		context.Background(),
		[]string{"apply"},
		&bytes.Buffer{},
		&bytes.Buffer{},
		client,
		nil,
	)
	if err == nil {
		t.Fatalf("expected apply validation error")
	}
}

func runWithClient(
	ctx context.Context,
	args []string,
	stdout *bytes.Buffer,
	stderr *bytes.Buffer,
	client *fakeChaosClient,
	readFile func(string) ([]byte, error),
) error {
	newClient := func(context.Context, commandConfig) (chaosAPI, error) {
		return client, nil
	}
	cfg, command, err := parseArgs(args, stdout, stderr)
	if err != nil {
		return err
	}
	cfg.stdout = stdout
	cfg.stderr = stderr
	cfg.newClient = newClient
	cfg.readFile = readFile
	if cfg.readFile == nil {
		cfg.readFile = func(string) ([]byte, error) { return nil, errors.New("unexpected read") }
	}

	switch command {
	case "apply":
		return runApply(ctx, cfg, client)
	case "status":
		return runStatus(ctx, cfg, client)
	case "delete":
		return runDelete(ctx, cfg, client)
	case "list":
		return runList(ctx, cfg, client)
	default:
		return run(ctx, args, stdout, stderr, newClient)
	}
}
