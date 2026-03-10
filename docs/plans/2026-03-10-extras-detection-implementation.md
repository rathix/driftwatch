# Extras Detection Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Detect resources in the cluster that don't exist in Git — via Flux inventory checking, namespace resource scanning, and unmanaged namespace auditing.

**Architecture:** Three-layer extras detector added to pkg/extras/, wired into the scan command via --detect-extras flag. Runs after normal scan pipeline, merges results before reporting.

**Tech Stack:** Go, client-go dynamic client, Flux CRD status fields

---

### Task 1: Add DetectionLayer to types and extend config

**Files:**
- Modify: `pkg/types/types.go`
- Modify: `pkg/config/config.go`
- Test: `pkg/types/types_test.go`

**Step 1: Write test for DetectionLayer string**

Add to `pkg/types/types_test.go`:

```go
func TestDetectionLayer_String(t *testing.T) {
	tests := []struct {
		layer DetectionLayer
		want  string
	}{
		{LayerFluxInventory, "flux_inventory"},
		{LayerNamespaceScan, "namespace_scan"},
		{LayerNamespaceAudit, "namespace_audit"},
	}
	for _, tt := range tests {
		if got := tt.layer.String(); got != tt.want {
			t.Errorf("got %q, want %q", got, tt.want)
		}
	}
}
```

**Step 2: Run test — FAIL**

Run: `go test ./pkg/types/ -v -run TestDetectionLayer`

**Step 3: Add DetectionLayer type and extend DriftResult**

Add to `pkg/types/types.go`:

```go
type DetectionLayer string

const (
	LayerFluxInventory  DetectionLayer = "flux_inventory"
	LayerNamespaceScan  DetectionLayer = "namespace_scan"
	LayerNamespaceAudit DetectionLayer = "namespace_audit"
)

func (d DetectionLayer) String() string {
	return string(d)
}
```

Add field to DriftResult:
```go
type DriftResult struct {
	// ...existing fields...
	DetectionLayer DetectionLayer `json:"detection_layer,omitempty"`
}
```

**Step 4: Add Extras config to pkg/config/config.go**

Add to Config struct:
```go
type Config struct {
	// ...existing fields...
	Extras  Extras   `yaml:"extras"`
}

type Extras struct {
	Exclude          []map[string]string `yaml:"exclude"`
	IgnoreNamespaces []string            `yaml:"ignoreNamespaces"`
}
```

Add `"extras"` to `allowedKeys` map.

Add defaults in `applyDefaults`:
```go
if len(cfg.Extras.Exclude) == 0 {
	cfg.Extras.Exclude = []map[string]string{
		{"kind": "Event"},
		{"kind": "Pod"},
		{"kind": "ReplicaSet"},
		{"kind": "Endpoints"},
		{"kind": "EndpointSlice"},
		{"kind": "ControllerRevision"},
		{"kind": "Lease"},
	}
}
if len(cfg.Extras.IgnoreNamespaces) == 0 {
	cfg.Extras.IgnoreNamespaces = []string{
		"kube-system",
		"kube-public",
		"kube-node-lease",
		"default",
	}
}
```

**Step 5: Run all tests**

Run: `go test ./pkg/types/ ./pkg/config/ -v`
Expected: PASS

**Step 6: Commit**

```bash
git add -A && git commit -m "feat: add DetectionLayer type and extras config"
```

---

### Task 2: Layer 1 — Flux Inventory Checker

**Files:**
- Create: `pkg/extras/inventory.go`
- Create: `pkg/extras/inventory_test.go`

**Step 1: Write tests**

```go
package extras

import (
	"context"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestFluxInventoryCheck_FindsExtras(t *testing.T) {
	// Create a Kustomization with inventory entries
	ks := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "kustomize.toolkit.fluxcd.io/v1",
			"kind":       "Kustomization",
			"metadata": map[string]interface{}{
				"name":      "apps",
				"namespace": "flux-system",
			},
			"status": map[string]interface{}{
				"inventory": map[string]interface{}{
					"entries": []interface{}{
						map[string]interface{}{
							"id": "default_nginx_apps_Deployment",
							"v":  "v1",
						},
						map[string]interface{}{
							"id": "default_redis_apps_Deployment",
							"v":  "v1",
						},
					},
				},
			},
		},
	}

	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"}: "KustomizationList",
		}, ks)

	// Expected set only has nginx, not redis
	expectedSet := map[string]bool{
		"apps/v1/Deployment/default/nginx": true,
	}

	checker := &FluxInventoryChecker{Client: client}
	results, err := checker.Check(context.Background(), expectedSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should find redis as extra
	found := false
	for _, r := range results {
		if r.ID.Name == "redis" && r.Status == types.StatusExtra {
			found = true
			if r.Severity != types.SeverityCritical {
				t.Errorf("expected critical severity, got %s", r.Severity)
			}
			if r.DetectionLayer != types.LayerFluxInventory {
				t.Errorf("expected flux_inventory layer, got %s", r.DetectionLayer)
			}
		}
	}
	if !found {
		t.Error("expected redis to be flagged as extra")
	}
}

func TestFluxInventoryCheck_NoExtras(t *testing.T) {
	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations"}: "KustomizationList",
		})

	checker := &FluxInventoryChecker{Client: client}
	results, err := checker.Check(context.Background(), map[string]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 extras, got %d", len(results))
	}
}
```

**Step 2: Run test — FAIL**

Run: `go test ./pkg/extras/ -v`

**Step 3: Implement inventory.go**

```go
package extras

import (
	"context"
	"strings"

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
)

// FluxInventoryChecker finds resources in Flux inventory that aren't in the expected set.
type FluxInventoryChecker struct {
	Client dynamic.Interface
}

// Check reads all Flux Kustomization and HelmRelease inventories and returns
// DriftResults for entries not found in expectedSet.
// expectedSet keys are ResourceIdentifier.String() values.
func (c *FluxInventoryChecker) Check(ctx context.Context, expectedSet map[string]bool) ([]types.DriftResult, error) {
	var results []types.DriftResult

	// Check Kustomization inventories
	ksList, err := c.Client.Resource(kustomizationGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, ks := range ksList.Items {
			extras := c.checkInventory(&ks, expectedSet)
			results = append(results, extras...)
		}
	}

	// Check HelmRelease inventories
	hrList, err := c.Client.Resource(helmReleaseGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, hr := range hrList.Items {
			extras := c.checkInventory(&hr, expectedSet)
			results = append(results, extras...)
		}
	}

	return results, nil
}

func (c *FluxInventoryChecker) checkInventory(obj *unstructured.Unstructured, expectedSet map[string]bool) []types.DriftResult {
	var results []types.DriftResult

	entries, found, _ := unstructured.NestedSlice(obj.Object, "status", "inventory", "entries")
	if !found {
		return nil
	}

	for _, entry := range entries {
		e, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := e["id"].(string)
		if id == "" {
			continue
		}

		rid := parseInventoryID(id)
		if rid == nil {
			continue
		}

		if !expectedSet[rid.String()] {
			results = append(results, types.DriftResult{
				ID:             *rid,
				Status:         types.StatusExtra,
				Severity:       types.SeverityCritical,
				DetectionLayer: types.LayerFluxInventory,
				Source: types.SourceInfo{
					FluxRef: &types.FluxRef{
						Kind:      obj.GetKind(),
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
				},
			})
		}
	}

	return results
}

// parseInventoryID parses Flux inventory ID format: "namespace_name_group_Kind"
// Example: "default_nginx_apps_Deployment"
func parseInventoryID(id string) *types.ResourceIdentifier {
	parts := strings.Split(id, "_")
	if len(parts) < 4 {
		return nil
	}
	namespace := parts[0]
	name := parts[1]
	group := parts[2]
	kind := parts[3]

	apiVersion := group
	if apiVersion == "" {
		apiVersion = "v1"
	} else {
		apiVersion = group + "/v1"
	}

	return &types.ResourceIdentifier{
		APIVersion: apiVersion,
		Kind:       kind,
		Namespace:  namespace,
		Name:       name,
	}
}
```

**Step 4: Run tests — PASS**

Run: `go test ./pkg/extras/ -v`

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: layer 1 — flux inventory checker for extra resources"
```

---

### Task 3: Layer 2 — Namespace Resource Scanner

**Files:**
- Create: `pkg/extras/namespace_scan.go`
- Create: `pkg/extras/namespace_scan_test.go`

**Step 1: Write tests**

```go
package extras

import (
	"context"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestNamespaceScanner_FindsRogueResources(t *testing.T) {
	// Create a rogue configmap in managed namespace
	rogue := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "rogue-config",
				"namespace": "prod",
			},
		},
	}

	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		}, rogue)

	scanner := &NamespaceScanner{
		Client:       client,
		ExcludeKinds: []string{"Event", "Pod", "ReplicaSet"},
		ResourceTypes: []schema.GroupVersionResource{
			{Group: "", Version: "v1", Resource: "configmaps"},
		},
	}

	expectedSet := map[string]bool{}
	inventorySet := map[string]bool{}
	managedNamespaces := []string{"prod"}

	results, err := scanner.Scan(context.Background(), managedNamespaces, expectedSet, inventorySet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 extra, got %d", len(results))
	}
	if results[0].ID.Name != "rogue-config" {
		t.Errorf("expected rogue-config, got %s", results[0].ID.Name)
	}
	if results[0].Severity != types.SeverityWarning {
		t.Errorf("expected warning severity, got %s", results[0].Severity)
	}
	if results[0].DetectionLayer != types.LayerNamespaceScan {
		t.Errorf("expected namespace_scan layer, got %s", results[0].DetectionLayer)
	}
}

func TestNamespaceScanner_SkipsKnownResources(t *testing.T) {
	known := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "known-config",
				"namespace": "prod",
			},
		},
	}

	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "configmaps"}: "ConfigMapList",
		}, known)

	scanner := &NamespaceScanner{
		Client:       client,
		ExcludeKinds: []string{},
		ResourceTypes: []schema.GroupVersionResource{
			{Group: "", Version: "v1", Resource: "configmaps"},
		},
	}

	expectedSet := map[string]bool{
		"v1/ConfigMap/prod/known-config": true,
	}

	results, err := scanner.Scan(context.Background(), []string{"prod"}, expectedSet, map[string]bool{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 extras, got %d", len(results))
	}
}
```

**Step 2: Run test — FAIL**

Run: `go test ./pkg/extras/ -v -run TestNamespaceScanner`

**Step 3: Implement namespace_scan.go**

```go
package extras

import (
	"context"

	"github.com/kennyandries/driftwatch/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// NamespaceScanner lists resources in managed namespaces to find rogue resources.
type NamespaceScanner struct {
	Client        dynamic.Interface
	ExcludeKinds  []string
	ResourceTypes []schema.GroupVersionResource // resource types to scan
}

// Scan lists resources in managedNamespaces and flags any not in expectedSet or inventorySet.
func (s *NamespaceScanner) Scan(ctx context.Context, managedNamespaces []string, expectedSet map[string]bool, inventorySet map[string]bool) ([]types.DriftResult, error) {
	excludeMap := make(map[string]bool)
	for _, k := range s.ExcludeKinds {
		excludeMap[k] = true
	}

	var results []types.DriftResult

	for _, ns := range managedNamespaces {
		for _, gvr := range s.ResourceTypes {
			list, err := s.Client.Resource(gvr).Namespace(ns).List(ctx, metav1.ListOptions{})
			if err != nil {
				continue
			}

			for _, item := range list.Items {
				kind := item.GetKind()
				if excludeMap[kind] {
					continue
				}

				rid := types.ResourceIdentifier{
					APIVersion: item.GetAPIVersion(),
					Kind:       kind,
					Namespace:  item.GetNamespace(),
					Name:       item.GetName(),
				}

				if expectedSet[rid.String()] || inventorySet[rid.String()] {
					continue
				}

				results = append(results, types.DriftResult{
					ID:             rid,
					Status:         types.StatusExtra,
					Severity:       types.SeverityWarning,
					DetectionLayer: types.LayerNamespaceScan,
				})
			}
		}
	}

	return results, nil
}
```

**Step 4: Run tests — PASS**

Run: `go test ./pkg/extras/ -v`

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: layer 2 — namespace resource scanner for rogue resources"
```

---

### Task 4: Layer 3 — Unmanaged Namespace Audit

**Files:**
- Create: `pkg/extras/namespace_audit.go`
- Create: `pkg/extras/namespace_audit_test.go`

**Step 1: Write tests**

```go
package extras

import (
	"context"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
)

func TestNamespaceAudit_FindsUnmanaged(t *testing.T) {
	ns1 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata":   map[string]interface{}{"name": "managed"},
		},
	}
	ns2 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata":   map[string]interface{}{"name": "rogue-ns"},
		},
	}
	ns3 := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata":   map[string]interface{}{"name": "kube-system"},
		},
	}

	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "namespaces"}: "NamespaceList",
		}, ns1, ns2, ns3)

	auditor := &NamespaceAuditor{
		Client:           client,
		IgnoreNamespaces: []string{"kube-system", "kube-public", "kube-node-lease", "default"},
	}

	managedNamespaces := []string{"managed"}

	results, err := auditor.Audit(context.Background(), managedNamespaces)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 unmanaged namespace, got %d", len(results))
	}
	if results[0].ID.Name != "rogue-ns" {
		t.Errorf("expected rogue-ns, got %s", results[0].ID.Name)
	}
	if results[0].DetectionLayer != types.LayerNamespaceAudit {
		t.Errorf("expected namespace_audit layer, got %s", results[0].DetectionLayer)
	}
}

func TestNamespaceAudit_AllManaged(t *testing.T) {
	ns := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "Namespace",
			"metadata":   map[string]interface{}{"name": "prod"},
		},
	}

	scheme := runtime.NewScheme()
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme,
		map[schema.GroupVersionResource]string{
			{Group: "", Version: "v1", Resource: "namespaces"}: "NamespaceList",
		}, ns)

	auditor := &NamespaceAuditor{
		Client:           client,
		IgnoreNamespaces: []string{},
	}

	results, err := auditor.Audit(context.Background(), []string{"prod"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0, got %d", len(results))
	}
}
```

**Step 2: Run test — FAIL**

Run: `go test ./pkg/extras/ -v -run TestNamespaceAudit`

**Step 3: Implement namespace_audit.go**

```go
package extras

import (
	"context"

	"github.com/kennyandries/driftwatch/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var namespaceGVR = schema.GroupVersionResource{
	Group: "", Version: "v1", Resource: "namespaces",
}

// NamespaceAuditor flags namespaces not managed by Flux.
type NamespaceAuditor struct {
	Client           dynamic.Interface
	IgnoreNamespaces []string
}

// Audit lists all namespaces and flags any not in managedNamespaces or ignoreNamespaces.
func (a *NamespaceAuditor) Audit(ctx context.Context, managedNamespaces []string) ([]types.DriftResult, error) {
	ignoreMap := make(map[string]bool)
	for _, ns := range a.IgnoreNamespaces {
		ignoreMap[ns] = true
	}
	managedMap := make(map[string]bool)
	for _, ns := range managedNamespaces {
		managedMap[ns] = true
	}

	nsList, err := a.Client.Resource(namespaceGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var results []types.DriftResult
	for _, ns := range nsList.Items {
		name := ns.GetName()
		if ignoreMap[name] || managedMap[name] {
			continue
		}

		results = append(results, types.DriftResult{
			ID: types.ResourceIdentifier{
				APIVersion: "v1",
				Kind:       "Namespace",
				Name:       name,
			},
			Status:         types.StatusExtra,
			Severity:       types.SeverityWarning,
			DetectionLayer: types.LayerNamespaceAudit,
		})
	}

	return results, nil
}
```

**Step 4: Run tests — PASS**

Run: `go test ./pkg/extras/ -v`

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: layer 3 — unmanaged namespace auditor"
```

---

### Task 5: Orchestrator + Managed Namespace Resolution

**Files:**
- Create: `pkg/extras/detector.go`
- Create: `pkg/extras/detector_test.go`

**Step 1: Write test**

```go
package extras

import (
	"context"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
)

type mockInventoryChecker struct {
	results []types.DriftResult
}
func (m *mockInventoryChecker) Check(_ context.Context, _ map[string]bool) ([]types.DriftResult, error) {
	return m.results, nil
}

type mockNamespaceScanner struct {
	results []types.DriftResult
}
func (m *mockNamespaceScanner) Scan(_ context.Context, _ []string, _ map[string]bool, _ map[string]bool) ([]types.DriftResult, error) {
	return m.results, nil
}

type mockNamespaceAuditor struct {
	results []types.DriftResult
}
func (m *mockNamespaceAuditor) Audit(_ context.Context, _ []string) ([]types.DriftResult, error) {
	return m.results, nil
}

func TestDetector_CombinesAllLayers(t *testing.T) {
	detector := &Detector{
		InventoryChecker: &mockInventoryChecker{
			results: []types.DriftResult{
				{ID: types.ResourceIdentifier{Name: "inv-extra"}, Status: types.StatusExtra, DetectionLayer: types.LayerFluxInventory},
			},
		},
		NamespaceScanner: &mockNamespaceScanner{
			results: []types.DriftResult{
				{ID: types.ResourceIdentifier{Name: "ns-extra"}, Status: types.StatusExtra, DetectionLayer: types.LayerNamespaceScan},
			},
		},
		NamespaceAuditor: &mockNamespaceAuditor{
			results: []types.DriftResult{
				{ID: types.ResourceIdentifier{Name: "unmanaged-ns"}, Status: types.StatusExtra, DetectionLayer: types.LayerNamespaceAudit},
			},
		},
	}

	existingResults := []types.DriftResult{
		{ID: types.ResourceIdentifier{Name: "nginx"}, Status: types.StatusInSync},
	}

	results, err := detector.Detect(context.Background(), existingResults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 extras, got %d", len(results))
	}
}
```

**Step 2: Run test — FAIL**

**Step 3: Implement detector.go**

```go
package extras

import (
	"context"

	"github.com/kennyandries/driftwatch/pkg/types"
)

// InventoryCheckerInterface abstracts Layer 1.
type InventoryCheckerInterface interface {
	Check(ctx context.Context, expectedSet map[string]bool) ([]types.DriftResult, error)
}

// NamespaceScannerInterface abstracts Layer 2.
type NamespaceScannerInterface interface {
	Scan(ctx context.Context, managedNamespaces []string, expectedSet map[string]bool, inventorySet map[string]bool) ([]types.DriftResult, error)
}

// NamespaceAuditorInterface abstracts Layer 3.
type NamespaceAuditorInterface interface {
	Audit(ctx context.Context, managedNamespaces []string) ([]types.DriftResult, error)
}

// Detector orchestrates all three extras detection layers.
type Detector struct {
	InventoryChecker InventoryCheckerInterface
	NamespaceScanner NamespaceScannerInterface
	NamespaceAuditor NamespaceAuditorInterface
}

// Detect runs all three layers and returns combined extra results.
// existingResults are from the normal scan pipeline — used to build the expected set.
func (d *Detector) Detect(ctx context.Context, existingResults []types.DriftResult) ([]types.DriftResult, error) {
	// Build expected set from existing scan results
	expectedSet := make(map[string]bool)
	managedNamespaces := make(map[string]bool)
	for _, r := range existingResults {
		expectedSet[r.ID.String()] = true
		if r.ID.Namespace != "" {
			managedNamespaces[r.ID.Namespace] = true
		}
	}

	var allExtras []types.DriftResult

	// Layer 1: Flux inventory
	inventorySet := make(map[string]bool)
	if d.InventoryChecker != nil {
		invResults, err := d.InventoryChecker.Check(ctx, expectedSet)
		if err != nil {
			return nil, err
		}
		allExtras = append(allExtras, invResults...)
		// Build inventory set for Layer 2 (don't double-flag)
		for _, r := range existingResults {
			inventorySet[r.ID.String()] = true
		}
	}

	// Layer 2: Namespace resource scan
	if d.NamespaceScanner != nil {
		nsList := make([]string, 0, len(managedNamespaces))
		for ns := range managedNamespaces {
			nsList = append(nsList, ns)
		}
		nsResults, err := d.NamespaceScanner.Scan(ctx, nsList, expectedSet, inventorySet)
		if err != nil {
			return nil, err
		}
		allExtras = append(allExtras, nsResults...)
	}

	// Layer 3: Unmanaged namespace audit
	if d.NamespaceAuditor != nil {
		nsList := make([]string, 0, len(managedNamespaces))
		for ns := range managedNamespaces {
			nsList = append(nsList, ns)
		}
		auditResults, err := d.NamespaceAuditor.Audit(ctx, nsList)
		if err != nil {
			return nil, err
		}
		allExtras = append(allExtras, auditResults...)
	}

	return allExtras, nil
}
```

**Step 4: Run tests — PASS**

Run: `go test ./pkg/extras/ -v`

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: extras detector orchestrating all three layers"
```

---

### Task 6: Wire into CLI + Update Reporters

**Files:**
- Modify: `cmd/scan.go`
- Modify: `pkg/reporter/terminal.go`
- Modify: `pkg/reporter/json.go`
- Modify: `pkg/reporter/reporter_test.go`

**Step 1: Add --detect-extras flag to cmd/scan.go init()**

Add to init():
```go
scanCmd.Flags().Bool("detect-extras", false, "Detect extra resources not in Git")
```

**Step 2: Wire extras detection in scan RunE**

After flux enrichment and before reporting, add:

```go
detectExtras, _ := cmd.Flags().GetBool("detect-extras")
if detectExtras && dynClient != nil {
    // Resolve resource types to scan (use discovery client)
    var resourceTypes []schema.GroupVersionResource
    if resourceMapper != nil {
        // Use common resource types
        resourceTypes = commonResourceTypes()
    }

    // Build extras config
    excludeKinds := []string{"Event", "Pod", "ReplicaSet", "Endpoints", "EndpointSlice", "ControllerRevision", "Lease"}
    ignoreNS := []string{"kube-system", "kube-public", "kube-node-lease", "default"}
    if cfg != nil {
        if len(cfg.Extras.Exclude) > 0 {
            excludeKinds = nil
            for _, e := range cfg.Extras.Exclude {
                if k, ok := e["kind"]; ok {
                    excludeKinds = append(excludeKinds, k)
                }
            }
        }
        if len(cfg.Extras.IgnoreNamespaces) > 0 {
            ignoreNS = cfg.Extras.IgnoreNamespaces
        }
    }

    detector := &extras.Detector{
        InventoryChecker: &extras.FluxInventoryChecker{Client: dynClient},
        NamespaceScanner: &extras.NamespaceScanner{
            Client:        dynClient,
            ExcludeKinds:  excludeKinds,
            ResourceTypes: resourceTypes,
        },
        NamespaceAuditor: &extras.NamespaceAuditor{
            Client:           dynClient,
            IgnoreNamespaces: ignoreNS,
        },
    }

    extrasResults, extrasErr := detector.Detect(context.Background(), allResults)
    if extrasErr != nil {
        fmt.Fprintf(os.Stderr, "Warning: extras detection error: %v\n", extrasErr)
    } else {
        allResults = append(allResults, extrasResults...)
    }
}
```

Add helper function:
```go
func commonResourceTypes() []schema.GroupVersionResource {
    return []schema.GroupVersionResource{
        {Group: "", Version: "v1", Resource: "configmaps"},
        {Group: "", Version: "v1", Resource: "services"},
        {Group: "", Version: "v1", Resource: "serviceaccounts"},
        {Group: "", Version: "v1", Resource: "persistentvolumeclaims"},
        {Group: "apps", Version: "v1", Resource: "deployments"},
        {Group: "apps", Version: "v1", Resource: "daemonsets"},
        {Group: "apps", Version: "v1", Resource: "statefulsets"},
        {Group: "batch", Version: "v1", Resource: "cronjobs"},
        {Group: "batch", Version: "v1", Resource: "jobs"},
        {Group: "networking.k8s.io", Version: "v1", Resource: "ingresses"},
        {Group: "networking.k8s.io", Version: "v1", Resource: "networkpolicies"},
        {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "roles"},
        {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "rolebindings"},
    }
}
```

**Step 3: Update terminal reporter for detection layer**

In `pkg/reporter/terminal.go`, update the StatusExtra case:
```go
case types.StatusExtra:
    extra++
    severity := result.Severity.String()
    layer := ""
    switch result.DetectionLayer {
    case types.LayerFluxInventory:
        layer = "In Flux inventory but not in Git sources"
    case types.LayerNamespaceScan:
        layer = "In managed namespace but not in Git or Flux inventory"
    case types.LayerNamespaceAudit:
        layer = "No Flux Kustomization or HelmRelease targets this namespace"
    }
    fmt.Fprintf(tr.w, "[%s] EXTRA: %s\n", strings.ToUpper(severity), result.ID)
    if layer != "" {
        fmt.Fprintf(tr.w, "  %s\n", layer)
    }
```

Add `"strings"` to imports.

**Step 4: Update JSON reporter**

Add `DetectionLayer` to the JSON output — it's already on DriftResult with `json:"detection_layer,omitempty"`, so no code change needed. Just add "unmanaged" count to summary:

In `pkg/reporter/json.go`, add `Unmanaged int` to jsonSummary and count it.

**Step 5: Run all tests**

Run: `go test ./... -v -race`
Expected: ALL PASS

**Step 6: Build and smoke test**

Run: `go build -o driftwatch . && ./driftwatch scan --help`
Expected: `--detect-extras` flag visible in help output

**Step 7: Commit**

```bash
git add -A && git commit -m "feat: wire extras detection into CLI with --detect-extras flag"
```

---

### Task 7: Tests + README Update

**Files:**
- Modify: `pkg/extras/inventory_test.go` (if needed)
- Modify: `README.md`

**Step 1: Run full test suite**

Run: `go test ./... -v -race`
Expected: ALL PASS

**Step 2: Update README.md**

Add an "Extras Detection" section after the "FluxCD Integration" section explaining `--detect-extras`, the three layers, and configuration.

**Step 3: Commit**

```bash
git add -A && git commit -m "docs: add extras detection to README"
```
