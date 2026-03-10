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
		{ID: types.ResourceIdentifier{Name: "nginx", Namespace: "default"}, Status: types.StatusInSync},
	}

	results, err := detector.Detect(context.Background(), existingResults)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("expected 3 extras, got %d", len(results))
	}
}

func TestDetector_NilLayers(t *testing.T) {
	detector := &Detector{}
	results, err := detector.Detect(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 extras, got %d", len(results))
	}
}
