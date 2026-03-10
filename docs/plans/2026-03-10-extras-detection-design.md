# Extras Detection Design

**Date:** 2026-03-10
**Status:** Approved

## Goal

Ensure cluster state == Git state. Detect any resource that exists in the cluster but is not declared in Git.

## CLI

```bash
driftwatch scan ./manifests --detect-extras
```

Opt-in flag. Requires cluster connection. Works alongside normal drift detection.

## Three Detection Layers

### Layer 1: Flux Inventory Check

Read `status.inventory.entries[]` from all Flux Kustomization and HelmRelease CRs. Any inventory entry NOT matched by a Git-declared resource is flagged.

- **Catches:** resources Flux previously deployed that were removed from Git but not pruned
- **Severity:** Critical

### Layer 2: Namespace Resource Scan

For each namespace managed by Flux (has a Kustomization/HelmRelease targeting it), LIST all resources. Any resource not in Git sources AND not in Flux inventory is flagged.

- **Catches:** rogue `kubectl apply` in managed namespaces
- **Severity:** Warning
- **Excluded resource types** (auto-managed noise):
  - Events, Endpoints, EndpointSlices
  - Pods, ReplicaSets, ControllerRevisions
  - Leases
  - ServiceAccount token Secrets
  - discovery.k8s.io resources
- Exclude list configurable via `driftwatch.yaml`

### Layer 3: Unmanaged Namespace Audit

List all namespaces. Flag any namespace not targeted by a Flux Kustomization or HelmRelease.

- **Catches:** entire namespaces created outside GitOps
- **Severity:** Warning
- **Default ignored namespaces:** kube-system, kube-public, kube-node-lease, default (configurable)

## Coverage Matrix

| Scenario | Layer | Severity |
|-|-|-|
| Resource in Git, matches cluster | - | InSync |
| Resource in Git, differs in cluster | Existing drift detection | Per field severity |
| Resource in Git, missing from cluster | Existing detection | Critical (Missing) |
| In Flux inventory, not in Git | Layer 1 | Critical |
| In managed namespace, not in Git or Flux | Layer 2 | Warning |
| Entire namespace not managed by Flux | Layer 3 | Warning |

## Configuration

```yaml
extras:
  exclude:
    - kind: Event
    - kind: Pod
    - kind: ReplicaSet
    - kind: Endpoints
    - kind: EndpointSlice
    - kind: ControllerRevision
    - kind: Lease
  ignoreNamespaces:
    - kube-system
    - kube-public
    - kube-node-lease
    - default
```

## Reporting

```
[CRITICAL] EXTRA: default/rogue-deploy (Deployment)
  In Flux inventory but not in Git sources

[WARNING] EXTRA: prod/manual-configmap (ConfigMap)
  In managed namespace but not in Git or Flux inventory

[WARNING] UNMANAGED NAMESPACE: temp-debug
  No Flux Kustomization or HelmRelease targets this namespace
```

JSON output includes extras in the same `results` array with `status: "extra"` and a `detection_layer` field.

## Implementation Notes

- Layer 1 reads Flux CR status fields (read-only, no new RBAC needed)
- Layer 2 uses dynamic client LIST with type filtering (uses existing RBAC)
- Layer 3 uses core v1 namespace LIST (uses existing RBAC)
- All three layers run after the normal scan pipeline completes
- Results merged into the same DriftResult array before reporting
