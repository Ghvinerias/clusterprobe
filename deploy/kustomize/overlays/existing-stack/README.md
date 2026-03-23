# Existing Observability Stack Overlay

This overlay deploys only the ClusterProbe Alloy agent and wires it to an
existing Prometheus/Loki/Tempo stack.

## Required edits

Edit `deploy/kustomize/overlays/existing-stack/alloy-config.river` before
applying this overlay and set the following placeholders:

- `REPLACE_CLUSTER_NAME`: Cluster label applied to all log streams.
- `REPLACE_PROM_REMOTE_WRITE_URL`: Prometheus remote_write endpoint.
- `REPLACE_LOKI_PUSH_URL`: Loki push endpoint.
- `REPLACE_TEMPO_OTLP_ENDPOINT`: Tempo OTLP gRPC endpoint.

Then run:

```
kustomize build deploy/kustomize/overlays/existing-stack | kubectl apply -f -
```
