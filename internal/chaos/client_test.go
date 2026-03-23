package chaos

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestChaosClientApplyAndStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		chaosListKinds(),
	)
	chaosClient := NewChaosClient(client, "cluster-probe")

	spec := ExperimentSpec{
		Name: "test-chaos",
		Kind: "PodChaos",
		Spec: map[string]any{
			"action": "pod-kill",
		},
	}

	id, err := chaosClient.Apply(context.Background(), spec)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if id != "test-chaos" {
		t.Fatalf("expected id to match")
	}

	status, err := chaosClient.Status(context.Background(), id)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.Name != id {
		t.Fatalf("expected status name %s", id)
	}
}

func TestChaosClientListAndDelete(t *testing.T) {
	scheme := runtime.NewScheme()
	gvr := schema.GroupVersionResource{Group: "chaos-mesh.org", Version: "v1alpha1", Resource: "podchaos"}
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "chaos-mesh.org/v1alpha1",
		"kind":       "PodChaos",
		"metadata": map[string]any{
			"name":      "list-chaos",
			"namespace": "cluster-probe",
		},
		"spec": map[string]any{
			"action": "pod-kill",
		},
	}}

	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(
		scheme,
		chaosListKinds(),
	)
	_, err := client.Resource(chaosGVRs["PodChaos"]).Namespace("cluster-probe").Create(
		context.Background(),
		obj,
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Fatalf("seed chaos: %v", err)
	}
	chaosClient := NewChaosClient(client, "cluster-probe")

	list, err := chaosClient.List(context.Background())
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 result, got %d", len(list))
	}

	if err := chaosClient.Delete(context.Background(), "list-chaos"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err = client.Resource(gvr).Namespace("cluster-probe").Get(context.Background(), "list-chaos", metav1.GetOptions{})
	if err == nil {
		t.Fatalf("expected not found after delete")
	}
}

func chaosListKinds() map[schema.GroupVersionResource]string {
	return map[schema.GroupVersionResource]string{
		chaosGVRs["PodChaos"]:     "PodChaosList",
		chaosGVRs["NetworkChaos"]: "NetworkChaosList",
		chaosGVRs["IOChaos"]:      "IOChaosList",
		chaosGVRs["StressChaos"]:  "StressChaosList",
		chaosGVRs["TimeChaos"]:    "TimeChaosList",
	}
}
