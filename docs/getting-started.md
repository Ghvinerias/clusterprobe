# ClusterProbe Getting Started

## Prerequisites

- Kubernetes 1.28+
- Longhorn storage class installed
- nginx-ingress controller
- ArgoCD (optional, for GitOps)

## Quick Start with Helm

```bash
helm repo add clusterprobe https://ghvinerias.github.io/clusterprobe
helm install clusterprobe clusterprobe/clusterprobe -n cluster-probe --create-namespace
kubectl -n cluster-probe get pods
```

## Quick Start with Kustomize + ArgoCD

```bash
kubectl apply -f deploy/argocd/clusterprobe-self-contained.yaml
kubectl -n argocd get applications
```

To use an existing observability stack, apply `deploy/argocd/clusterprobe-existing-stack.yaml` and update the Alloy endpoints in `deploy/kustomize/overlays/existing-stack/alloy-config.river`.

## Configuration Reference

See `docs/helm-values.md` for all Helm values.

## First Load Test Walkthrough

1. Port-forward the UI service:
   ```bash
   kubectl -n cluster-probe port-forward svc/clusterprobe-ui 8081:8081
   ```
2. Open `http://localhost:8081` and create a new scenario.
3. Watch Grafana dashboards (self-contained overlay) for ops/sec, error rate, and latency.
4. Trigger a chaos experiment from the UI to validate resilience.

## Observability

- Metrics: open Grafana and use the Overview and Workloads dashboards.
- Logs: use the Loki datasource and filter by `service` or `pod` labels.
- Traces: open the Tempo datasource, filter by `service` and `trace_id`.

## CI/CD Notes

The `build-push` workflow requires Bitwarden secrets for Docker Hub:

1. Add `BW_ACCESS_TOKEN` to GitHub Actions secrets.
2. Replace the two UUID placeholders in `.github/workflows/build-push.yml` with your Bitwarden secret IDs for `DOCKERHUB_USERNAME` and `DOCKERHUB_SECRET`.
3. Images publish to `docker.io/{username}/clusterprobe-{service}`.

## Teardown

```bash
kubectl delete -k deploy/kustomize/overlays/self-contained
```
