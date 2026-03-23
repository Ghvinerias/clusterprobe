# ArgoCD Applications

This directory contains ArgoCD `Application` manifests for ClusterProbe.

## Apply

```bash
kubectl apply -f deploy/argocd/clusterprobe-self-contained.yaml
kubectl apply -f deploy/argocd/clusterprobe-existing-stack.yaml
```

## Customize

- Set `spec.source.targetRevision` to a tag or commit SHA for reproducible deployments.
- Update `spec.source.repoURL` if you are using a fork.
