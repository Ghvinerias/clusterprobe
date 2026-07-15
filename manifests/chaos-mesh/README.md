# Chaos Mesh for ClusterProbe

This directory vendors a minimal Chaos Mesh installation for airgapped clusters.
It includes the CRDs, controller-manager, and chaos-daemon.

## Privilege requirements

`chaos-daemon` must run privileged because it injects faults by manipulating
kernel primitives (cgroups, network namespaces, and filesystem hooks) and needs
access to host paths such as `/proc` and `/sys`.

## Namespace scope

Chaos Mesh is configured to only inject faults into the `cluster-probe`
namespace by enabling filter-namespace and annotating the namespace with
`chaos-mesh.org/inject=enabled`.

## Apply

Apply CRDs first, then the controller and daemon:

```
kubectl apply -f chaos-mesh-crds.yaml
kubectl apply -f chaos-mesh-controller.yaml
kubectl apply -f chaos-mesh-daemon.yaml
```

## Experiment library

The `experiments/` directory contains reusable Chaos Mesh experiments mapped to ClusterProbe workload types.

| Experiment | Primary workload coverage | Failure mode |
| --- | --- | --- |
| `cpu-stress.yaml` | `cpu_burn`, `mixed` | CPU saturation on Worker pods |
| `memory-stress-workers.yaml` | `mem_alloc`, `mixed` | Memory pressure on Worker pods |
| `io-fault.yaml` | `db_write`, `db_read`, `mixed` | Postgres storage latency |
| `network-delay.yaml` | `db_write`, `db_read`, `mixed` | Worker to Postgres latency |
| `network-loss-worker-postgres.yaml` | `db_write`, `db_read`, `mixed` | Worker to Postgres packet loss |
| `network-partition-api-rabbitmq.yaml` | scenario creation and scheduling | API to RabbitMQ partition |
| `network-partition-worker-rabbitmq.yaml` | queue draining and Worker recovery | Worker to RabbitMQ partition |
| `pod-kill-api.yaml` | API availability | API pod kill |
| `pod-kill-worker.yaml` | all workload types | Worker pod kill |

Apply an experiment with `chaos-ctrl`:

```
go run ./cmd/chaos-ctrl apply --namespace cluster-probe -f manifests/chaos-mesh/experiments/memory-stress-workers.yaml
go run ./cmd/chaos-ctrl status --namespace cluster-probe memory-stress-workers
go run ./cmd/chaos-ctrl delete --namespace cluster-probe memory-stress-workers
```

Run one experiment at a time for baseline validation. Combine experiments only
after scenario lifecycle, queue depth, logs, metrics, and traces are known-good
for the individual fault.
