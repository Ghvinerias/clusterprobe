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

The `experiments/` directory contains reusable Chaos Mesh experiments mapped to
ClusterProbe workload types.

| Experiment | Target | Workload coverage | Expected effect |
| --- | --- | --- | --- |
| `cpu-stress.yaml` | Worker pods | `cpu_burn`, `mixed` | Worker CPU saturation lowers throughput while readiness stays healthy. |
| `memory-stress-workers.yaml` | Worker pods | `mem_alloc`, `mixed` | Worker memory pressure is visible without exceeding pod limits. |
| `io-fault.yaml` | Postgres pod volume | `db_write`, `db_read`, `mixed` | Postgres storage latency increases query duration without corruption. |
| `network-delay.yaml` | Worker to Postgres | `db_write`, `db_read`, `mixed` | DB workloads slow down and recover when latency ends. |
| `network-loss-worker-postgres.yaml` | Worker to Postgres | `db_write`, `db_read`, `mixed` | Error rate becomes visible and scenarios avoid stuck running states. |
| `network-partition-api-rabbitmq.yaml` | API to RabbitMQ | scenario creation and scheduling | API reports transient messaging errors and reconnects after partition. |
| `network-partition-worker-rabbitmq.yaml` | Worker to RabbitMQ | queue draining and Worker recovery | Queued work remains durable and workers resume consumption. |
| `pod-kill-api.yaml` | API pod | API availability | API deployment replaces a killed pod and scenario creation recovers. |
| `pod-kill-worker.yaml` | Worker pod | all workload types | Worker deployment replaces a killed pod and queue depth drains. |

Workload type coverage summary:

| Workload type | Recommended experiments |
| --- | --- |
| `cpu_burn` | `cpu-stress.yaml`, `pod-kill-worker.yaml` |
| `mem_alloc` | `memory-stress-workers.yaml`, `pod-kill-worker.yaml` |
| `db_write` | `io-fault.yaml`, `network-delay.yaml`, `network-loss-worker-postgres.yaml`, `pod-kill-worker.yaml` |
| `db_read` | `io-fault.yaml`, `network-delay.yaml`, `network-loss-worker-postgres.yaml`, `pod-kill-worker.yaml` |
| `mixed` | `cpu-stress.yaml`, `memory-stress-workers.yaml`, `io-fault.yaml`, `network-delay.yaml`, `network-loss-worker-postgres.yaml`, `pod-kill-worker.yaml` |

Apply an experiment with `chaos-ctrl`:

```
go run ./cmd/chaos-ctrl apply --namespace cluster-probe -f manifests/chaos-mesh/experiments/memory-stress-workers.yaml
go run ./cmd/chaos-ctrl status --namespace cluster-probe memory-stress-workers
go run ./cmd/chaos-ctrl delete --namespace cluster-probe memory-stress-workers
```

Run one experiment at a time for baseline validation. Combine experiments only
after scenario lifecycle, queue depth, logs, metrics, and traces are known-good
for the individual fault.
