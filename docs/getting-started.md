# ClusterProbe Getting Started

ClusterProbe can run as a Helm release for local validation or as Kustomize overlays for GitOps-style deployments.

## Prerequisites

- Kubernetes 1.28+
- Helm 3.12+
- `kubectl`
- A default storage class for PVC-backed components
- Chaos Mesh support if you enable real chaos experiments
- ArgoCD only if you use the manifests in `deploy/argocd`

For a local Colima/k3s cluster, use the `local-path` storage class and keep `chaos.enabled=false` unless you have confirmed Chaos Mesh works on the node.

## Quick start with Helm

Production-like install using the chart defaults:

```bash
helm upgrade --install clusterprobe ./deploy/helm/clusterprobe \
  -n cluster-probe \
  --create-namespace

kubectl -n cluster-probe get pods
```

Local Colima/k3s install:

```bash
helm upgrade --install clusterprobe ./deploy/helm/clusterprobe \
  -n cluster-probe \
  --create-namespace \
  --set global.storageClass=local-path \
  --set api.replicas=1 \
  --set worker.replicas=1 \
  --set ui.replicas=1 \
  --set chaos.enabled=false

kubectl -n cluster-probe rollout status deploy/clusterprobe-clusterprobe-api
kubectl -n cluster-probe rollout status deploy/clusterprobe-clusterprobe-ui
kubectl -n cluster-probe rollout status deploy/clusterprobe-clusterprobe-worker
```

## Open the UI and API

```bash
kubectl -n cluster-probe port-forward svc/clusterprobe-clusterprobe-ui 8081:8081
kubectl -n cluster-probe port-forward svc/clusterprobe-clusterprobe-api 8080:8080
```

- UI: `http://localhost:8081`
- API health: `http://localhost:8080/healthz`
- API status: `http://localhost:8080/api/v1/status`

## Local smoke validation

After the API and UI are reachable, run the product smoke script:

```bash
make smoke-local
```

By default it expects:

- API: `http://127.0.0.1:8080`
- UI: `http://127.0.0.1:8081`

Override those when using alternate port-forwards:

```bash
API_URL=http://127.0.0.1:18120 \
UI_URL=http://127.0.0.1:18121 \
make smoke-local
```

The smoke script verifies:

- API health and status endpoints
- UI dashboard rendering
- scenario creation and terminal status polling
- metrics snapshot production for the created scenario
- UI log SSE stream availability
- scenarios UI rendering of the created scenario

For the full local validation suite, including Go tests, race tests,
Testcontainers integration tests, Helm/Kustomize rendering, product smoke, and
browser smoke, run:

```bash
API_URL=http://127.0.0.1:8080 \
UI_URL=http://127.0.0.1:8081 \
make validate-local
```

On Colima, `scripts/validate-local.sh` automatically uses
`$HOME/.colima/default/docker.sock` when present and disables the Testcontainers
Ryuk sidecar for local runs where the host socket path cannot be mounted inside
the Colima VM. Set `CHROMIUM_EXECUTABLE_PATH` if Chromium is installed outside
`/Applications/Chromium.app`.

## First workload scenario

Create a short mixed workload through the API:

```bash
curl -sS -X POST http://localhost:8080/api/v1/scenarios \
  -H 'content-type: application/json' \
  -d '{
    "name": "smoke-mixed",
    "profile": {
      "rps": 5,
      "duration": 15000000000,
      "payload_size_bytes": 128,
      "concurrency": 2,
      "target_queue": "workload.high",
      "workload_type": "mixed"
    }
  }'
```

Then check:

```bash
curl -sS http://localhost:8080/api/v1/scenarios
curl -sS http://localhost:8080/api/v1/metrics/snapshot
```

The UI scenarios page should show the scenario moving through `running` and then `completed` after the Worker consumes it from RabbitMQ.

## Chaos experiments

The API and UI can manage Chaos Mesh experiment manifests. `chaos-ctrl` provides the same control path from the command line:

```bash
go run ./cmd/chaos-ctrl version
go run ./cmd/chaos-ctrl list --namespace cluster-probe
go run ./cmd/chaos-ctrl apply --namespace cluster-probe -f manifests/chaos-mesh/experiments/pod-kill-worker.yaml
go run ./cmd/chaos-ctrl status --namespace cluster-probe pod-kill-worker
go run ./cmd/chaos-ctrl delete --namespace cluster-probe pod-kill-worker
```

Only run these commands on clusters where Chaos Mesh CRDs and controllers are installed and permitted to inject the selected failure mode.

## Kustomize overlays

The overlays reference shared manifests outside the overlay directory, so render them with Kustomize load restrictions disabled:

```bash
kubectl kustomize deploy/kustomize/overlays/self-contained --load-restrictor=LoadRestrictionsNone | kubectl apply -f -
```

Use the existing-stack overlay when Prometheus, Loki, and Tempo already exist:

```bash
kubectl kustomize deploy/kustomize/overlays/existing-stack --load-restrictor=LoadRestrictionsNone | kubectl apply -f -
```

For Worker autoscaling with KEDA, apply the opt-in overlay after KEDA CRDs are installed:

```bash
kubectl kustomize deploy/kustomize/overlays/keda-worker --load-restrictor=LoadRestrictionsNone | kubectl apply -f -
```

## Configuration reference

- Helm values: `docs/helm-values.md`
- Notifications: `docs/notifications.md`

## Observability

- Metrics enter Prometheus through Alloy OTLP and remote write.
- Logs are collected by Alloy and sent to Loki with Kubernetes labels.
- Traces are exported over OTLP/gRPC to Tempo.
- Grafana dashboards are included for overview, workloads, chaos, and traces.

## CI/CD notes

The `ci` workflow enforces the local quality gates:

- lint
- `go build ./...`
- `go test ./...`
- race tests
- internal coverage threshold
- Testcontainers-backed integration tests
- Helm lint and render
- Kustomize base and overlay rendering
- per-service Docker image builds

The manual `build-push` workflow publishes multi-architecture images for `api`, `worker`, `ui`, and `chaos-ctrl`.

Required GitHub Actions secret:

- `BW_ACCESS_TOKEN`: Bitwarden access token used by `bitwarden/sm-action`.

The workflow currently resolves Docker Hub credentials from the Bitwarden secret IDs configured in `.github/workflows/build-push.yml` and publishes:

- `docker.io/<dockerhub-user>/clusterprobe-api:<tag>`
- `docker.io/<dockerhub-user>/clusterprobe-worker:<tag>`
- `docker.io/<dockerhub-user>/clusterprobe-ui:<tag>`
- `docker.io/<dockerhub-user>/clusterprobe-chaos-ctrl:<tag>`

## Teardown

Helm:

```bash
helm uninstall clusterprobe -n cluster-probe
```

Kustomize:

```bash
kubectl kustomize deploy/kustomize/overlays/self-contained --load-restrictor=LoadRestrictionsNone | kubectl delete -f -
```
