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
