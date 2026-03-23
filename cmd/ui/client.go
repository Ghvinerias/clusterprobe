package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Ghvinerias/clusterprobe/internal/workload"
)

type apiClient struct {
	baseURL string
	client  *http.Client
}

func newAPIClient(baseURL string) *apiClient {
	base := strings.TrimSuffix(strings.TrimSpace(baseURL), "/")
	if base == "" {
		base = "http://localhost:8080"
	}
	return &apiClient{
		baseURL: base,
		client:  &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *apiClient) ListScenarios(ctx context.Context) ([]workload.ScenarioResponse, error) {
	var out []workload.ScenarioResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/scenarios", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *apiClient) CreateScenario(
	ctx context.Context,
	req workload.ScenarioRequest,
) (workload.ScenarioResponse, error) {
	var out workload.ScenarioResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/scenarios", req, &out); err != nil {
		return workload.ScenarioResponse{}, err
	}
	return out, nil
}

func (c *apiClient) StopScenario(ctx context.Context, id string) (workload.ScenarioResponse, error) {
	var out workload.ScenarioResponse
	path := fmt.Sprintf("/api/v1/scenarios/%s/stop", id)
	if err := c.doJSON(ctx, http.MethodPut, path, nil, &out); err != nil {
		return workload.ScenarioResponse{}, err
	}
	return out, nil
}

func (c *apiClient) ListExperiments(ctx context.Context) ([]workload.ChaosExperimentResponse, error) {
	var out []workload.ChaosExperimentResponse
	if err := c.doJSON(ctx, http.MethodGet, "/api/v1/chaos/experiments", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *apiClient) CreateExperiment(
	ctx context.Context,
	req workload.ChaosExperimentRequest,
) (workload.ChaosExperimentResponse, error) {
	var out workload.ChaosExperimentResponse
	if err := c.doJSON(ctx, http.MethodPost, "/api/v1/chaos/experiments", req, &out); err != nil {
		return workload.ChaosExperimentResponse{}, err
	}
	return out, nil
}

func (c *apiClient) LogsStream(ctx context.Context) (io.ReadCloser, error) {
	url := c.baseURL + "/api/v1/logs/stream"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("logs stream request: %w", err)
	}
	if resp.StatusCode >= http.StatusBadRequest {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("logs stream status: %s", resp.Status)
	}
	return resp.Body, nil
}

func (c *apiClient) doJSON(ctx context.Context, method, path string, payload any, dest any) error {
	url := c.baseURL + path
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("encode payload: %w", err)
		}
		body = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("api error: %s", resp.Status)
	}

	if dest == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(dest); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
