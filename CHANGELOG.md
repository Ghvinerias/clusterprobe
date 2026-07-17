# Changelog

## Unreleased

- Added opt-in live Chaos Mesh smoke validation for Worker-targeted stress experiments.
- Added release image verification and OCI revision labels for service images.
- Added live HTMX refresh for scenario lifecycle events on scenario detail pages.
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
- Updated Makefile developer targets for per-service Docker builds, full
  integration test packages, race tests, and Kustomize rendering.
- Fixed workload lint findings and MongoDB test formatting.
- Filtered API scenario status queries to lifecycle events so workload rows do
  not temporarily blank scenario state.
- Kept API-rendered chaos status badges polling while experiments are running.
- Split scenario lifecycle records into `scenario_events` so workload samples in
  `load_events` cannot affect scenario state.
- Added a repeatable local product smoke script for API, UI, scenario,
  metrics, and log-stream validation.
- Added live-polling scenario table rows in the UI and hid stop actions for
  terminal scenarios.
- Synchronized Chaos Mesh live status back into stored experiment metadata and
  removed Mongo records after successful experiment deletion.
- Expanded CI to cover build, race, coverage, integration, Helm, Kustomize, and
  Docker image validation, and documented the local smoke workflow.
- Completed bundled Chaos Mesh prerequisites with leader-election lease RBAC
  and the missing controller CRDs required for self-contained reconciliation.
- Made Worker startup declare RabbitMQ topology and reconnect consumer channels
  after consume setup failures so scenarios do not remain queued after fresh
  cluster startup.
- Added chaos experiment status fallback from Kubernetes object duration when
  Chaos Mesh does not publish a `status.phase` field.
- Added Playwright browser smoke coverage for dashboard, scenarios, chaos, and
  logs against a live local deployment.
- Added scenario and chaos experiment detail pages with lifecycle/configuration
  visibility from the UI.
- Added `make validate-local` and `make validate-local-k8s` for repeatable full
  local validation across Go, integration, Helm, Kustomize, smoke, and browser
  checks.
- Added a Worker readiness endpoint that only reports ready after RabbitMQ
  consumers are initialized.
- Excluded local dependencies, reports, screenshots, and handoff artifacts from
  Docker build contexts.
- Fixed scenario Stop handling so stopped scenarios preserve their name/profile
  metadata and Worker terminal updates do not overwrite a user-requested stop.
- Expanded local product smoke validation to cover scenario Stop API behavior
  and stopped scenario rendering in the UI.
- Added Worker-side cancellation for in-flight stopped scenarios and made CPU,
  DB write, and DB read workloads honor context cancellation promptly.

## v0.1.0

- Scaffolded Go module, shared config, and telemetry bootstrap.
- Added database clients, messaging integration, and workload generators.
- Implemented API, worker, and UI services with OTEL instrumentation.
- Added observability stack manifests and Grafana dashboards.
- Included Chaos Mesh integration and prebuilt experiments.
- Added Helm chart, Kustomize overlays, ArgoCD applications, and CI workflows.
