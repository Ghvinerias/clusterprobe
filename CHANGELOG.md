# Changelog

## Unreleased

- Raised every `internal/` package above the 70% coverage threshold.
- Repaired Helm and Kustomize validation, including opt-out of the KEDA
  `ScaledObject` from the base deployment.
- Updated deployment manifests and overlays to use the
  `slickg/clusterprobe-*` image repositories.
- Added explicit semver tags to manually dispatched release image builds.
- Fixed local Colima deployment issues: Docker target-arch builds, automatic
  Postgres schema initialization, UI health/API wiring, Worker probe port, and
  Alloy/Tempo Helm configuration.
- Fixed scenario lifecycle reporting so Worker executions append running and
  terminal status events, and API scenario lists show the latest event per
  scenario.
- Wired Alloy OTLP metrics into Prometheus remote write and made Alloy roll
  automatically when its rendered config changes.
- Added the API log stream route used by the UI logs page.
- Derived API scenario status counters from the latest persisted scenario
  events.
- Added bounded RabbitMQ startup retry for API and Worker processes.
- Added API and Worker startup probes so dependency retry is not interrupted
  by liveness checks.
- Added `chaos-ctrl`, a small CLI for applying, listing, checking, and deleting
  Chaos Mesh experiments from manifests.
- Added the `chaos-ctrl` container image to the manual release workflow.
- Added a RabbitMQ testcontainers integration test covering topology,
  publishing, and consumption.
- Added opt-in KEDA Worker autoscaling for Helm and Kustomize, and moved the
  ScaledObject out of the base manifests.
- Aligned Kustomize Alloy and Prometheus observability manifests with the
  Helm-validated telemetry flow.
- Refreshed getting-started, Helm values, README, and notification docs to
  match the current Helm, Kustomize, KEDA, and `chaos-ctrl` workflows.
- Completed the chaos UI control loop with UI-local status polling, delete
  actions, broader experiment type options, and handler coverage.
- Expanded the Chaos Mesh experiment library with memory pressure, Postgres
  packet loss, and Worker/RabbitMQ partition cases plus an experiment matrix.
- Made the Helm KEDA `ScaledObject` template safe for older releases that do
  not yet have `worker.autoscaling.keda` values.
- Treated empty DB reads as non-fatal so first-run mixed workloads can complete
  before enough rows exist in the read window.
- Added Helm API ServiceAccount and Chaos Mesh RBAC so API-triggered
  experiments have the same permissions as the Kustomize deployment.
- Fixed the Helm and Kustomize Chaos Mesh controller command for bundled
  Chaos Mesh images.
- Fixed the Helm and Kustomize Chaos Mesh daemon command and added configurable
  runtime socket settings.
- Vendored Chaos Mesh CRDs into the Helm chart and added controller webhook
  certificates for self-contained local installs.
- Defaulted Chaos Mesh daemon runtime socket to the generic containerd path used
  by Colima and other containerd-based clusters.
- Allowed `chaos-ctrl` global flags before or after the subcommand.
- Removed stale Helm install notes, replaced deprecated Kustomize fields, and
  dropped the README WIP badge after local end-to-end validation.
- Fixed workload lint findings and MongoDB test formatting.

## v0.1.0

- Scaffolded Go module, shared config, and telemetry bootstrap.
- Added database clients, messaging integration, and workload generators.
- Implemented API, worker, and UI services with OTEL instrumentation.
- Added observability stack manifests and Grafana dashboards.
- Included Chaos Mesh integration and prebuilt experiments.
- Added Helm chart, Kustomize overlays, ArgoCD applications, and CI workflows.
