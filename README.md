# ClusterProbe

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
- Scenario stop controls cancel queued and in-flight Worker execution
- RabbitMQ-based scheduling and result fan-out
- Built-in observability with OTEL, Prometheus, Loki, and Tempo
- Chaos Mesh integrations and prebuilt experiments

**Quick Start (Helm)**

```bash
helm install clusterprobe ./deploy/helm/clusterprobe -n cluster-probe --create-namespace
kubectl -n cluster-probe get pods
```

For a full local deploy-and-validate cycle on Colima/k3s, run:

```bash
make validate-local-k8s
```

That target builds local service images, upgrades the Helm release, waits for
Kubernetes readiness, creates temporary port-forwards, and runs Go,
integration, smoke, and Playwright browser checks. Use `make smoke-local` when
the API and UI are already reachable and you only need the product smoke.

**Docs**
- User guide: `docs/user-guide.md`
- Getting started: `docs/getting-started.md`
- Helm values: `docs/helm-values.md`
- Notifications: `docs/notifications.md`
