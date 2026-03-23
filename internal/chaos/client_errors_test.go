package chaos

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestChaosClientApplyValidationErrors(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		chaosListKinds(),
	)
	chaosClient := NewChaosClient(client, "cluster-probe")

	_, err := chaosClient.Apply(context.Background(), ExperimentSpec{})
	require.Error(t, err)

	_, err = chaosClient.Apply(context.Background(), ExperimentSpec{Name: "test"})
	require.Error(t, err)

	_, err = chaosClient.Apply(context.Background(), ExperimentSpec{Name: "test", Kind: "PodChaos"})
	require.Error(t, err)

	_, err = chaosClient.Apply(context.Background(), ExperimentSpec{
		Name: "test",
		Kind: "UnknownChaos",
		Spec: map[string]any{"action": "test"},
	})
	require.Error(t, err)
}

func TestChaosClientStatusAndDeleteErrors(t *testing.T) {
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		chaosListKinds(),
	)
	chaosClient := NewChaosClient(client, "cluster-probe")

	_, err := chaosClient.Status(context.Background(), "")
	require.Error(t, err)

	_, err = chaosClient.Status(context.Background(), "missing")
	require.Error(t, err)

	err = chaosClient.Delete(context.Background(), "")
	require.Error(t, err)

	err = chaosClient.Delete(context.Background(), "missing")
	require.Error(t, err)
}

func TestStatusFromObject(t *testing.T) {
	start := time.Now().UTC().Truncate(time.Second)
	end := start.Add(1 * time.Minute)

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "chaos-mesh.org/v1alpha1",
		"kind":       "PodChaos",
		"metadata": map[string]any{
			"name":      "test",
			"namespace": "cluster-probe",
		},
		"status": map[string]any{
			"phase":     "Running",
			"message":   "ok",
			"startTime": start.Format(time.RFC3339),
			"endTime":   end.Format(time.RFC3339),
		},
	}}

	status := statusFromObject(obj)
	require.Equal(t, "test", status.Name)
	require.Equal(t, "Running", status.Phase)
	require.Equal(t, "ok", status.Message)
	require.NotNil(t, status.StartTime)
	require.NotNil(t, status.EndTime)
	require.Equal(t, start, *status.StartTime)
	require.Equal(t, end, *status.EndTime)

	obj = &unstructured.Unstructured{Object: map[string]any{
		"metadata": map[string]any{
			"name":      "fallback",
			"namespace": "cluster-probe",
		},
		"status": map[string]any{
			"startTime": "invalid",
			"endTime":   "invalid",
		},
	}}

	status = statusFromObject(obj)
	require.Equal(t, "unknown", status.Phase)
	require.Nil(t, status.StartTime)
	require.Nil(t, status.EndTime)
}
