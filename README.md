# Driftwatch

Detect Kubernetes config drift between your Git-stored manifests and what's actually running in the cluster. Compare plain YAML, Helm charts, and Kustomize overlays against live state — with first-class FluxCD integration.

## Why Driftwatch?

If you're not fully GitOps yet (or want to verify your GitOps pipeline is working correctly), resources can silently drift from their declared state. Someone runs `kubectl edit`, a controller mutates a field, or a failed reconciliation leaves things out of sync. Driftwatch catches all of it.

## Features

- **Multi-source support** — plain manifests, Helm charts, and Kustomize overlays
- **FluxCD integration** — reads HelmRelease/Kustomization CRs, checks reconciliation status, resolves source revisions
- **Severity classification** — drifts categorized as critical (images, RBAC, ports), warning (replicas, resources), or info
- **Configurable ignore rules** — filter out Kubernetes noise (managedFields, status, annotations)
- **Secret-safe** — Secrets excluded by default, sensitive fields redacted in output
- **CI-friendly** — JSON output, threshold-based exit codes (`--fail-on=critical`)
- **Security by design** — read-only cluster access, no shell execution, YAML bomb protection, path traversal prevention

## Quick Start

### Install

```bash
go install github.com/kennyandries/driftwatch@latest
```

Or build from source:

```bash
git clone https://github.com/kennyandries/driftwatch.git
cd driftwatch
make build
```

### Scan a directory

```bash
# Scan all manifests in a directory
driftwatch scan ./k8s/

# Scan with specific kubeconfig and context
driftwatch scan ./infrastructure --kubeconfig ~/.kube/config --context production

# JSON output for CI pipelines
driftwatch scan ./apps --output json --fail-on critical

# Limit to specific namespaces
driftwatch scan ./manifests --namespace traefik,cert-manager
```

### Initialize a config file

```bash
driftwatch init
```

This creates a `driftwatch.yaml` with sensible defaults:

```yaml
sources: []
ignore:
  fields:
    - "metadata.managedFields"
    - "metadata.resourceVersion"
    - "metadata.uid"
    - "metadata.generation"
    - "metadata.creationTimestamp"
    - "status"
  resources:
    - kind: Secret
severity:
  critical:
    - "spec.containers.*.image"
    - "spec.template.spec.containers.*.image"
    - "rules"
  warning:
    - "spec.replicas"
    - "spec.template.spec.resources"
cluster:
  kubeconfig: ""
  context: ""
flux:
  enabled: true
```

### Validate config

```bash
driftwatch validate --config driftwatch.yaml
```

## Configuration

### Sources

Define which directories to scan and their types:

```yaml
sources:
  - path: ./infrastructure
    type: kustomize
  - path: ./apps
    type: kustomize
  - path: ./charts/myapp
    type: helm
```

If no sources are configured, driftwatch auto-detects source types by walking the directory:
- `Chart.yaml` present → Helm chart
- `kustomization.yaml` present → Kustomize overlay
- `.yaml`/`.yml` with `apiVersion`/`kind` → plain manifest

### Ignore Rules

Filter out Kubernetes-managed noise:

```yaml
ignore:
  fields:
    - "metadata.managedFields"
    - "metadata.annotations.kubectl.kubernetes.io/*"
    - "metadata.resourceVersion"
    - "metadata.uid"
    - "metadata.generation"
    - "metadata.creationTimestamp"
    - "status"
  resources:
    - kind: Secret
```

### Severity Classification

Control which fields trigger which severity level:

```yaml
severity:
  critical:
    - "spec.containers.*.image"
    - "spec.template.spec.containers.*.image"
    - "rules"                    # RBAC
    - "spec.ports"
  warning:
    - "spec.replicas"
    - "spec.template.spec.resources"
```

Fields not matching any rule default to `info` severity.

### FluxCD Integration

When Flux CRDs are detected in the cluster, driftwatch automatically:

1. Reads `HelmRelease` and `Kustomization` CRs
2. Resolves `GitRepository`/`HelmRepository` sources to expected revisions
3. Checks reconciliation conditions (Ready, Stalled, Failed)
4. Overlays Flux status onto drift results

```yaml
flux:
  enabled: true  # or false to disable, auto-detected by default
```

## CLI Reference

```
Usage:
  driftwatch [command]

Commands:
  scan       Scan manifests and compare against live cluster state
  init       Generate a starter driftwatch.yaml
  validate   Validate config file without scanning
  version    Print version information

Scan Flags:
  --config        Config file path (default: ./driftwatch.yaml)
  --kubeconfig    Kubeconfig path (default: ~/.kube/config)
  --context       Kubernetes context (default: current)
  --namespace     Limit to namespace(s), comma-separated
  --source-type   Force source type: manifest, helm, kustomize (default: auto)
  --output        Output format: terminal, json (default: terminal)
  --fail-on       Severity threshold for exit code: critical, warning, info (default: critical)
  --flux          Flux enrichment: auto, enabled, disabled (default: auto)
```

### Exit Codes

| Code | Meaning |
|-|-|
| 0 | No drift above threshold |
| 1 | Drift detected above threshold |
| 2 | Error |

## RBAC

Driftwatch only needs read access to the cluster. RBAC manifests are provided in `deploy/rbac/`:

- `clusterrole.yaml` — broad read-only access (`get`, `list` on all resources)
- `clusterrole-scoped.yaml` — least-privilege, scoped to specific API groups

Apply the scoped role for production:

```bash
kubectl apply -f deploy/rbac/clusterrole-scoped.yaml
kubectl create clusterrolebinding driftwatch \
  --clusterrole=driftwatch-scoped \
  --serviceaccount=default:driftwatch
```

## Security

Driftwatch is built with security as a core design principle:

- **Read-only** — only `GET` and `LIST` operations, never writes to the cluster
- **No shell execution** — Helm and Kustomize rendering done in-process via Go libraries
- **Secret-safe** — `Secret` resources excluded by default, sensitive fields (`*password*`, `*token*`, `*key*`) redacted in output
- **Input validation** — all paths sandboxed to repo root, symlinks rejected, absolute paths rejected
- **YAML safety** — 10MB file size limit, 100-level nesting depth limit, anchor bomb protection
- **Output sanitization** — control characters stripped from terminal output
- **Strict config** — unknown config keys rejected, overly broad ignore rules warned

## CI/CD Integration

### GitHub Actions

```yaml
- name: Check for drift
  run: |
    driftwatch scan ./manifests \
      --output json \
      --fail-on critical \
      --kubeconfig ${{ secrets.KUBECONFIG }}
```

### Exit code threshold

Use `--fail-on` to control when the pipeline fails:

```bash
# Fail only on critical drift (image changes, RBAC, ports)
driftwatch scan ./k8s --fail-on critical

# Fail on warnings too (replica count, resource limits)
driftwatch scan ./k8s --fail-on warning

# Fail on any drift at all
driftwatch scan ./k8s --fail-on info
```

## Architecture

```
CLI (cobra)
  -> Discovery (auto-detect source types)
    -> Rendering (manifest / kustomize / helm)
      -> Fetching (live cluster state via client-go)
        -> Diffing (field-by-field comparison)
          -> Flux Enrichment (reconciliation status)
            -> Reporting (terminal / JSON)
```

Each stage is a Go interface, independently testable. See [design document](docs/plans/2026-03-10-driftwatch-design.md) for details.

## Development

```bash
# Run tests
make test

# Run tests with race detector
go test ./... -race

# Lint
make lint

# Build
make build

# Run vulnerability check
make vulncheck
```

## License

MIT
