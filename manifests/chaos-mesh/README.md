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
