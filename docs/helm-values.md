# Helm Values Reference

This document lists every key in `deploy/helm/clusterprobe/values.yaml`, with type, default, and a short description.

## Global

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `global.namespace` | string | `cluster-probe` | Kubernetes namespace to deploy into when not using the release namespace. |
| `global.imagePullPolicy` | string | `IfNotPresent` | Image pull policy applied to all containers. |
| `global.storageClass` | string | `longhorn` | Storage class for PVCs. |
| `global.secretName` | string | `""` | Optional secret name to mount as `envFrom` for app services. |
| `global.environment` | string | `dev` | Environment label injected into service env. |
| `global.chaosListenAddr` | string | `:8082` | Placeholder listen address for chaos endpoints. |

## API

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `api.enabled` | bool | `true` | Deploy the API service. |
| `api.image.repository` | string | `docker.io/clusterprobe/clusterprobe-api` | API container image repository. |
| `api.image.tag` | string | `latest` | API image tag. |
| `api.replicas` | int | `2` | API replicas. |
| `api.service.type` | string | `ClusterIP` | API service type. |
| `api.service.port` | int | `8080` | API service port. |
| `api.resources.requests.cpu` | string | `100m` | API CPU request. |
| `api.resources.requests.memory` | string | `128Mi` | API memory request. |
| `api.resources.limits.cpu` | string | `500m` | API CPU limit. |
| `api.resources.limits.memory` | string | `512Mi` | API memory limit. |
| `api.config.listenAddr` | string | `:8080` | API listen address. |
| `api.config.logLevel` | string | `info` | API log level. |

## Worker

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `worker.enabled` | bool | `true` | Deploy the worker service. |
| `worker.image.repository` | string | `docker.io/clusterprobe/clusterprobe-worker` | Worker image repository. |
| `worker.image.tag` | string | `latest` | Worker image tag. |
| `worker.replicas` | int | `3` | Worker replicas. |
| `worker.service.type` | string | `ClusterIP` | Worker service type. |
| `worker.service.port` | int | `8080` | Worker service port. |
| `worker.resources.requests.cpu` | string | `100m` | Worker CPU request. |
| `worker.resources.requests.memory` | string | `128Mi` | Worker memory request. |
| `worker.resources.limits.cpu` | string | `1000m` | Worker CPU limit. |
| `worker.resources.limits.memory` | string | `512Mi` | Worker memory limit. |
| `worker.config.concurrency` | int | `10` | Worker goroutine concurrency. |
| `worker.config.logLevel` | string | `info` | Worker log level. |

## UI

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `ui.enabled` | bool | `true` | Deploy the UI service. |
| `ui.image.repository` | string | `docker.io/clusterprobe/clusterprobe-ui` | UI image repository. |
| `ui.image.tag` | string | `latest` | UI image tag. |
| `ui.replicas` | int | `2` | UI replicas. |
| `ui.service.type` | string | `ClusterIP` | UI service type. |
| `ui.service.port` | int | `8081` | UI service port. |
| `ui.resources.requests.cpu` | string | `50m` | UI CPU request. |
| `ui.resources.requests.memory` | string | `128Mi` | UI memory request. |
| `ui.resources.limits.cpu` | string | `300m` | UI CPU limit. |
| `ui.resources.limits.memory` | string | `256Mi` | UI memory limit. |
| `ui.config.listenAddr` | string | `:8081` | UI listen address. |
| `ui.config.apiBaseURL` | string | `http://clusterprobe-api:8080` | API base URL the UI calls. |
| `ui.config.logLevel` | string | `info` | UI log level. |

## Observability

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `observability.selfContained` | bool | `true` | Deploy Prometheus, Loki, Tempo, Grafana when true. |
| `observability.externalEndpoints.prometheus` | string | `""` | Remote-write endpoint when using an existing stack. |
| `observability.externalEndpoints.loki` | string | `""` | Loki push endpoint when using an existing stack. |
| `observability.externalEndpoints.tempo` | string | `""` | Tempo OTLP endpoint when using an existing stack. |
| `observability.alloy.image.repository` | string | `grafana/alloy` | Alloy image repository. |
| `observability.alloy.image.tag` | string | `v1.6.1` | Alloy image tag. |
| `observability.alloy.resources.requests.cpu` | string | `100m` | Alloy CPU request. |
| `observability.alloy.resources.requests.memory` | string | `128Mi` | Alloy memory request. |
| `observability.alloy.resources.limits.cpu` | string | `500m` | Alloy CPU limit. |
| `observability.alloy.resources.limits.memory` | string | `512Mi` | Alloy memory limit. |
| `observability.prometheus.image.repository` | string | `prom/prometheus` | Prometheus image repository. |
| `observability.prometheus.image.tag` | string | `v2.53.1` | Prometheus image tag. |
| `observability.prometheus.storage` | string | `20Gi` | Prometheus PVC size. |
| `observability.loki.image.repository` | string | `grafana/loki` | Loki image repository. |
| `observability.loki.image.tag` | string | `3.1.0` | Loki image tag. |
| `observability.loki.storage` | string | `20Gi` | Loki PVC size. |
| `observability.tempo.image.repository` | string | `grafana/tempo` | Tempo image repository. |
| `observability.tempo.image.tag` | string | `2.5.0` | Tempo image tag. |
| `observability.tempo.storage` | string | `10Gi` | Tempo PVC size. |
| `observability.grafana.image.repository` | string | `grafana/grafana` | Grafana image repository. |
| `observability.grafana.image.tag` | string | `11.2.0` | Grafana image tag. |
| `observability.grafana.storage` | string | `5Gi` | Grafana PVC size. |
| `observability.grafana.adminUser` | string | `admin` | Grafana admin user. |
| `observability.grafana.adminPassword` | string | `admin` | Grafana admin password. |

## Postgres

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `postgres.enabled` | bool | `true` | Deploy Postgres. |
| `postgres.image.repository` | string | `postgres` | Postgres image repository. |
| `postgres.image.tag` | string | `16` | Postgres image tag. |
| `postgres.storage` | string | `20Gi` | Postgres PVC size. |
| `postgres.username` | string | `clusterprobe` | Postgres username. |
| `postgres.password` | string | `changeme` | Postgres password. |
| `postgres.database` | string | `clusterprobe` | Postgres database. |

## Redis

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `redis.enabled` | bool | `true` | Deploy Redis. |
| `redis.image.repository` | string | `redis` | Redis image repository. |
| `redis.image.tag` | string | `7` | Redis image tag. |
| `redis.storage` | string | `5Gi` | Redis PVC size. |

## MongoDB

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `mongodb.enabled` | bool | `true` | Deploy MongoDB. |
| `mongodb.image.repository` | string | `mongo` | MongoDB image repository. |
| `mongodb.image.tag` | string | `7` | MongoDB image tag. |
| `mongodb.storage` | string | `20Gi` | MongoDB PVC size. |

## RabbitMQ

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `rabbitmq.enabled` | bool | `true` | Deploy RabbitMQ. |
| `rabbitmq.image.repository` | string | `rabbitmq` | RabbitMQ image repository. |
| `rabbitmq.image.tag` | string | `3.13-management` | RabbitMQ image tag. |
| `rabbitmq.storage` | string | `10Gi` | RabbitMQ PVC size. |

## Chaos Mesh

| Key | Type | Default | Description |
| --- | --- | --- | --- |
| `chaos.enabled` | bool | `true` | Deploy Chaos Mesh components. |
| `chaos.controller.image.repository` | string | `ghcr.io/chaos-mesh/chaos-mesh` | Chaos Mesh controller image. |
| `chaos.controller.image.tag` | string | `v2.7.0` | Chaos Mesh controller tag. |
| `chaos.daemon.image.repository` | string | `ghcr.io/chaos-mesh/chaos-daemon` | Chaos Mesh daemon image. |
| `chaos.daemon.image.tag` | string | `v2.7.0` | Chaos Mesh daemon tag. |
