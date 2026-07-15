# KEDA Worker Autoscaling Overlay

This optional overlay adds the Worker `ScaledObject` for RabbitMQ queue-based
autoscaling.

Use it only in clusters where KEDA CRDs are already installed:

```bash
kubectl apply -k deploy/kustomize/overlays/keda-worker
```

The base deployment intentionally excludes this overlay so clusters without
KEDA can still apply the default manifests.
