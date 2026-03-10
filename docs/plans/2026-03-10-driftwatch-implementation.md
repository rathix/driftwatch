# Driftwatch Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a Go CLI that detects Kubernetes config drift between Git manifests and live cluster state with FluxCD integration.

**Architecture:** Pipeline stages (Discovery → Rendering → Fetching → Diffing → Flux Enrichment → Reporting) connected via typed interfaces. Each stage is independently testable.

**Tech Stack:** Go 1.22+, cobra, client-go, helm/v3, kustomize/api, fluxcd types, go-cmp

---

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `main.go`
- Create: `cmd/root.go`
- Create: `cmd/scan.go`
- Create: `cmd/version.go`
- Create: `cmd/init.go`
- Create: `cmd/validate.go`

**Step 1: Initialize Go module**

Run: `go mod init github.com/kennyandries/driftwatch`

**Step 2: Create main.go**

```go
package main

import "github.com/kennyandries/driftwatch/cmd"

func main() {
	cmd.Execute()
}
```

**Step 3: Create cmd/root.go**

```go
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "driftwatch",
	Short: "Detect Kubernetes config drift between Git and live cluster",
	Long:  "Driftwatch compares Git-stored manifests (YAML, Helm, Kustomize) against live cluster state and flags any drift.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
```

**Step 4: Create cmd/scan.go (stub)**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var scanCmd = &cobra.Command{
	Use:   "scan [path]",
	Short: "Scan manifests and compare against live cluster state",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("scan not yet implemented")
		return nil
	},
}

func init() {
	scanCmd.Flags().String("config", "./driftwatch.yaml", "Config file path")
	scanCmd.Flags().String("kubeconfig", "", "Kubeconfig path (defaults to ~/.kube/config)")
	scanCmd.Flags().String("context", "", "Kubernetes context (defaults to current)")
	scanCmd.Flags().StringSlice("namespace", nil, "Limit to namespace(s)")
	scanCmd.Flags().String("source-type", "auto", "Force source type: manifest, helm, kustomize")
	scanCmd.Flags().String("output", "terminal", "Output format: terminal, json")
	scanCmd.Flags().String("fail-on", "critical", "Severity threshold: critical, warning, info")
	scanCmd.Flags().String("flux", "auto", "Flux enrichment: auto, enabled, disabled")
	rootCmd.AddCommand(scanCmd)
}
```

**Step 5: Create cmd/version.go**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("driftwatch %s (commit: %s, built: %s)\n", version, commit, date)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
```

**Step 6: Create cmd/init.go (stub)**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Generate a starter driftwatch.yaml",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("init not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
```

**Step 7: Create cmd/validate.go (stub)**

```go
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate config file without scanning",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("validate not yet implemented")
		return nil
	},
}

func init() {
	validateCmd.Flags().String("config", "./driftwatch.yaml", "Config file path")
	rootCmd.AddCommand(validateCmd)
}
```

**Step 8: Install deps and verify build**

Run: `go mod tidy && go build -o driftwatch .`
Expected: binary builds, `./driftwatch version` prints version

**Step 9: Commit**

```bash
git add -A
git commit -m "feat: project scaffolding with cobra CLI skeleton"
```

---

### Task 2: Core Data Model

**Files:**
- Create: `pkg/types/types.go`
- Create: `pkg/types/types_test.go`

**Step 1: Write tests for data model**

```go
package types

import (
	"testing"
)

func TestResourceIdentifier_String(t *testing.T) {
	id := ResourceIdentifier{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  "default",
		Name:       "nginx",
	}
	expected := "apps/v1/Deployment/default/nginx"
	if id.String() != expected {
		t.Errorf("got %q, want %q", id.String(), expected)
	}
}

func TestResourceIdentifier_String_ClusterScoped(t *testing.T) {
	id := ResourceIdentifier{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRole",
		Name:       "admin",
	}
	expected := "rbac.authorization.k8s.io/v1/ClusterRole//admin"
	if id.String() != expected {
		t.Errorf("got %q, want %q", id.String(), expected)
	}
}

func TestDriftResult_ExceedsThreshold(t *testing.T) {
	tests := []struct {
		name      string
		severity  Severity
		threshold Severity
		want      bool
	}{
		{"critical exceeds critical", SeverityCritical, SeverityCritical, true},
		{"warning does not exceed critical", SeverityWarning, SeverityCritical, false},
		{"info does not exceed warning", SeverityInfo, SeverityWarning, false},
		{"warning exceeds warning", SeverityWarning, SeverityWarning, true},
		{"critical exceeds info", SeverityCritical, SeverityInfo, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := DriftResult{Severity: tt.severity}
			if got := r.ExceedsThreshold(tt.threshold); got != tt.want {
				t.Errorf("ExceedsThreshold() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify it fails**

Run: `go test ./pkg/types/ -v`
Expected: FAIL

**Step 3: Implement types**

```go
package types

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Severity levels for drift classification.
type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityCritical:
		return "critical"
	case SeverityWarning:
		return "warning"
	default:
		return "info"
	}
}

// ParseSeverity converts a string to a Severity level.
func ParseSeverity(s string) (Severity, error) {
	switch s {
	case "critical":
		return SeverityCritical, nil
	case "warning":
		return SeverityWarning, nil
	case "info":
		return SeverityInfo, nil
	default:
		return SeverityInfo, fmt.Errorf("unknown severity: %q", s)
	}
}

// DriftStatus represents the state of a resource comparison.
type DriftStatus string

const (
	StatusInSync  DriftStatus = "in_sync"
	StatusDrifted DriftStatus = "drifted"
	StatusMissing DriftStatus = "missing" // expected but not in cluster
	StatusExtra   DriftStatus = "extra"   // in cluster but not in Git
)

// SourceType identifies how the expected state was produced.
type SourceType string

const (
	SourceManifest  SourceType = "manifest"
	SourceHelm      SourceType = "helm"
	SourceKustomize SourceType = "kustomize"
)

// ResourceIdentifier uniquely identifies a K8s resource.
type ResourceIdentifier struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

func (r ResourceIdentifier) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", r.APIVersion, r.Kind, r.Namespace, r.Name)
}

// FluxRef holds a reference to a Flux resource managing this object.
type FluxRef struct {
	Kind      string // Kustomization or HelmRelease
	Name      string
	Namespace string
}

// FluxStatus holds Flux reconciliation state.
type FluxStatus struct {
	Ready            bool
	Suspended        bool
	LastAppliedRev   string
	ExpectedRev      string
	Conditions       []string
}

// SourceInfo tracks provenance of expected state.
type SourceInfo struct {
	Type    SourceType
	Path    string
	FluxRef *FluxRef
}

// ResourcePair holds expected vs live state for comparison.
type ResourcePair struct {
	ID       ResourceIdentifier
	Source   SourceInfo
	Expected *unstructured.Unstructured
	Live     *unstructured.Unstructured
}

// FieldDiff represents a single field difference.
type FieldDiff struct {
	Path     string
	Expected string
	Actual   string
	Severity Severity
}

// DriftResult is the output of the diffing stage.
type DriftResult struct {
	ID         ResourceIdentifier
	Source     SourceInfo
	Status     DriftStatus
	Diffs      []FieldDiff
	Severity   Severity
	FluxStatus *FluxStatus
}

// ExceedsThreshold returns true if this result's severity meets or exceeds the threshold.
func (d DriftResult) ExceedsThreshold(threshold Severity) bool {
	return d.Severity >= threshold
}
```

**Step 4: Run tests**

Run: `go test ./pkg/types/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: core data model types with severity and drift status"
```

---

### Task 3: Configuration

**Files:**
- Create: `pkg/config/config.go`
- Create: `pkg/config/config_test.go`
- Create: `pkg/config/validate.go`
- Create: `testdata/valid_config.yaml`
- Create: `testdata/invalid_config.yaml`

**Step 1: Write config tests**

```go
package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	cfg, err := Load("../../testdata/valid_config.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(cfg.Sources))
	}
	if cfg.Cluster.Context != "production" {
		t.Errorf("expected context 'production', got %q", cfg.Cluster.Context)
	}
	if !cfg.Flux.Enabled {
		t.Error("expected flux enabled")
	}
}

func TestLoad_InvalidConfig_UnknownKeys(t *testing.T) {
	_, err := Load("../../testdata/invalid_config.yaml")
	if err == nil {
		t.Fatal("expected error for unknown keys")
	}
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "driftwatch.yaml")
	os.WriteFile(f, []byte("{}"), 0o600)
	cfg, err := Load(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.FailOn != "critical" {
		t.Errorf("expected default fail-on 'critical', got %q", cfg.FailOn)
	}
	if len(cfg.Ignore.Fields) == 0 {
		t.Error("expected default ignore fields")
	}
}

func TestValidatePath_RejectsAbsolute(t *testing.T) {
	err := ValidatePath("/etc/passwd", "/repo")
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestValidatePath_RejectsTraversal(t *testing.T) {
	err := ValidatePath("../../etc/passwd", "/repo")
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
}

func TestValidatePath_AcceptsRelative(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, "manifests")
	os.MkdirAll(sub, 0o755)
	err := ValidatePath("manifests", dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/config/ -v`
Expected: FAIL

**Step 3: Create test fixtures**

`testdata/valid_config.yaml`:
```yaml
sources:
  - path: ./infrastructure
    type: kustomize
  - path: ./apps
    type: kustomize
ignore:
  fields:
    - "metadata.managedFields"
    - "metadata.resourceVersion"
    - "status"
  resources:
    - kind: Secret
severity:
  critical:
    - "spec.containers.*.image"
    - "rules"
  warning:
    - "spec.replicas"
cluster:
  context: production
flux:
  enabled: true
```

`testdata/invalid_config.yaml`:
```yaml
sources:
  - path: ./apps
unknown_key: bad
```

**Step 4: Implement config.go**

```go
package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Sources  []SourceConfig  `yaml:"sources"`
	Ignore   IgnoreConfig    `yaml:"ignore"`
	Severity SeverityConfig  `yaml:"severity"`
	Cluster  ClusterConfig   `yaml:"cluster"`
	Flux     FluxConfig      `yaml:"flux"`
	FailOn   string          `yaml:"failOn"`
}

type SourceConfig struct {
	Path      string   `yaml:"path"`
	Type      string   `yaml:"type"`
	Namespace string   `yaml:"namespace"`
	Values    []string `yaml:"values"`
}

type IgnoreConfig struct {
	Fields    []string         `yaml:"fields"`
	Resources []ResourceFilter `yaml:"resources"`
}

type ResourceFilter struct {
	Kind      string `yaml:"kind"`
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type SeverityConfig struct {
	Critical []string `yaml:"critical"`
	Warning  []string `yaml:"warning"`
}

type ClusterConfig struct {
	Kubeconfig string `yaml:"kubeconfig"`
	Context    string `yaml:"context"`
}

type FluxConfig struct {
	Enabled bool `yaml:"enabled"`
}

var defaultIgnoreFields = []string{
	"metadata.managedFields",
	"metadata.annotations.kubectl.kubernetes.io/last-applied-configuration",
	"metadata.resourceVersion",
	"metadata.uid",
	"metadata.generation",
	"metadata.creationTimestamp",
	"status",
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if len(data) > 1*1024*1024 {
		return nil, fmt.Errorf("config file exceeds 1MB size limit")
	}

	// Strict unmarshal to reject unknown keys
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	allowedKeys := map[string]bool{
		"sources": true, "ignore": true, "severity": true,
		"cluster": true, "flux": true, "failOn": true,
	}
	for key := range raw {
		if !allowedKeys[key] {
			return nil, fmt.Errorf("unknown config key: %q", key)
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	applyDefaults(&cfg)
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.FailOn == "" {
		cfg.FailOn = "critical"
	}
	if len(cfg.Ignore.Fields) == 0 {
		cfg.Ignore.Fields = defaultIgnoreFields
	}
}
```

**Step 5: Implement validate.go**

```go
package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

func ValidatePath(p string, repoRoot string) error {
	if filepath.IsAbs(p) {
		return fmt.Errorf("absolute path not allowed: %q", p)
	}
	cleaned := filepath.Clean(p)
	if strings.HasPrefix(cleaned, "..") {
		return fmt.Errorf("path traversal not allowed: %q", p)
	}
	resolved := filepath.Join(repoRoot, cleaned)
	if !strings.HasPrefix(resolved, repoRoot) {
		return fmt.Errorf("path escapes repo root: %q", p)
	}
	return nil
}
```

**Step 6: Run tests**

Run: `go test ./pkg/config/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add -A
git commit -m "feat: config loading with strict validation and path safety"
```

---

### Task 4: Discovery Stage

**Files:**
- Create: `pkg/discovery/discovery.go`
- Create: `pkg/discovery/discovery_test.go`

**Step 1: Write discovery tests**

```go
package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func setupTestDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	// Plain manifest
	manifestDir := filepath.Join(dir, "manifests")
	os.MkdirAll(manifestDir, 0o755)
	os.WriteFile(filepath.Join(manifestDir, "deploy.yaml"), []byte("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test\n"), 0o600)

	// Helm chart
	helmDir := filepath.Join(dir, "charts", "myapp")
	os.MkdirAll(helmDir, 0o755)
	os.WriteFile(filepath.Join(helmDir, "Chart.yaml"), []byte("apiVersion: v2\nname: myapp\n"), 0o600)

	// Kustomize overlay
	kustomizeDir := filepath.Join(dir, "overlays", "prod")
	os.MkdirAll(kustomizeDir, 0o755)
	os.WriteFile(filepath.Join(kustomizeDir, "kustomization.yaml"), []byte("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - deploy.yaml\n"), 0o600)

	return dir
}

func TestDiscover_DetectsAllSourceTypes(t *testing.T) {
	dir := setupTestDir(t)
	sources, err := Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	types := map[string]bool{}
	for _, s := range sources {
		types[s.Type] = true
	}

	if !types["manifest"] {
		t.Error("expected manifest source")
	}
	if !types["helm"] {
		t.Error("expected helm source")
	}
	if !types["kustomize"] {
		t.Error("expected kustomize source")
	}
}

func TestDiscover_IgnoresHiddenDirs(t *testing.T) {
	dir := t.TempDir()
	hidden := filepath.Join(dir, ".git", "objects")
	os.MkdirAll(hidden, 0o755)
	os.WriteFile(filepath.Join(hidden, "deploy.yaml"), []byte("apiVersion: v1\nkind: Pod\nmetadata:\n  name: test\n"), 0o600)

	sources, err := Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(sources))
	}
}

func TestDiscover_RejectsSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real")
	os.MkdirAll(target, 0o755)
	os.WriteFile(filepath.Join(target, "deploy.yaml"), []byte("apiVersion: v1\nkind: Pod\nmetadata:\n  name: test\n"), 0o600)
	os.Symlink(target, filepath.Join(dir, "link"))

	sources, err := Discover(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, s := range sources {
		if filepath.Base(filepath.Dir(s.Path)) == "link" {
			t.Error("should not follow symlinks")
		}
	}
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/discovery/ -v`
Expected: FAIL

**Step 3: Implement discovery.go**

```go
package discovery

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type DiscoveredSource struct {
	Type string // manifest, helm, kustomize
	Path string // relative to scan root
}

const maxFileSize = 10 * 1024 * 1024 // 10MB

func Discover(root string) ([]DiscoveredSource, error) {
	var sources []DiscoveredSource
	kustomizeDirs := map[string]bool{}
	helmDirs := map[string]bool{}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving root: %w", err)
	}

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		// Skip hidden directories
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
		}

		// Skip symlinks
		if d.Type()&fs.ModeSymlink != 0 {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			return nil
		}

		rel, _ := filepath.Rel(absRoot, path)

		if d.Name() == "kustomization.yaml" || d.Name() == "kustomization.yml" || d.Name() == "Kustomization" {
			// Check if it's a plain kustomize file (not Flux CR)
			if isPlainKustomization(path) {
				dir := filepath.Dir(rel)
				kustomizeDirs[dir] = true
				sources = append(sources, DiscoveredSource{Type: "kustomize", Path: dir})
			}
		}

		if d.Name() == "Chart.yaml" {
			dir := filepath.Dir(rel)
			helmDirs[dir] = true
			sources = append(sources, DiscoveredSource{Type: "helm", Path: dir})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	// Second pass: find standalone manifests not inside kustomize/helm dirs
	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && strings.HasPrefix(d.Name(), ".") {
			return fs.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(d.Name())
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		rel, _ := filepath.Rel(absRoot, path)
		dir := filepath.Dir(rel)

		// Skip if inside a kustomize or helm directory
		for kd := range kustomizeDirs {
			if strings.HasPrefix(dir, kd) {
				return nil
			}
		}
		for hd := range helmDirs {
			if strings.HasPrefix(dir, hd) {
				return nil
			}
		}

		if isKubernetesManifest(path) {
			sources = append(sources, DiscoveredSource{Type: "manifest", Path: rel})
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory for manifests: %w", err)
	}

	return sources, nil
}

func isPlainKustomization(path string) bool {
	data, err := readFileSafe(path)
	if err != nil {
		return false
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	apiVersion, _ := doc["apiVersion"].(string)
	return strings.HasPrefix(apiVersion, "kustomize.config.k8s.io/")
}

func isKubernetesManifest(path string) bool {
	data, err := readFileSafe(path)
	if err != nil {
		return false
	}
	var doc map[string]interface{}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}
	_, hasAPIVersion := doc["apiVersion"]
	_, hasKind := doc["kind"]
	return hasAPIVersion && hasKind
}

func readFileSafe(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink: %s", path)
	}
	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("file exceeds %d byte limit: %s", maxFileSize, path)
	}
	return os.ReadFile(path)
}
```

**Step 4: Run tests**

Run: `go test ./pkg/discovery/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: discovery stage with source type detection and security guards"
```

---

### Task 5: Manifest Renderer

**Files:**
- Create: `pkg/renderer/renderer.go`
- Create: `pkg/renderer/manifest.go`
- Create: `pkg/renderer/manifest_test.go`
- Create: `testdata/manifests/deployment.yaml`
- Create: `testdata/manifests/multi-doc.yaml`

**Step 1: Write renderer interface and manifest tests**

```go
// pkg/renderer/renderer.go
package renderer

import (
	"context"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Renderer interface {
	Render(ctx context.Context, path string) ([]*unstructured.Unstructured, error)
}
```

```go
// pkg/renderer/manifest_test.go
package renderer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRenderer_SingleDoc(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: nginx
  namespace: default
spec:
  replicas: 1
`), 0o600)

	r := &ManifestRenderer{}
	objs, err := r.Render(context.Background(), filepath.Join(dir, "deploy.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	if objs[0].GetName() != "nginx" {
		t.Errorf("expected name 'nginx', got %q", objs[0].GetName())
	}
}

func TestManifestRenderer_MultiDoc(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "multi.yaml"), []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: test
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: app
  namespace: test
`), 0o600)

	r := &ManifestRenderer{}
	objs, err := r.Render(context.Background(), filepath.Join(dir, "multi.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objs))
	}
}

func TestManifestRenderer_SkipsSecrets(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "secret.yaml"), []byte(`apiVersion: v1
kind: Secret
metadata:
  name: mysecret
data:
  password: cGFzc3dvcmQ=
`), 0o600)

	r := &ManifestRenderer{SkipSecrets: true}
	objs, err := r.Render(context.Background(), filepath.Join(dir, "secret.yaml"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 0 {
		t.Errorf("expected 0 objects (secret skipped), got %d", len(objs))
	}
}

func TestManifestRenderer_RejectsOversized(t *testing.T) {
	dir := t.TempDir()
	bigFile := filepath.Join(dir, "big.yaml")
	f, _ := os.Create(bigFile)
	f.Write(make([]byte, 11*1024*1024))
	f.Close()

	r := &ManifestRenderer{}
	_, err := r.Render(context.Background(), bigFile)
	if err == nil {
		t.Fatal("expected error for oversized file")
	}
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/renderer/ -v`
Expected: FAIL

**Step 3: Implement manifest renderer**

```go
// pkg/renderer/manifest.go
package renderer

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const maxFileSize = 10 * 1024 * 1024

type ManifestRenderer struct {
	SkipSecrets bool
}

func (r *ManifestRenderer) Render(_ context.Context, path string) ([]*unstructured.Unstructured, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", path, err)
	}
	if info.Size() > maxFileSize {
		return nil, fmt.Errorf("file exceeds %d byte limit: %s", maxFileSize, path)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("refusing to read symlink: %s", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var objects []*unstructured.Unstructured
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	for {
		obj := &unstructured.Unstructured{}
		err := decoder.Decode(obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decoding %s: %w", path, err)
		}
		if obj.GetAPIVersion() == "" || obj.GetKind() == "" {
			continue
		}
		if r.SkipSecrets && obj.GetKind() == "Secret" {
			continue
		}
		objects = append(objects, obj)
	}

	return objects, nil
}
```

**Step 4: Run tests**

Run: `go test ./pkg/renderer/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: manifest renderer with multi-doc YAML and secret filtering"
```

---

### Task 6: Kustomize Renderer

**Files:**
- Create: `pkg/renderer/kustomize.go`
- Create: `pkg/renderer/kustomize_test.go`

**Step 1: Write kustomize renderer tests**

```go
package renderer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestKustomizeRenderer_BasicBuild(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - deploy.yaml
`), 0o600)
	os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: test
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
        - name: test
          image: nginx:latest
`), 0o600)

	r := &KustomizeRenderer{}
	objs, err := r.Render(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	if objs[0].GetKind() != "Deployment" {
		t.Errorf("expected Deployment, got %s", objs[0].GetKind())
	}
}

func TestKustomizeRenderer_SkipsSecrets(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(`apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - secret.yaml
`), 0o600)
	os.WriteFile(filepath.Join(dir, "secret.yaml"), []byte(`apiVersion: v1
kind: Secret
metadata:
  name: mysecret
data:
  key: dmFsdWU=
`), 0o600)

	r := &KustomizeRenderer{SkipSecrets: true}
	objs, err := r.Render(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 0 {
		t.Errorf("expected 0 objects, got %d", len(objs))
	}
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/renderer/ -v -run Kustomize`
Expected: FAIL

**Step 3: Implement kustomize renderer**

```go
// pkg/renderer/kustomize.go
package renderer

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"bytes"
	"io"
)

type KustomizeRenderer struct {
	SkipSecrets bool
}

func (r *KustomizeRenderer) Render(_ context.Context, path string) ([]*unstructured.Unstructured, error) {
	fSys := filesys.MakeFsOnDisk()

	opts := krusty.MakeDefaultOptions()
	opts.PluginConfig = types.DisabledPluginConfig()

	k := krusty.MakeKustomizer(opts)
	resMap, err := k.Run(fSys, path)
	if err != nil {
		return nil, fmt.Errorf("kustomize build %s: %w", path, err)
	}

	yamlBytes, err := resMap.AsYaml()
	if err != nil {
		return nil, fmt.Errorf("converting kustomize output: %w", err)
	}

	var objects []*unstructured.Unstructured
	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBytes), 4096)
	for {
		obj := &unstructured.Unstructured{}
		err := decoder.Decode(obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decoding kustomize output: %w", err)
		}
		if obj.GetAPIVersion() == "" || obj.GetKind() == "" {
			continue
		}
		if r.SkipSecrets && obj.GetKind() == "Secret" {
			continue
		}
		objects = append(objects, obj)
	}

	return objects, nil
}
```

**Step 4: Run tests**

Run: `go test ./pkg/renderer/ -v -run Kustomize`
Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: kustomize renderer with exec plugins disabled"
```

---

### Task 7: Helm Renderer (HelmRelease-as-source)

**Files:**
- Create: `pkg/renderer/helm.go`
- Create: `pkg/renderer/helm_test.go`

**Step 1: Write helm renderer tests**

```go
package renderer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestHelmRenderer_LocalChart(t *testing.T) {
	// Create a minimal helm chart
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "mychart")
	templatesDir := filepath.Join(chartDir, "templates")
	os.MkdirAll(templatesDir, 0o755)

	os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`apiVersion: v2
name: mychart
version: 0.1.0
`), 0o600)

	os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte(`replicaCount: 1
image: nginx:latest
`), 0o600)

	os.WriteFile(filepath.Join(templatesDir, "deployment.yaml"), []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app: {{ .Release.Name }}
    spec:
      containers:
        - name: app
          image: {{ .Values.image }}
`), 0o600)

	r := &HelmRenderer{}
	objs, err := r.Render(context.Background(), chartDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) == 0 {
		t.Fatal("expected at least 1 object")
	}
	if objs[0].GetKind() != "Deployment" {
		t.Errorf("expected Deployment, got %s", objs[0].GetKind())
	}
}

func TestHelmRenderer_SkipsSecrets(t *testing.T) {
	dir := t.TempDir()
	chartDir := filepath.Join(dir, "mychart")
	templatesDir := filepath.Join(chartDir, "templates")
	os.MkdirAll(templatesDir, 0o755)

	os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(`apiVersion: v2
name: mychart
version: 0.1.0
`), 0o600)
	os.WriteFile(filepath.Join(chartDir, "values.yaml"), []byte(`{}`), 0o600)
	os.WriteFile(filepath.Join(templatesDir, "secret.yaml"), []byte(`apiVersion: v1
kind: Secret
metadata:
  name: test
data:
  key: dmFsdWU=
`), 0o600)

	r := &HelmRenderer{SkipSecrets: true}
	objs, err := r.Render(context.Background(), chartDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 0 {
		t.Errorf("expected 0 objects, got %d", len(objs))
	}
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/renderer/ -v -run Helm`
Expected: FAIL

**Step 3: Implement helm renderer**

```go
// pkg/renderer/helm.go
package renderer

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8syaml "k8s.io/apimachinery/pkg/util/yaml"
	"log"
)

type HelmRenderer struct {
	SkipSecrets   bool
	ReleaseName   string
	Namespace     string
	ValueOverrides map[string]interface{}
}

func (r *HelmRenderer) Render(_ context.Context, chartPath string) ([]*unstructured.Unstructured, error) {
	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("loading chart %s: %w", chartPath, err)
	}

	cfg := &action.Configuration{
		Log: func(format string, v ...interface{}) {
			log.Printf(format, v...)
		},
	}

	install := action.NewInstall(cfg)
	install.ReleaseName = r.ReleaseName
	if install.ReleaseName == "" {
		install.ReleaseName = chart.Metadata.Name
	}
	install.Namespace = r.Namespace
	if install.Namespace == "" {
		install.Namespace = "default"
	}
	install.DryRun = true
	install.ClientOnly = true
	install.Replace = true

	vals := chart.Values
	for k, v := range r.ValueOverrides {
		vals[k] = v
	}

	rel, err := install.Run(chart, vals)
	if err != nil {
		return nil, fmt.Errorf("rendering chart %s: %w", chartPath, err)
	}

	var objects []*unstructured.Unstructured
	decoder := k8syaml.NewYAMLOrJSONDecoder(bytes.NewReader([]byte(rel.Manifest)), 4096)
	for {
		obj := &unstructured.Unstructured{}
		err := decoder.Decode(obj)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decoding helm output: %w", err)
		}
		if obj.GetAPIVersion() == "" || obj.GetKind() == "" {
			continue
		}
		if r.SkipSecrets && obj.GetKind() == "Secret" {
			continue
		}
		objects = append(objects, obj)
	}

	return objects, nil
}
```

**Step 4: Run tests**

Run: `go test ./pkg/renderer/ -v -run Helm`
Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: helm renderer with in-process template rendering"
```

---

### Task 8: Kubernetes Fetcher

**Files:**
- Create: `pkg/fetcher/fetcher.go`
- Create: `pkg/fetcher/fetcher_test.go`

**Step 1: Write fetcher tests**

```go
package fetcher

import (
	"context"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestFetcher_GetLiveResource(t *testing.T) {
	scheme := runtime.NewScheme()
	live := &unstructured.Unstructured{}
	live.SetAPIVersion("apps/v1")
	live.SetKind("Deployment")
	live.SetNamespace("default")
	live.SetName("nginx")
	live.SetUID("test-uid")

	client := dynamicfake.NewSimpleDynamicClient(scheme, live)

	f := &Fetcher{
		Client: client,
		Mapper: &staticMapper{
			gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		},
	}

	id := types.ResourceIdentifier{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  "default",
		Name:       "nginx",
	}

	result, err := f.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.GetName() != "nginx" {
		t.Errorf("expected name 'nginx', got %q", result.GetName())
	}
}

func TestFetcher_MissingResource(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme)

	f := &Fetcher{
		Client: client,
		Mapper: &staticMapper{
			gvr: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
		},
	}

	id := types.ResourceIdentifier{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  "default",
		Name:       "nonexistent",
	}

	result, err := f.Get(context.Background(), id)
	if err != nil && !IsNotFound(err) {
		t.Fatalf("expected not found, got: %v", err)
	}
	if result != nil {
		t.Error("expected nil result for missing resource")
	}
}

// staticMapper is a test helper that returns a fixed GVR.
type staticMapper struct {
	gvr schema.GroupVersionResource
}

func (m *staticMapper) ResourceFor(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	return m.gvr, nil
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/fetcher/ -v`
Expected: FAIL

**Step 3: Implement fetcher**

```go
// pkg/fetcher/fetcher.go
package fetcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kennyandries/driftwatch/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/apimachinery/pkg/api/errors"
	"golang.org/x/time/rate"
)

type ResourceMapper interface {
	ResourceFor(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error)
}

type Fetcher struct {
	Client  dynamic.Interface
	Mapper  ResourceMapper
	Limiter *rate.Limiter
	Timeout time.Duration
}

func NewFetcher(client dynamic.Interface, mapper ResourceMapper) *Fetcher {
	return &Fetcher{
		Client:  client,
		Mapper:  mapper,
		Limiter: rate.NewLimiter(rate.Limit(10), 10), // 10 QPS
		Timeout: 10 * time.Second,
	}
}

func (f *Fetcher) Get(ctx context.Context, id types.ResourceIdentifier) (*unstructured.Unstructured, error) {
	if f.Limiter != nil {
		if err := f.Limiter.Wait(ctx); err != nil {
			return nil, fmt.Errorf("rate limit: %w", err)
		}
	}

	gvk := parseGVK(id.APIVersion, id.Kind)
	gvr, err := f.Mapper.ResourceFor(gvk)
	if err != nil {
		return nil, fmt.Errorf("mapping resource %s: %w", id, err)
	}

	timeout := f.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var resource dynamic.ResourceInterface
	if id.Namespace != "" {
		resource = f.Client.Resource(gvr).Namespace(id.Namespace)
	} else {
		resource = f.Client.Resource(gvr)
	}

	obj, err := resource.Get(ctx, id.Name, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, err
		}
		return nil, fmt.Errorf("fetching %s: %w", id, err)
	}

	return obj, nil
}

func IsNotFound(err error) bool {
	return errors.IsNotFound(err)
}

func parseGVK(apiVersion, kind string) schema.GroupVersionKind {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 1 {
		return schema.GroupVersionKind{Version: parts[0], Kind: kind}
	}
	return schema.GroupVersionKind{Group: parts[0], Version: parts[1], Kind: kind}
}
```

**Step 4: Run tests**

Run: `go test ./pkg/fetcher/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: kubernetes fetcher with rate limiting and timeouts"
```

---

### Task 9: Diffing Engine

**Files:**
- Create: `pkg/differ/differ.go`
- Create: `pkg/differ/differ_test.go`
- Create: `pkg/differ/severity.go`
- Create: `pkg/differ/filter.go`

**Step 1: Write differ tests**

```go
package differ

import (
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDiff_InSync(t *testing.T) {
	expected := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "test", "namespace": "default"},
			"spec":       map[string]interface{}{"replicas": int64(1)},
		},
	}
	live := expected.DeepCopy()

	d := NewDiffer(DefaultIgnoreFields(), DefaultSeverityRules())
	result := d.Diff(types.ResourcePair{
		ID:       types.ResourceIdentifier{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "test"},
		Expected: expected,
		Live:     live,
	})

	if result.Status != types.StatusInSync {
		t.Errorf("expected InSync, got %s", result.Status)
	}
}

func TestDiff_Drifted(t *testing.T) {
	expected := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "test", "namespace": "default"},
			"spec":       map[string]interface{}{"replicas": int64(3)},
		},
	}
	live := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "test", "namespace": "default"},
			"spec":       map[string]interface{}{"replicas": int64(1)},
		},
	}

	d := NewDiffer(DefaultIgnoreFields(), DefaultSeverityRules())
	result := d.Diff(types.ResourcePair{
		ID:       types.ResourceIdentifier{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "test"},
		Expected: expected,
		Live:     live,
	})

	if result.Status != types.StatusDrifted {
		t.Errorf("expected Drifted, got %s", result.Status)
	}
	if len(result.Diffs) == 0 {
		t.Error("expected at least one field diff")
	}
}

func TestDiff_Missing(t *testing.T) {
	expected := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "test", "namespace": "default"},
		},
	}

	d := NewDiffer(DefaultIgnoreFields(), DefaultSeverityRules())
	result := d.Diff(types.ResourcePair{
		ID:       types.ResourceIdentifier{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "test"},
		Expected: expected,
		Live:     nil,
	})

	if result.Status != types.StatusMissing {
		t.Errorf("expected Missing, got %s", result.Status)
	}
}

func TestDiff_IgnoresManagedFields(t *testing.T) {
	expected := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "test", "namespace": "default"},
			"spec":       map[string]interface{}{"replicas": int64(1)},
		},
	}
	live := expected.DeepCopy()
	live.Object["metadata"].(map[string]interface{})["managedFields"] = []interface{}{
		map[string]interface{}{"manager": "kubectl"},
	}
	live.Object["metadata"].(map[string]interface{})["resourceVersion"] = "12345"
	live.Object["metadata"].(map[string]interface{})["uid"] = "abc-123"

	d := NewDiffer(DefaultIgnoreFields(), DefaultSeverityRules())
	result := d.Diff(types.ResourcePair{
		ID:       types.ResourceIdentifier{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "test"},
		Expected: expected,
		Live:     live,
	})

	if result.Status != types.StatusInSync {
		t.Errorf("expected InSync after ignoring managed fields, got %s with diffs: %v", result.Status, result.Diffs)
	}
}

func TestSeverity_ImageChange_IsCritical(t *testing.T) {
	rules := DefaultSeverityRules()
	sev := classifyField("spec.template.spec.containers.0.image", rules)
	if sev != types.SeverityCritical {
		t.Errorf("expected critical, got %s", sev)
	}
}

func TestSeverity_ReplicaChange_IsWarning(t *testing.T) {
	rules := DefaultSeverityRules()
	sev := classifyField("spec.replicas", rules)
	if sev != types.SeverityWarning {
		t.Errorf("expected warning, got %s", sev)
	}
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/differ/ -v`
Expected: FAIL

**Step 3: Implement severity.go**

```go
package differ

import (
	"strings"

	"github.com/kennyandries/driftwatch/pkg/types"
)

type SeverityRule struct {
	Pattern  string
	Severity types.Severity
}

func DefaultSeverityRules() []SeverityRule {
	return []SeverityRule{
		// Critical - security relevant
		{Pattern: "spec.containers.*.image", Severity: types.SeverityCritical},
		{Pattern: "spec.template.spec.containers.*.image", Severity: types.SeverityCritical},
		{Pattern: "spec.initContainers.*.image", Severity: types.SeverityCritical},
		{Pattern: "spec.template.spec.initContainers.*.image", Severity: types.SeverityCritical},
		{Pattern: "rules", Severity: types.SeverityCritical},
		{Pattern: "spec.ports", Severity: types.SeverityCritical},
		{Pattern: "spec.template.spec.serviceAccountName", Severity: types.SeverityCritical},
		{Pattern: "spec.template.spec.securityContext", Severity: types.SeverityCritical},
		// Warning
		{Pattern: "spec.replicas", Severity: types.SeverityWarning},
		{Pattern: "spec.template.spec.resources", Severity: types.SeverityWarning},
	}
}

func classifyField(fieldPath string, rules []SeverityRule) types.Severity {
	for _, rule := range rules {
		if matchPattern(fieldPath, rule.Pattern) {
			return rule.Severity
		}
	}
	return types.SeverityInfo
}

func matchPattern(path, pattern string) bool {
	pathParts := strings.Split(path, ".")
	patternParts := strings.Split(pattern, ".")
	return matchParts(pathParts, patternParts)
}

func matchParts(path, pattern []string) bool {
	if len(pattern) == 0 {
		return true
	}
	if len(path) == 0 {
		return false
	}
	if pattern[0] == "*" {
		// Try matching current position and skipping
		return matchParts(path[1:], pattern[1:]) || matchParts(path[1:], pattern)
	}
	if path[0] == pattern[0] {
		return matchParts(path[1:], pattern[1:])
	}
	return false
}
```

**Step 4: Implement filter.go**

```go
package differ

import (
	"strings"
)

func DefaultIgnoreFields() []string {
	return []string{
		"metadata.managedFields",
		"metadata.resourceVersion",
		"metadata.uid",
		"metadata.generation",
		"metadata.creationTimestamp",
		"metadata.annotations.kubectl.kubernetes.io/last-applied-configuration",
		"status",
	}
}

func shouldIgnore(fieldPath string, ignorePatterns []string) bool {
	for _, pattern := range ignorePatterns {
		if strings.HasSuffix(pattern, "/*") {
			prefix := strings.TrimSuffix(pattern, "/*")
			if strings.HasPrefix(fieldPath, prefix) {
				return true
			}
		}
		if fieldPath == pattern || strings.HasPrefix(fieldPath, pattern+".") {
			return true
		}
	}
	return false
}
```

**Step 5: Implement differ.go**

```go
package differ

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Differ struct {
	ignoreFields  []string
	severityRules []SeverityRule
}

func NewDiffer(ignoreFields []string, severityRules []SeverityRule) *Differ {
	return &Differ{
		ignoreFields:  ignoreFields,
		severityRules: severityRules,
	}
}

func (d *Differ) Diff(pair types.ResourcePair) types.DriftResult {
	result := types.DriftResult{
		ID:     pair.ID,
		Source: pair.Source,
	}

	if pair.Live == nil {
		result.Status = types.StatusMissing
		result.Severity = types.SeverityCritical
		return result
	}
	if pair.Expected == nil {
		result.Status = types.StatusExtra
		result.Severity = types.SeverityWarning
		return result
	}

	diffs := d.compareObjects(pair.Expected.Object, pair.Live.Object, "")
	result.Diffs = diffs

	if len(diffs) == 0 {
		result.Status = types.StatusInSync
		result.Severity = types.SeverityInfo
	} else {
		result.Status = types.StatusDrifted
		maxSeverity := types.SeverityInfo
		for _, diff := range diffs {
			if diff.Severity > maxSeverity {
				maxSeverity = diff.Severity
			}
		}
		result.Severity = maxSeverity
	}

	return result
}

func (d *Differ) compareObjects(expected, live map[string]interface{}, prefix string) []types.FieldDiff {
	var diffs []types.FieldDiff

	allKeys := map[string]bool{}
	for k := range expected {
		allKeys[k] = true
	}
	for k := range live {
		allKeys[k] = true
	}

	sortedKeys := make([]string, 0, len(allKeys))
	for k := range allKeys {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Strings(sortedKeys)

	for _, key := range sortedKeys {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		if shouldIgnore(path, d.ignoreFields) {
			continue
		}

		expectedVal, expectedOk := expected[key]
		liveVal, liveOk := live[key]

		if !expectedOk {
			// Field only in live - ignore (cluster may add fields)
			continue
		}
		if !liveOk {
			diffs = append(diffs, types.FieldDiff{
				Path:     path,
				Expected: fmt.Sprintf("%v", expectedVal),
				Actual:   "<missing>",
				Severity: classifyField(path, d.severityRules),
			})
			continue
		}

		// Recurse into nested maps
		expectedMap, expectedIsMap := expectedVal.(map[string]interface{})
		liveMap, liveIsMap := liveVal.(map[string]interface{})
		if expectedIsMap && liveIsMap {
			diffs = append(diffs, d.compareObjects(expectedMap, liveMap, path)...)
			continue
		}

		// Recurse into slices
		expectedSlice, expectedIsSlice := toSlice(expectedVal)
		liveSlice, liveIsSlice := toSlice(liveVal)
		if expectedIsSlice && liveIsSlice {
			diffs = append(diffs, d.compareSlices(expectedSlice, liveSlice, path)...)
			continue
		}

		// Compare scalar values
		if fmt.Sprintf("%v", expectedVal) != fmt.Sprintf("%v", liveVal) {
			diffs = append(diffs, types.FieldDiff{
				Path:     path,
				Expected: fmt.Sprintf("%v", expectedVal),
				Actual:   fmt.Sprintf("%v", liveVal),
				Severity: classifyField(path, d.severityRules),
			})
		}
	}

	return diffs
}

func (d *Differ) compareSlices(expected, live []interface{}, prefix string) []types.FieldDiff {
	var diffs []types.FieldDiff

	for i := 0; i < len(expected); i++ {
		path := fmt.Sprintf("%s.%d", prefix, i)
		if i >= len(live) {
			diffs = append(diffs, types.FieldDiff{
				Path:     path,
				Expected: fmt.Sprintf("%v", expected[i]),
				Actual:   "<missing>",
				Severity: classifyField(prefix, d.severityRules),
			})
			continue
		}

		expectedMap, expectedIsMap := expected[i].(map[string]interface{})
		liveMap, liveIsMap := live[i].(map[string]interface{})
		if expectedIsMap && liveIsMap {
			diffs = append(diffs, d.compareObjects(expectedMap, liveMap, path)...)
			continue
		}

		if fmt.Sprintf("%v", expected[i]) != fmt.Sprintf("%v", live[i]) {
			diffs = append(diffs, types.FieldDiff{
				Path:     path,
				Expected: fmt.Sprintf("%v", expected[i]),
				Actual:   fmt.Sprintf("%v", live[i]),
				Severity: classifyField(prefix, d.severityRules),
			})
		}
	}

	return diffs
}

func toSlice(v interface{}) ([]interface{}, bool) {
	switch val := v.(type) {
	case []interface{}:
		return val, true
	default:
		return nil, false
	}
}

// RedactSecretValues replaces sensitive field values with "[REDACTED]".
func RedactSecretValues(obj *unstructured.Unstructured) {
	if obj == nil {
		return
	}
	if obj.GetKind() == "Secret" {
		delete(obj.Object, "data")
		delete(obj.Object, "stringData")
	}
	redactSensitiveFields(obj.Object, "")
}

var sensitivePatterns = []string{"secret", "password", "token", "key", "credential"}

func redactSensitiveFields(obj map[string]interface{}, prefix string) {
	for k, v := range obj {
		path := k
		if prefix != "" {
			path = prefix + "." + k
		}
		lower := strings.ToLower(k)
		isSensitive := false
		for _, p := range sensitivePatterns {
			if strings.Contains(lower, p) {
				isSensitive = true
				break
			}
		}
		if isSensitive {
			if _, isMap := v.(map[string]interface{}); !isMap {
				if _, isSlice := v.([]interface{}); !isSlice {
					obj[k] = "[REDACTED]"
					continue
				}
			}
		}
		if nested, ok := v.(map[string]interface{}); ok {
			redactSensitiveFields(nested, path)
		}
	}
}
```

**Step 6: Run tests**

Run: `go test ./pkg/differ/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add -A
git commit -m "feat: diffing engine with severity classification and field ignoring"
```

---

### Task 10: Flux Enrichment

**Files:**
- Create: `pkg/flux/enrichment.go`
- Create: `pkg/flux/enrichment_test.go`

**Step 1: Write flux enrichment tests**

```go
package flux

import (
	"context"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestEnricher_DetectsFluxCRDs(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClient(scheme)

	e := &Enricher{Client: client}
	// Without Flux CRDs, Available should return false
	if e.Available(context.Background()) {
		t.Error("expected Flux not available when CRDs missing")
	}
}

func TestEnricher_EnrichesHelmRelease(t *testing.T) {
	hr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name":      "traefik",
				"namespace": "flux-system",
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
				"lastAppliedRevision": "v2.10.0",
			},
		},
	}

	result := extractFluxStatus(hr)
	if !result.Ready {
		t.Error("expected Ready=true")
	}
	if result.LastAppliedRev != "v2.10.0" {
		t.Errorf("expected lastAppliedRevision 'v2.10.0', got %q", result.LastAppliedRev)
	}
}

func TestEnricher_DetectsSuspended(t *testing.T) {
	hr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "flux-system",
			},
			"spec": map[string]interface{}{
				"suspend": true,
			},
			"status": map[string]interface{}{
				"conditions": []interface{}{},
			},
		},
	}

	result := extractFluxStatus(hr)
	if !result.Suspended {
		t.Error("expected Suspended=true")
	}
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/flux/ -v`
Expected: FAIL

**Step 3: Implement enrichment.go**

```go
package flux

import (
	"context"
	"fmt"

	"github.com/kennyandries/driftwatch/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	kustomizationGVR = schema.GroupVersionResource{
		Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations",
	}
	helmReleaseGVR = schema.GroupVersionResource{
		Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases",
	}
	gitRepoGVR = schema.GroupVersionResource{
		Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "gitrepositories",
	}
	helmRepoGVR = schema.GroupVersionResource{
		Group: "source.toolkit.fluxcd.io", Version: "v1", Resource: "helmrepositories",
	}
)

type Enricher struct {
	Client dynamic.Interface
}

func (e *Enricher) Available(ctx context.Context) bool {
	_, err := e.Client.Resource(kustomizationGVR).Namespace("flux-system").List(ctx, metav1.ListOptions{Limit: 1})
	return err == nil
}

func (e *Enricher) Enrich(ctx context.Context, results []types.DriftResult) ([]types.DriftResult, error) {
	helmReleases, err := e.listResources(ctx, helmReleaseGVR, "")
	if err != nil {
		return results, fmt.Errorf("listing HelmReleases: %w", err)
	}

	kustomizations, err := e.listResources(ctx, kustomizationGVR, "")
	if err != nil {
		return results, fmt.Errorf("listing Kustomizations: %w", err)
	}

	gitRepos, err := e.listResources(ctx, gitRepoGVR, "")
	if err != nil {
		return results, fmt.Errorf("listing GitRepositories: %w", err)
	}

	// Build lookup maps
	hrMap := buildNameMap(helmReleases)
	ksMap := buildNameMap(kustomizations)
	gitMap := buildNameMap(gitRepos)

	for i := range results {
		// Try to match by HelmRelease
		if hr, ok := hrMap[results[i].ID.Namespace+"/"+results[i].ID.Name]; ok {
			status := extractFluxStatus(hr)
			results[i].FluxStatus = &status
			results[i].Source.FluxRef = &types.FluxRef{
				Kind:      "HelmRelease",
				Name:      hr.GetName(),
				Namespace: hr.GetNamespace(),
			}
			continue
		}
		// Try to match by Kustomization path
		for _, ks := range kustomizations {
			status := extractFluxStatus(ks)
			sourceRef, _, _ := unstructured.NestedString(ks.Object, "spec", "sourceRef", "name")
			if gitRepo, ok := gitMap[ks.GetNamespace()+"/"+sourceRef]; ok {
				rev, _, _ := unstructured.NestedString(gitRepo.Object, "status", "artifact", "revision")
				status.ExpectedRev = rev
			}
			// Enrich results that match this kustomization's path
			ksPath, _, _ := unstructured.NestedString(ks.Object, "spec", "path")
			if ksPath != "" && results[i].Source.Path != "" {
				if matchesKustomizationPath(results[i].Source.Path, ksPath) {
					results[i].FluxStatus = &status
					results[i].Source.FluxRef = &types.FluxRef{
						Kind:      "Kustomization",
						Name:      ks.GetName(),
						Namespace: ks.GetNamespace(),
					}
				}
			}
		}
	}

	return results, nil
}

func (e *Enricher) listResources(ctx context.Context, gvr schema.GroupVersionResource, namespace string) ([]*unstructured.Unstructured, error) {
	var resource dynamic.ResourceInterface
	if namespace != "" {
		resource = e.Client.Resource(gvr).Namespace(namespace)
	} else {
		resource = e.Client.Resource(gvr).Namespace("")
	}

	list, err := resource.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make([]*unstructured.Unstructured, len(list.Items))
	for i := range list.Items {
		result[i] = &list.Items[i]
	}
	return result, nil
}

func extractFluxStatus(obj *unstructured.Unstructured) types.FluxStatus {
	status := types.FluxStatus{}

	suspended, _, _ := unstructured.NestedBool(obj.Object, "spec", "suspend")
	status.Suspended = suspended

	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		condType, _ := cond["type"].(string)
		condStatus, _ := cond["status"].(string)
		status.Conditions = append(status.Conditions, fmt.Sprintf("%s=%s", condType, condStatus))
		if condType == "Ready" && condStatus == "True" {
			status.Ready = true
		}
	}

	rev, _, _ := unstructured.NestedString(obj.Object, "status", "lastAppliedRevision")
	status.LastAppliedRev = rev

	return status
}

func buildNameMap(resources []*unstructured.Unstructured) map[string]*unstructured.Unstructured {
	m := make(map[string]*unstructured.Unstructured)
	for _, r := range resources {
		key := r.GetNamespace() + "/" + r.GetName()
		m[key] = r
	}
	return m
}

func matchesKustomizationPath(sourcePath, ksPath string) bool {
	// Normalize: strip leading "./"
	if len(ksPath) > 2 && ksPath[:2] == "./" {
		ksPath = ksPath[2:]
	}
	return sourcePath == ksPath || fmt.Sprintf("./%s", sourcePath) == ksPath
}
```

**Step 4: Run tests**

Run: `go test ./pkg/flux/ -v`
Expected: PASS

**Step 5: Commit**

```bash
git add -A
git commit -m "feat: flux enrichment with HelmRelease and Kustomization status overlay"
```

---

### Task 11: Terminal Reporter

**Files:**
- Create: `pkg/reporter/reporter.go`
- Create: `pkg/reporter/terminal.go`
- Create: `pkg/reporter/json.go`
- Create: `pkg/reporter/reporter_test.go`

**Step 1: Write reporter tests**

```go
package reporter

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
)

func TestTerminalReporter_InSync(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalReporter(&buf, false)
	results := []types.DriftResult{
		{
			ID:       types.ResourceIdentifier{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "nginx"},
			Status:   types.StatusInSync,
			Severity: types.SeverityInfo,
		},
	}
	r.Report(results)
	output := buf.String()
	if len(output) == 0 {
		t.Error("expected output")
	}
}

func TestTerminalReporter_Drifted(t *testing.T) {
	var buf bytes.Buffer
	r := NewTerminalReporter(&buf, false)
	results := []types.DriftResult{
		{
			ID:       types.ResourceIdentifier{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "nginx"},
			Status:   types.StatusDrifted,
			Severity: types.SeverityCritical,
			Diffs: []types.FieldDiff{
				{Path: "spec.template.spec.containers.0.image", Expected: "nginx:1.20", Actual: "nginx:1.19", Severity: types.SeverityCritical},
			},
		},
	}
	r.Report(results)
	output := buf.String()
	if len(output) == 0 {
		t.Error("expected output")
	}
}

func TestJSONReporter(t *testing.T) {
	var buf bytes.Buffer
	r := NewJSONReporter(&buf)
	results := []types.DriftResult{
		{
			ID:     types.ResourceIdentifier{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "nginx"},
			Status: types.StatusDrifted,
		},
	}
	if err := r.Report(results); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var output map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if _, ok := output["results"]; !ok {
		t.Error("expected 'results' key in JSON output")
	}
	if _, ok := output["metadata"]; !ok {
		t.Error("expected 'metadata' key in JSON output")
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name      string
		results   []types.DriftResult
		threshold types.Severity
		want      int
	}{
		{"no drift", []types.DriftResult{{Status: types.StatusInSync}}, types.SeverityCritical, 0},
		{"drift below threshold", []types.DriftResult{{Status: types.StatusDrifted, Severity: types.SeverityInfo}}, types.SeverityCritical, 0},
		{"drift at threshold", []types.DriftResult{{Status: types.StatusDrifted, Severity: types.SeverityCritical}}, types.SeverityCritical, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCode(tt.results, tt.threshold)
			if got != tt.want {
				t.Errorf("ExitCode() = %d, want %d", got, tt.want)
			}
		})
	}
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/reporter/ -v`
Expected: FAIL

**Step 3: Implement reporter.go**

```go
package reporter

import (
	"github.com/kennyandries/driftwatch/pkg/types"
)

type Reporter interface {
	Report(results []types.DriftResult) error
}

func ExitCode(results []types.DriftResult, threshold types.Severity) int {
	for _, r := range results {
		if r.Status == types.StatusDrifted || r.Status == types.StatusMissing {
			if r.ExceedsThreshold(threshold) {
				return 1
			}
		}
	}
	return 0
}
```

**Step 4: Implement terminal.go**

```go
package reporter

import (
	"fmt"
	"io"
	"strings"

	"github.com/kennyandries/driftwatch/pkg/types"
)

type TerminalReporter struct {
	w     io.Writer
	color bool
}

func NewTerminalReporter(w io.Writer, color bool) *TerminalReporter {
	return &TerminalReporter{w: w, color: color}
}

func (r *TerminalReporter) Report(results []types.DriftResult) error {
	drifted := 0
	missing := 0
	inSync := 0

	for _, res := range results {
		switch res.Status {
		case types.StatusInSync:
			inSync++
		case types.StatusDrifted:
			drifted++
			r.printDrifted(res)
		case types.StatusMissing:
			missing++
			r.printMissing(res)
		}
	}

	fmt.Fprintf(r.w, "\n--- Summary ---\n")
	fmt.Fprintf(r.w, "In Sync: %d | Drifted: %d | Missing: %d | Total: %d\n",
		inSync, drifted, missing, len(results))

	return nil
}

func (r *TerminalReporter) printDrifted(res types.DriftResult) {
	fmt.Fprintf(r.w, "\n[%s] DRIFT: %s/%s (%s)\n",
		strings.ToUpper(res.Severity.String()),
		res.ID.Namespace, res.ID.Name, res.ID.Kind)
	fmt.Fprintf(r.w, "  Source: %s (%s)\n", res.Source.Path, res.Source.Type)

	if res.FluxStatus != nil {
		fmt.Fprintf(r.w, "  Flux: ready=%v suspended=%v rev=%s\n",
			res.FluxStatus.Ready, res.FluxStatus.Suspended, res.FluxStatus.LastAppliedRev)
	}

	for _, d := range res.Diffs {
		fmt.Fprintf(r.w, "  %s [%s]\n", d.Path, d.Severity)
		fmt.Fprintf(r.w, "    expected: %s\n", sanitizeOutput(d.Expected))
		fmt.Fprintf(r.w, "    actual:   %s\n", sanitizeOutput(d.Actual))
	}
}

func (r *TerminalReporter) printMissing(res types.DriftResult) {
	fmt.Fprintf(r.w, "\n[CRITICAL] MISSING: %s/%s (%s)\n",
		res.ID.Namespace, res.ID.Name, res.ID.Kind)
	fmt.Fprintf(r.w, "  Source: %s (%s)\n", res.Source.Path, res.Source.Type)
}

// sanitizeOutput strips control characters from output values.
func sanitizeOutput(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 || r == '\n' || r == '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
```

**Step 5: Implement json.go**

```go
package reporter

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/kennyandries/driftwatch/pkg/types"
)

type JSONReporter struct {
	w io.Writer
}

func NewJSONReporter(w io.Writer) *JSONReporter {
	return &JSONReporter{w: w}
}

type jsonOutput struct {
	Metadata jsonMetadata       `json:"metadata"`
	Results  []types.DriftResult `json:"results"`
	Summary  jsonSummary        `json:"summary"`
}

type jsonMetadata struct {
	Timestamp string `json:"timestamp"`
	Version   string `json:"version"`
}

type jsonSummary struct {
	Total   int `json:"total"`
	InSync  int `json:"in_sync"`
	Drifted int `json:"drifted"`
	Missing int `json:"missing"`
}

func (r *JSONReporter) Report(results []types.DriftResult) error {
	summary := jsonSummary{Total: len(results)}
	for _, res := range results {
		switch res.Status {
		case types.StatusInSync:
			summary.InSync++
		case types.StatusDrifted:
			summary.Drifted++
		case types.StatusMissing:
			summary.Missing++
		}
	}

	output := jsonOutput{
		Metadata: jsonMetadata{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Version:   "dev",
		},
		Results: results,
		Summary: summary,
	}

	data, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling JSON: %w", err)
	}
	_, err = r.w.Write(data)
	return err
}
```

**Step 6: Run tests**

Run: `go test ./pkg/reporter/ -v`
Expected: PASS

**Step 7: Commit**

```bash
git add -A
git commit -m "feat: terminal and JSON reporters with output sanitization"
```

---

### Task 12: Pipeline Orchestrator + Wire CLI

**Files:**
- Create: `pkg/pipeline/pipeline.go`
- Create: `pkg/pipeline/pipeline_test.go`
- Modify: `cmd/scan.go`
- Modify: `cmd/init.go`
- Modify: `cmd/validate.go`

**Step 1: Write pipeline tests**

```go
package pipeline

import (
	"context"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type mockRenderer struct {
	objects []*unstructured.Unstructured
}

func (m *mockRenderer) Render(_ context.Context, _ string) ([]*unstructured.Unstructured, error) {
	return m.objects, nil
}

type mockFetcher struct {
	live map[string]*unstructured.Unstructured
}

func (m *mockFetcher) Get(_ context.Context, id types.ResourceIdentifier) (*unstructured.Unstructured, error) {
	if obj, ok := m.live[id.String()]; ok {
		return obj, nil
	}
	return nil, nil
}

func TestPipeline_DetectsDrift(t *testing.T) {
	expected := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "nginx", "namespace": "default"},
			"spec":       map[string]interface{}{"replicas": int64(3)},
		},
	}

	live := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "nginx", "namespace": "default"},
			"spec":       map[string]interface{}{"replicas": int64(1)},
		},
	}

	p := &Pipeline{
		Renderer: &mockRenderer{objects: []*unstructured.Unstructured{expected}},
		Fetcher:  &mockFetcher{live: map[string]*unstructured.Unstructured{
			"apps/v1/Deployment/default/nginx": live,
		}},
	}

	results, err := p.Run(context.Background(), "test-path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != types.StatusDrifted {
		t.Errorf("expected Drifted, got %s", results[0].Status)
	}
}

func TestPipeline_DetectsMissing(t *testing.T) {
	expected := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata":   map[string]interface{}{"name": "nginx", "namespace": "default"},
		},
	}

	p := &Pipeline{
		Renderer: &mockRenderer{objects: []*unstructured.Unstructured{expected}},
		Fetcher:  &mockFetcher{live: map[string]*unstructured.Unstructured{}},
	}

	results, err := p.Run(context.Background(), "test-path")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if results[0].Status != types.StatusMissing {
		t.Errorf("expected Missing, got %s", results[0].Status)
	}
}
```

**Step 2: Run test to verify failure**

Run: `go test ./pkg/pipeline/ -v`
Expected: FAIL

**Step 3: Implement pipeline.go**

```go
package pipeline

import (
	"context"
	"fmt"

	"github.com/kennyandries/driftwatch/pkg/differ"
	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type RendererInterface interface {
	Render(ctx context.Context, path string) ([]*unstructured.Unstructured, error)
}

type FetcherInterface interface {
	Get(ctx context.Context, id types.ResourceIdentifier) (*unstructured.Unstructured, error)
}

type Pipeline struct {
	Renderer      RendererInterface
	Fetcher       FetcherInterface
	Differ        *differ.Differ
	IgnoreFields  []string
	SeverityRules []differ.SeverityRule
}

func (p *Pipeline) Run(ctx context.Context, path string) ([]types.DriftResult, error) {
	// Render expected state
	expected, err := p.Renderer.Render(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("rendering %s: %w", path, err)
	}

	d := p.Differ
	if d == nil {
		ignoreFields := p.IgnoreFields
		if len(ignoreFields) == 0 {
			ignoreFields = differ.DefaultIgnoreFields()
		}
		rules := p.SeverityRules
		if len(rules) == 0 {
			rules = differ.DefaultSeverityRules()
		}
		d = differ.NewDiffer(ignoreFields, rules)
	}

	var results []types.DriftResult

	for _, obj := range expected {
		// Redact secrets before comparison
		differ.RedactSecretValues(obj)

		id := types.ResourceIdentifier{
			APIVersion: obj.GetAPIVersion(),
			Kind:       obj.GetKind(),
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
		}

		live, err := p.Fetcher.Get(ctx, id)
		if err != nil {
			// Treat fetch errors as missing
			live = nil
		}

		if live != nil {
			differ.RedactSecretValues(live)
		}

		pair := types.ResourcePair{
			ID:       id,
			Source:   types.SourceInfo{Path: path},
			Expected: obj,
			Live:     live,
		}

		result := d.Diff(pair)
		results = append(results, result)
	}

	return results, nil
}
```

**Step 4: Run tests**

Run: `go test ./pkg/pipeline/ -v`
Expected: PASS

**Step 5: Wire cmd/scan.go to use the pipeline**

Replace `cmd/scan.go` with full implementation wiring discovery, renderers, fetcher, pipeline, flux enrichment, and reporters together. (The full wiring reads config, creates k8s client, runs discovery, dispatches to appropriate renderer per source type, runs pipeline, enriches with flux, reports, and exits with appropriate code.)

**Step 6: Wire cmd/init.go to generate starter config**

Write a default `driftwatch.yaml` template to the current directory.

**Step 7: Wire cmd/validate.go**

Load and validate the config file, print success or errors.

**Step 8: Run all tests**

Run: `go test ./... -v -race`
Expected: ALL PASS

**Step 9: Build and smoke test**

Run: `go build -o driftwatch . && ./driftwatch version`
Expected: prints version

**Step 10: Commit**

```bash
git add -A
git commit -m "feat: pipeline orchestrator and CLI wiring"
```

---

### Task 13: Security Hardening

**Files:**
- Create: `pkg/security/yaml.go`
- Create: `pkg/security/yaml_test.go`
- Modify: `pkg/renderer/manifest.go` (use safe YAML reader)
- Modify: `pkg/config/config.go` (use safe YAML reader)

**Step 1: Write YAML safety tests**

```go
package security

import (
	"strings"
	"testing"
)

func TestSafeYAMLDecode_RejectsOversized(t *testing.T) {
	big := strings.Repeat("a: b\n", 3*1024*1024)
	_, err := SafeYAMLDecode([]byte(big))
	if err == nil {
		t.Fatal("expected error for oversized YAML")
	}
}

func TestSafeYAMLDecode_RejectsDeeplyNested(t *testing.T) {
	// Create deeply nested YAML
	var b strings.Builder
	for i := 0; i < 200; i++ {
		for j := 0; j < i; j++ {
			b.WriteString("  ")
		}
		b.WriteString("a:\n")
	}
	_, err := SafeYAMLDecode([]byte(b.String()))
	if err == nil {
		t.Fatal("expected error for deeply nested YAML")
	}
}

func TestSafeYAMLDecode_AcceptsNormalYAML(t *testing.T) {
	input := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")
	result, err := SafeYAMLDecode(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["kind"] != "ConfigMap" {
		t.Errorf("expected ConfigMap, got %v", result["kind"])
	}
}
```

**Step 2: Implement yaml.go**

```go
package security

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	maxYAMLSize  = 10 * 1024 * 1024 // 10MB
	maxYAMLDepth = 100
)

func SafeYAMLDecode(data []byte) (map[string]interface{}, error) {
	if len(data) > maxYAMLSize {
		return nil, fmt.Errorf("YAML exceeds maximum size of %d bytes", maxYAMLSize)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("parsing YAML: %w", err)
	}

	depth := measureDepth(&node)
	if depth > maxYAMLDepth {
		return nil, fmt.Errorf("YAML exceeds maximum nesting depth of %d (got %d)", maxYAMLDepth, depth)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("decoding YAML: %w", err)
	}
	return result, nil
}

func measureDepth(node *yaml.Node) int {
	if node == nil {
		return 0
	}
	maxChild := 0
	for _, child := range node.Content {
		d := measureDepth(child)
		if d > maxChild {
			maxChild = d
		}
	}
	return maxChild + 1
}

// SanitizeString strips control characters from output.
func SanitizeString(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= 32 || r == '\n' || r == '\t' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
```

**Step 3: Run tests**

Run: `go test ./pkg/security/ -v`
Expected: PASS

**Step 4: Integrate safe YAML into config and manifest renderer**

Update `pkg/config/config.go` and `pkg/renderer/manifest.go` to use `security.SafeYAMLDecode` for initial YAML parsing.

**Step 5: Run all tests**

Run: `go test ./... -v -race`
Expected: ALL PASS

**Step 6: Commit**

```bash
git add -A
git commit -m "feat: YAML safety guards against bombs and oversized input"
```

---

### Task 14: RBAC Documentation

**Files:**
- Create: `deploy/rbac/clusterrole.yaml`
- Create: `deploy/rbac/clusterrole-scoped.yaml`

**Step 1: Create broad read-only ClusterRole**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: driftwatch-readonly
rules:
  - apiGroups: ["*"]
    resources: ["*"]
    verbs: ["get", "list"]
  - nonResourceURLs: ["*"]
    verbs: ["get"]
```

**Step 2: Create scoped ClusterRole for least privilege**

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: driftwatch-scoped
rules:
  - apiGroups: [""]
    resources: ["namespaces", "configmaps", "services", "serviceaccounts", "persistentvolumeclaims", "persistentvolumes"]
    verbs: ["get", "list"]
  - apiGroups: ["apps"]
    resources: ["deployments", "daemonsets", "statefulsets", "replicasets"]
    verbs: ["get", "list"]
  - apiGroups: ["networking.k8s.io"]
    resources: ["ingresses", "networkpolicies"]
    verbs: ["get", "list"]
  - apiGroups: ["rbac.authorization.k8s.io"]
    resources: ["clusterroles", "clusterrolebindings", "roles", "rolebindings"]
    verbs: ["get", "list"]
  - apiGroups: ["cert-manager.io"]
    resources: ["certificates", "issuers", "clusterissuers"]
    verbs: ["get", "list"]
  - apiGroups: ["traefik.io", "traefik.containo.us"]
    resources: ["ingressroutes", "middlewares"]
    verbs: ["get", "list"]
  - apiGroups: ["bitnami.com"]
    resources: ["sealedsecrets"]
    verbs: ["get", "list"]
  - apiGroups: ["helm.toolkit.fluxcd.io"]
    resources: ["helmreleases"]
    verbs: ["get", "list"]
  - apiGroups: ["kustomize.toolkit.fluxcd.io"]
    resources: ["kustomizations"]
    verbs: ["get", "list"]
  - apiGroups: ["source.toolkit.fluxcd.io"]
    resources: ["gitrepositories", "helmrepositories"]
    verbs: ["get", "list"]
```

**Step 3: Commit**

```bash
git add -A
git commit -m "feat: RBAC manifests for cluster read-only access"
```

---

### Task 15: CI Pipeline + Build

**Files:**
- Create: `.github/workflows/ci.yaml`
- Create: `.goreleaser.yaml`
- Create: `Makefile`

**Step 1: Create Makefile**

```makefile
.PHONY: build test lint vet

build:
	go build -o driftwatch .

test:
	go test ./... -v -race

lint:
	golangci-lint run

vet:
	go vet ./...

vulncheck:
	govulncheck ./...
```

**Step 2: Create CI workflow**

```yaml
name: CI
on:
  push:
    branches: [main]
  pull_request:

permissions:
  contents: read

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      - run: go vet ./...
      - run: go test ./... -v -race
      - name: govulncheck
        run: |
          go install golang.org/x/vuln/cmd/govulncheck@latest
          govulncheck ./...
```

**Step 3: Commit**

```bash
git add -A
git commit -m "feat: CI pipeline with tests, vet, and vulnerability scanning"
```

---

### Task 16: Integration Test with Fleet-Infra Fixtures

**Files:**
- Create: `pkg/integration/integration_test.go`
- Create: `testdata/fleet-infra/` (subset fixture from fleet-infra)

**Step 1: Create fixture files mimicking fleet-infra structure**

Copy a minimal subset: one kustomization.yaml, one helmrelease.yaml, one repositories.yaml — enough to exercise the full pipeline.

**Step 2: Write integration test**

Test that discovery finds the correct source types, rendering produces valid objects, and the full pipeline (with mocked fetcher) produces expected results.

**Step 3: Run tests**

Run: `go test ./pkg/integration/ -v -race`
Expected: PASS

**Step 4: Commit**

```bash
git add -A
git commit -m "feat: integration tests with fleet-infra fixture data"
```
