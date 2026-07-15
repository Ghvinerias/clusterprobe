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
- Fixed workload lint findings and MongoDB test formatting.

## v0.1.0

- Scaffolded Go module, shared config, and telemetry bootstrap.
- Added database clients, messaging integration, and workload generators.
- Implemented API, worker, and UI services with OTEL instrumentation.
- Added observability stack manifests and Grafana dashboards.
- Included Chaos Mesh integration and prebuilt experiments.
- Added Helm chart, Kustomize overlays, ArgoCD applications, and CI workflows.
