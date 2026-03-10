# Driftwatch Design Document

**Date:** 2026-03-10
**Status:** Approved

## Overview

Driftwatch is a Go CLI tool that compares Git-stored Kubernetes manifests (plain YAML, Helm, Kustomize) against live cluster state and flags any drift. It provides first-class FluxCD integration and is designed with security as a core principle.

## Architecture

Pipeline architecture with distinct stages:

```
CLI (cobra)
  -> Discovery (walks dirs, detects source types, reads config)
    -> Rendering (produces expected K8s objects per source type)
      -> Fetching (gets live objects from cluster via k8s API)
        -> Diffing (structured field-by-field comparison)
          -> Flux Enrichment (overlays reconciliation status)
            -> Reporting (terminal/JSON output, severity, exit codes)
```

Each stage is a Go interface. Data flows as typed structs.

## Data Model

### ResourceIdentifier

Uniquely identifies a K8s resource: APIVersion, Kind, Namespace, Name.

### ResourcePair

Holds expected vs live state for comparison, plus source provenance (type, path, optional FluxRef).

### DriftResult

Output of diffing: resource ID, source info, drift status (InSync/Drifted/Missing/Extra), per-field diffs with severity, optional Flux reconciliation status.

### FieldDiff

Single field difference: JSON path, expected value, actual value, severity level.

## Pipeline Stages

### Discovery

Walks target directory, identifies source types by convention:
- `Chart.yaml` -> Helm chart
- `kustomization.yaml` -> Kustomize overlay
- `.yaml`/`.yml` with `apiVersion`/`kind` -> plain manifests
- `driftwatch.yaml` at root -> config file overrides

### Rendering

Each source type implements a `Renderer` interface:
- **ManifestRenderer** -- parses multi-doc YAML, validates against K8s schema
- **HelmRenderer** -- uses `helm.sh/helm/v3/pkg/engine` for in-process rendering. Supports both local charts and Flux HelmRelease-as-source (reads HelmRelease CR spec for chart ref, version, inline values)
- **KustomizeRenderer** -- uses `sigs.k8s.io/kustomize/api/krusty` for in-process build

### Fetching

Uses `client-go` dynamic client (GET/LIST only). Rate-limited (default 10 QPS), per-request timeout (default 10s).

### Diffing

Structured comparison using `github.com/google/go-cmp` with:
- Configurable ignore rules (default: managedFields, resourceVersion, uid, generation, status)
- Severity classification based on field path matching
- Secret data fields stripped before comparison
- SealedSecrets are safe to diff

### Flux Enrichment

When Flux CRDs detected in cluster:
1. Reads Kustomization and HelmRelease CRs
2. Resolves GitRepository/HelmRepository sources -> expected revision
3. Checks reconciliation conditions (Ready, Stalled, Failed)
4. Overlays FluxStatus onto matching DriftResult entries

Runs in parallel with diffing, results merged.

### Reporting

- **Terminal:** colored diff output, summary table with severity counts
- **JSON:** structured `[]DriftResult` for machine consumption
- **Exit codes:** 0 = no drift above threshold, 1 = drift above threshold, 2 = error
- `--fail-on=critical|warning|info` controls threshold

## Configuration

### driftwatch.yaml

```yaml
sources:
  - path: ./infra/base
    type: kustomize
    namespace: infra
  - path: ./apps/api
    type: helm
    values:
      - values.yaml
      - values-prod.yaml
ignore:
  fields:
    - "metadata.managedFields"
    - "metadata.annotations.kubectl.kubernetes.io/*"
    - "metadata.resourceVersion"
    - "metadata.uid"
    - "metadata.generation"
    - "status"
  resources:
    - kind: Secret
severity:
  critical:
    - "spec.containers.*.image"
    - "spec.template.spec.containers.*.image"
    - "rules"
    - "spec.ports"
  warning:
    - "spec.replicas"
    - "spec.template.spec.resources"
cluster:
  kubeconfig: ""
  context: ""
flux:
  enabled: true
```

Strict schema validation -- unknown keys rejected.

## CLI Interface

```
driftwatch scan [path] [flags]
driftwatch init
driftwatch version
driftwatch validate
```

### Flags

| Flag | Default | Description |
|-|-|-|
| `--config` | `./driftwatch.yaml` | Config file path |
| `--kubeconfig` | `~/.kube/config` | Kubeconfig path |
| `--context` | current context | Kubernetes context |
| `--namespace` | all | Limit to namespace(s) |
| `--source-type` | auto | Force source type |
| `--output` | `terminal` | Output format: terminal, json |
| `--fail-on` | `critical` | Severity threshold for exit code |
| `--flux` | auto | Flux enrichment mode |

## Fleet-Infra Compatibility

Designed for compatibility with the fleet-infra repository pattern:
- HelmRelease-as-source: reads HelmRelease CR spec (chart, version, inline values) and renders from remote chart
- Inline values: extracts values from `spec.values` in HelmRelease CRs
- SealedSecrets: supported as diffable resource (encrypted, safe)
- Flux dependency chain: repositories -> infrastructure -> apps ordering reflected in scan grouping
- Cluster config: ConfigMap-based cluster variables supported

## Security Design

### Principles

- **Read-only cluster access** -- GET/LIST only, never writes
- **No secrets in output** -- Secret resources excluded by default, redaction patterns for sensitive fields
- **No shell execution** -- Helm/Kustomize rendering via Go libraries, not CLI subprocesses
- **Input validation** -- paths sandboxed to repo root, symlinks not followed, absolute paths rejected
- **YAML safety** -- size limits (10MB/file), anchor/alias bomb protection, schema validation

### Threat Mitigations

| Threat | Mitigation |
|-|-|
| Credential leakage in output | Secrets excluded, redaction patterns for `*secret*`, `*password*`, `*token*`, `*key*` |
| Malicious manifests | YAML size limits, bomb protection, schema validation |
| Helm template injection | In-process rendering, no shell, values validated |
| Kustomize exec plugins | Disabled by default, explicit opt-in with warning |
| Path traversal | Relative paths only, symlinks blocked, absolute paths rejected |
| Cluster API abuse | Read-only, rate-limited, timeouts, no write ops compiled in |
| Supply chain | Minimal deps, checksum verification, govulncheck in CI |
| Output injection | Control characters stripped from terminal output |
| Config tampering | Strict schema, unknown keys rejected, warn on broad ignore rules |

### RBAC

Read-only ClusterRole shipped with docs. Scoped variant provided for least-privilege environments.

### Audit Trail

Every scan logs: timestamp, cluster context, namespaces, resource count, drift count. JSON output includes metadata block with scan provenance. No sensitive data in logs.

## Testing Strategy

### Unit Tests

Per stage with mocked inputs/outputs:
- Discovery: mock filesystem with source type combinations
- Rendering: fixture manifests/charts/kustomizations per renderer
- Diffing: known pairs -> expected FieldDiffs and severity
- Reporting: output format assertions
- Flux enrichment: mock CRs -> expected FluxStatus

### Integration Tests

Using `envtest` (in-process K8s API):
- Full pipeline with fixture resources
- Ignore rules, severity thresholds, exit codes
- Flux CRD integration with mock resources

### Security Tests

- Path traversal rejection
- YAML bomb protection
- Secret data never in output
- Malformed config rejection
- Output sanitization

### CI Pipeline

- `go vet`, `staticcheck`, `golangci-lint`
- Unit tests with `-race`
- Integration tests with envtest
- `govulncheck` for dependency scanning
- Binary builds: linux/darwin, arm64/amd64
