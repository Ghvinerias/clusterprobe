# Self-Contained Overlay

This overlay deploys the full ClusterProbe stack plus the observability stack (Alloy, Prometheus, Loki, Tempo, Grafana) and Chaos Mesh.

## Prerequisites

- Kubernetes 1.28+
- Longhorn storage class installed and set as `longhorn`
- nginx-ingress controller (for Grafana ingress)

## Deploy

```bash
kubectl apply -k deploy/kustomize/overlays/self-contained
```

## Notes

- Replicas for API, worker, and UI are reduced to 1 for homelab-friendly sizing.
- Grafana is exposed via the ingress manifest under `grafana.clusterprobe.local`.
