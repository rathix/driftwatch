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
			{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"}:         "HelmReleaseList",
		}, ks)

	expectedSet := map[string]bool{
		"apps/v1/Deployment/default/nginx": true,
	}

	checker := &FluxInventoryChecker{Client: client}
	results, err := checker.Check(context.Background(), expectedSet)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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
			{Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases"}:         "HelmReleaseList",
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

func TestParseInventoryID(t *testing.T) {
	tests := []struct {
		id   string
		want *types.ResourceIdentifier
	}{
		{
			"default_nginx_apps_Deployment",
			&types.ResourceIdentifier{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "default", Name: "nginx"},
		},
		{
			"too_short",
			nil,
		},
	}
	for _, tt := range tests {
		got := parseInventoryID(tt.id)
		if tt.want == nil && got != nil {
			t.Errorf("parseInventoryID(%q) = %v, want nil", tt.id, got)
		}
		if tt.want != nil && got != nil {
			if got.String() != tt.want.String() {
				t.Errorf("parseInventoryID(%q) = %v, want %v", tt.id, got.String(), tt.want.String())
			}
		}
	}
}
