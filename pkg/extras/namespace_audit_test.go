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
