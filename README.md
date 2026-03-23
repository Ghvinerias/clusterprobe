# ClusterProbe

![Status: WIP](https://img.shields.io/badge/status-wip-yellow)

ClusterProbe is a Kubernetes synthetic load testing harness designed to deploy a
self-contained stack, generate realistic workloads, and validate cluster
behavior under stress for on-premise environments.

**Architecture (high-level)**
```
+---------+     +------------+     +----------+
|  UI     | --> |   API      | --> | RabbitMQ |
+---------+     +------------+     +----------+
     |               |                 |
     |               v                 v
     |          +---------+       +---------+
     |          | Postgres|       | Worker  |
     |          +---------+       +---------+
     |               ^                 |
     |               |                 v
     |          +---------+       +---------+
     +--------> | Redis   |       | MongoDB |
                +---------+       +---------+

Observability: Alloy -> Prometheus/Loki/Tempo -> Grafana
Chaos: Chaos Mesh experiments triggered via API/UI
```

**Features**
- Synthetic workload generators (CPU, memory, DB read/write, mixed)
- REST API + HTMX UI for scenario management
- RabbitMQ-based scheduling and result fan-out
- Built-in observability with OTEL, Prometheus, Loki, and Tempo
- Chaos Mesh integrations and prebuilt experiments

**Quick Start (Helm)**
```bash
helm install clusterprobe ./deploy/helm/clusterprobe -n cluster-probe --create-namespace
kubectl -n cluster-probe get pods
```

**Docs**
- Getting started: `docs/getting-started.md`
- Helm values: `docs/helm-values.md`

