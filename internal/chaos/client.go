package chaos

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

const chaosScope = "clusterprobe/chaos"

var chaosGroupVersion = schema.GroupVersion{Group: "chaos-mesh.org", Version: "v1alpha1"}

var chaosGVRs = map[string]schema.GroupVersionResource{
	"PodChaos":     {Group: chaosGroupVersion.Group, Version: chaosGroupVersion.Version, Resource: "podchaos"},
	"NetworkChaos": {Group: chaosGroupVersion.Group, Version: chaosGroupVersion.Version, Resource: "networkchaos"},
	"IOChaos":      {Group: chaosGroupVersion.Group, Version: chaosGroupVersion.Version, Resource: "iochaos"},
	"StressChaos":  {Group: chaosGroupVersion.Group, Version: chaosGroupVersion.Version, Resource: "stresschaos"},
	"TimeChaos":    {Group: chaosGroupVersion.Group, Version: chaosGroupVersion.Version, Resource: "timechaos"},
}

// ChaosClient wraps a Kubernetes dynamic client for Chaos Mesh resources.
type ChaosClient struct {
	client    dynamic.Interface
	namespace string
	tracer    trace.Tracer
}

// NewChaosClient constructs a ChaosClient.
func NewChaosClient(client dynamic.Interface, namespace string) *ChaosClient {
	return &ChaosClient{
		client:    client,
		namespace: namespace,
		tracer:    otel.Tracer(chaosScope),
	}
}

// Apply creates a Chaos Mesh experiment and returns the created name.
func (c *ChaosClient) Apply(ctx context.Context, spec ExperimentSpec) (string, error) {
	ctx, span := c.tracer.Start(ctx, "chaos.apply")
	defer span.End()

	if spec.Name == "" {
		err := fmt.Errorf("experiment name is required")
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid name")
		return "", err
	}
	if spec.Kind == "" {
		err := fmt.Errorf("experiment kind is required")
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid kind")
		return "", err
	}
	if spec.Spec == nil {
		err := fmt.Errorf("experiment spec is required")
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid spec")
		return "", err
	}

	namespace := spec.Namespace
	if namespace == "" {
		namespace = c.namespace
	}

	gvr, ok := chaosGVRs[spec.Kind]
	if !ok {
		err := fmt.Errorf("unsupported chaos kind %s", spec.Kind)
		span.RecordError(err)
		span.SetStatus(codes.Error, "unsupported kind")
		return "", err
	}

	span.SetAttributes(
		attribute.String("chaos.kind", spec.Kind),
		attribute.String("chaos.name", spec.Name),
		attribute.String("chaos.namespace", namespace),
	)

	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": chaosGroupVersion.String(),
		"kind":       spec.Kind,
		"metadata": map[string]any{
			"name":      spec.Name,
			"namespace": namespace,
		},
		"spec": spec.Spec,
	}}

	created, err := c.client.Resource(gvr).Namespace(namespace).Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "create failed")
		return "", fmt.Errorf("create chaos experiment: %w", err)
	}

	return created.GetName(), nil
}

// Status returns the status of an experiment by name.
func (c *ChaosClient) Status(ctx context.Context, id string) (ExperimentStatus, error) {
	ctx, span := c.tracer.Start(ctx, "chaos.status")
	defer span.End()

	if id == "" {
		err := fmt.Errorf("experiment id is required")
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid id")
		return ExperimentStatus{}, err
	}

	for kind, gvr := range chaosGVRs {
		obj, err := c.client.Resource(gvr).Namespace(c.namespace).Get(ctx, id, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "get failed")
			return ExperimentStatus{}, fmt.Errorf("get chaos experiment: %w", err)
		}
		status := statusFromObject(obj)
		status.Kind = kind
		return status, nil
	}

	err := fmt.Errorf("experiment %s not found", id)
	span.RecordError(err)
	span.SetStatus(codes.Error, "not found")
	return ExperimentStatus{}, err
}

// Delete removes an experiment by name.
func (c *ChaosClient) Delete(ctx context.Context, id string) error {
	ctx, span := c.tracer.Start(ctx, "chaos.delete")
	defer span.End()

	if id == "" {
		err := fmt.Errorf("experiment id is required")
		span.RecordError(err)
		span.SetStatus(codes.Error, "invalid id")
		return err
	}

	for _, gvr := range chaosGVRs {
		err := c.client.Resource(gvr).Namespace(c.namespace).Delete(ctx, id, metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "delete failed")
			return fmt.Errorf("delete chaos experiment: %w", err)
		}
		return nil
	}

	return fmt.Errorf("experiment %s not found", id)
}

// List returns the status of all experiments in the namespace.
func (c *ChaosClient) List(ctx context.Context) ([]ExperimentStatus, error) {
	ctx, span := c.tracer.Start(ctx, "chaos.list")
	defer span.End()

	var results []ExperimentStatus
	for kind, gvr := range chaosGVRs {
		list, err := c.client.Resource(gvr).Namespace(c.namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "list failed")
			return nil, fmt.Errorf("list chaos experiments: %w", err)
		}
		for i := range list.Items {
			status := statusFromObject(&list.Items[i])
			status.Kind = kind
			results = append(results, status)
		}
	}
	return results, nil
}

func statusFromObject(obj *unstructured.Unstructured) ExperimentStatus {
	status := ExperimentStatus{
		ID:        obj.GetName(),
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Kind:      obj.GetKind(),
		Phase:     "unknown",
	}

	if phase, found, _ := unstructured.NestedString(obj.Object, "status", "phase"); found && phase != "" {
		status.Phase = phase
	}
	if message, found, _ := unstructured.NestedString(obj.Object, "status", "message"); found {
		status.Message = message
	}
	if start, found, _ := unstructured.NestedString(obj.Object, "status", "startTime"); found {
		if parsed, err := time.Parse(time.RFC3339, start); err == nil {
			status.StartTime = &parsed
		}
	}
	if end, found, _ := unstructured.NestedString(obj.Object, "status", "endTime"); found {
		if parsed, err := time.Parse(time.RFC3339, end); err == nil {
			status.EndTime = &parsed
		}
	}

	return status
}
