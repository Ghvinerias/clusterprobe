# Existing Stack Overlay

This overlay deploys ClusterProbe services plus Alloy, and connects to an existing Prometheus/Loki/Tempo stack.

## Required Values

Update `deploy/kustomize/overlays/existing-stack/alloy-config.river` with your endpoints:

- `${PROMETHEUS_URL}`: Prometheus remote write URL
- `${LOKI_URL}`: Loki push URL
- `${TEMPO_URL}`: Tempo OTLP gRPC endpoint

## Deploy

```bash
kubectl apply -k deploy/kustomize/overlays/existing-stack
```

## Notes

- This overlay does not deploy Grafana, Prometheus, Loki, or Tempo.
- Chaos Mesh is still deployed for local chaos experiments.
