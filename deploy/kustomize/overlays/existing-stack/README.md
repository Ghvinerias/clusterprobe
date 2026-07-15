# Existing Stack Overlay

This overlay deploys ClusterProbe services plus Alloy, and connects to an existing Prometheus/Loki/Tempo stack.

## Required Values

Update the Alloy environment variables in `manifests/observability/alloy/alloy-daemonset.yaml` or apply a local patch for your cluster:

- `PROM_REMOTE_WRITE_URL`: Prometheus remote write URL
- `LOKI_URL`: Loki push URL
- `TEMPO_OTLP_ENDPOINT`: Tempo OTLP gRPC endpoint

## Deploy

```bash
kubectl kustomize deploy/kustomize/overlays/existing-stack --load-restrictor=LoadRestrictionsNone | kubectl apply -f -
```

## Notes

- This overlay does not deploy Grafana, Prometheus, Loki, or Tempo.
- Chaos Mesh is still deployed for local chaos experiments.
