# Changelog

## Unreleased

- Raised every `internal/` package above the 70% coverage threshold.
- Repaired Helm and Kustomize validation, including opt-out of the KEDA
  `ScaledObject` from the base deployment.
- Updated deployment manifests and overlays to use the
  `slickg/clusterprobe-*` image repositories.
- Fixed workload lint findings and MongoDB test formatting.

## v0.1.0

- Scaffolded Go module, shared config, and telemetry bootstrap.
- Added database clients, messaging integration, and workload generators.
- Implemented API, worker, and UI services with OTEL instrumentation.
- Added observability stack manifests and Grafana dashboards.
- Included Chaos Mesh integration and prebuilt experiments.
- Added Helm chart, Kustomize overlays, ArgoCD applications, and CI workflows.
