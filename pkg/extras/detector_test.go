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
	results     []types.DriftResult
	helmSkipped int
}

func (m *mockNamespaceScanner) Scan(_ context.Context, _ []string, _ map[string]bool, _ map[string]bool) ([]types.DriftResult, error) {
	return m.results, nil
}

func (m *mockNamespaceScanner) HelmSkipped() int {
	return m.helmSkipped
}

type mockNamespaceAuditor struct {
	results []types.DriftResult
}

func (m *mockNamespaceAuditor) Audit(_ context.Context, _ []string) ([]types.DriftResult, error) {
	return m.results, nil
}

type mockHelmNamespaceResolver struct {
	namespaces    map[string]bool
	allNamespaces map[string]bool
}

func (m *mockHelmNamespaceResolver) Resolve(_ context.Context) (map[string]bool, error) {
	return m.namespaces, nil
}

func (m *mockHelmNamespaceResolver) ResolveAll(_ context.Context) (map[string]bool, error) {
	if m.allNamespaces != nil {
		return m.allNamespaces, nil
	}
	return m.namespaces, nil
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
		{ID: types.ResourceIdentifier{Name: "nginx", Namespace: "default"}, Status: types.StatusInSync},
	}

	results, skipped, err := detector.Detect(context.Background(), existingResults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 extras, got %d", len(results))
	}
	if skipped.HelmManagedResources != 0 {
		t.Errorf("expected 0 helm skipped, got %d", skipped.HelmManagedResources)
	}
	if skipped.KubeDefaultResources != 0 {
		t.Errorf("expected 0 kube skipped, got %d", skipped.KubeDefaultResources)
	}
}

func TestDetector_NilLayers(t *testing.T) {
	detector := &Detector{}
	results, skipped, err := detector.Detect(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 extras, got %d", len(results))
	}
	if skipped.HelmManagedResources != 0 || skipped.KubeDefaultResources != 0 {
		t.Errorf("expected zero skipped counts")
	}
}

func TestDetector_SkipsHelmManagedResources(t *testing.T) {
	detector := &Detector{
		InventoryChecker: &mockInventoryChecker{
			results: []types.DriftResult{
				{
					ID:             types.ResourceIdentifier{Kind: "Deployment", Namespace: "cert-manager", Name: "cert-manager"},
					Status:         types.StatusExtra,
					DetectionLayer: types.LayerFluxInventory,
					Source: types.SourceInfo{
						FluxRef: &types.FluxRef{Kind: "HelmRelease", Name: "cert-manager", Namespace: "flux-system"},
					},
				},
				{
					ID:             types.ResourceIdentifier{Kind: "ClusterRole", Name: "cert-manager-controller"},
					Status:         types.StatusExtra,
					DetectionLayer: types.LayerFluxInventory,
					Source: types.SourceInfo{
						FluxRef: &types.FluxRef{Kind: "HelmRelease", Name: "cert-manager", Namespace: "flux-system"},
					},
				},
				{
					ID:             types.ResourceIdentifier{Kind: "Deployment", Namespace: "default", Name: "rogue-app"},
					Status:         types.StatusExtra,
					DetectionLayer: types.LayerFluxInventory,
					Source: types.SourceInfo{
						FluxRef: &types.FluxRef{Kind: "Kustomization", Name: "apps", Namespace: "flux-system"},
					},
				},
			},
		},
	}

	results, skipped, err := detector.Detect(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (kustomization extra only), got %d", len(results))
	}
	if results[0].ID.Name != "rogue-app" {
		t.Errorf("expected rogue-app, got %s", results[0].ID.Name)
	}
	if skipped.HelmManagedResources != 2 {
		t.Errorf("expected 2 helm skipped, got %d", skipped.HelmManagedResources)
	}
}

func TestDetector_SkipsKubeAutoCreated(t *testing.T) {
	detector := &Detector{
		NamespaceScanner: &mockNamespaceScanner{
			results: []types.DriftResult{
				{
					ID:             types.ResourceIdentifier{Kind: "ConfigMap", Namespace: "prod", Name: "kube-root-ca.crt"},
					Status:         types.StatusExtra,
					DetectionLayer: types.LayerNamespaceScan,
				},
				{
					ID:             types.ResourceIdentifier{Kind: "ServiceAccount", Namespace: "prod", Name: "default"},
					Status:         types.StatusExtra,
					DetectionLayer: types.LayerNamespaceScan,
				},
				{
					ID:             types.ResourceIdentifier{Kind: "ConfigMap", Namespace: "prod", Name: "rogue-config"},
					Status:         types.StatusExtra,
					DetectionLayer: types.LayerNamespaceScan,
				},
			},
		},
	}

	existingResults := []types.DriftResult{
		{ID: types.ResourceIdentifier{Name: "app", Namespace: "prod"}, Status: types.StatusInSync},
	}

	results, skipped, err := detector.Detect(context.Background(), existingResults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (rogue-config only), got %d", len(results))
	}
	if results[0].ID.Name != "rogue-config" {
		t.Errorf("expected rogue-config, got %s", results[0].ID.Name)
	}
	if skipped.KubeDefaultResources != 2 {
		t.Errorf("expected 2 kube skipped, got %d", skipped.KubeDefaultResources)
	}
}

func TestDetector_HelmSkippedStillInInventorySet(t *testing.T) {
	detector := &Detector{
		InventoryChecker: &mockInventoryChecker{
			results: []types.DriftResult{
				{
					ID:             types.ResourceIdentifier{Kind: "Deployment", Namespace: "prod", Name: "helm-app"},
					Status:         types.StatusExtra,
					DetectionLayer: types.LayerFluxInventory,
					Source: types.SourceInfo{
						FluxRef: &types.FluxRef{Kind: "HelmRelease", Name: "myrelease", Namespace: "flux-system"},
					},
				},
			},
		},
		NamespaceScanner: &mockNamespaceScanner{
			results: []types.DriftResult{},
		},
	}

	existingResults := []types.DriftResult{
		{ID: types.ResourceIdentifier{Name: "other", Namespace: "prod"}, Status: types.StatusInSync},
	}

	results, skipped, err := detector.Detect(context.Background(), existingResults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
	if skipped.HelmManagedResources != 1 {
		t.Errorf("expected 1 helm skipped, got %d", skipped.HelmManagedResources)
	}
}

func TestDetector_IgnoresNamespacesInLayer2(t *testing.T) {
	detector := &Detector{
		NamespaceScanner: &mockNamespaceScanner{
			results: []types.DriftResult{
				{
					ID:             types.ResourceIdentifier{Kind: "ConfigMap", Namespace: "prod", Name: "real-extra"},
					Status:         types.StatusExtra,
					DetectionLayer: types.LayerNamespaceScan,
				},
			},
		},
		IgnoreNamespaces: []string{"kube-system"},
	}

	// kube-system is in existingResults but should be excluded from Layer 2 scanning
	existingResults := []types.DriftResult{
		{ID: types.ResourceIdentifier{Name: "app", Namespace: "prod"}, Status: types.StatusInSync},
		{ID: types.ResourceIdentifier{Name: "leader-election", Namespace: "kube-system"}, Status: types.StatusInSync},
	}

	results, _, err := detector.Detect(context.Background(), existingResults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].ID.Name != "real-extra" {
		t.Errorf("expected real-extra, got %s", results[0].ID.Name)
	}
}

func TestDetector_SkipsHelmNamespacesWithoutInventory(t *testing.T) {
	detector := &Detector{
		NamespaceScanner: &mockNamespaceScanner{
			results: []types.DriftResult{
				{
					ID:             types.ResourceIdentifier{Kind: "Deployment", Namespace: "servarr", Name: "servarr-bazarr"},
					Status:         types.StatusExtra,
					DetectionLayer: types.LayerNamespaceScan,
				},
				{
					ID:             types.ResourceIdentifier{Kind: "Service", Namespace: "servarr", Name: "servarr-bazarr"},
					Status:         types.StatusExtra,
					DetectionLayer: types.LayerNamespaceScan,
				},
				{
					ID:             types.ResourceIdentifier{Kind: "ConfigMap", Namespace: "prod", Name: "rogue"},
					Status:         types.StatusExtra,
					DetectionLayer: types.LayerNamespaceScan,
				},
			},
		},
		HelmNamespaceResolver: &mockHelmNamespaceResolver{
			namespaces: map[string]bool{"servarr": true},
		},
	}

	existingResults := []types.DriftResult{
		{ID: types.ResourceIdentifier{Name: "app", Namespace: "prod"}, Status: types.StatusInSync},
		{ID: types.ResourceIdentifier{Name: "other", Namespace: "servarr"}, Status: types.StatusInSync},
	}

	results, skipped, err := detector.Detect(context.Background(), existingResults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result (rogue only), got %d", len(results))
	}
	if results[0].ID.Name != "rogue" {
		t.Errorf("expected rogue, got %s", results[0].ID.Name)
	}
	if skipped.HelmManagedResources != 2 {
		t.Errorf("expected 2 helm skipped, got %d", skipped.HelmManagedResources)
	}
}

func TestIsKubeAutoCreated(t *testing.T) {
	tests := []struct {
		id   types.ResourceIdentifier
		want bool
	}{
		{types.ResourceIdentifier{Kind: "ConfigMap", Name: "kube-root-ca.crt"}, true},
		{types.ResourceIdentifier{Kind: "ServiceAccount", Name: "default"}, true},
		{types.ResourceIdentifier{Kind: "ConfigMap", Name: "app-config"}, false},
		{types.ResourceIdentifier{Kind: "ServiceAccount", Name: "my-app"}, false},
		{types.ResourceIdentifier{Kind: "Deployment", Name: "default"}, false},
	}

	for _, tt := range tests {
		got := isKubeAutoCreated(tt.id)
		if got != tt.want {
			t.Errorf("isKubeAutoCreated(%s/%s) = %v, want %v", tt.id.Kind, tt.id.Name, got, tt.want)
		}
	}
}
