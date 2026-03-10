package extras

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// HelmNamespaceResolver finds namespaces targeted by HelmReleases.
type HelmNamespaceResolver struct {
	Client dynamic.Interface
}

// Resolve returns a set of namespaces targeted by HelmReleases that have no inventory.
// Resources in these namespaces are likely Helm-managed even without inventory tracking.
func (r *HelmNamespaceResolver) Resolve(ctx context.Context) (map[string]bool, error) {
	hrList, err := r.Client.Resource(helmReleaseGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool)
	for _, hr := range hrList.Items {
		if hasInventory(&hr) {
			continue
		}
		ns := targetNamespace(&hr)
		if ns != "" {
			result[ns] = true
		}
	}
	return result, nil
}

// ResolveAll returns a set of ALL namespaces targeted by HelmReleases (regardless of inventory).
func (r *HelmNamespaceResolver) ResolveAll(ctx context.Context) (map[string]bool, error) {
	hrList, err := r.Client.Resource(helmReleaseGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	result := make(map[string]bool)
	for _, hr := range hrList.Items {
		ns := targetNamespace(&hr)
		if ns != "" {
			result[ns] = true
		}
	}
	return result, nil
}

func hasInventory(hr *unstructured.Unstructured) bool {
	entries, found, _ := unstructured.NestedSlice(hr.Object, "status", "inventory", "entries")
	return found && len(entries) > 0
}

func targetNamespace(hr *unstructured.Unstructured) string {
	// spec.targetNamespace takes precedence
	if ns, ok, _ := unstructured.NestedString(hr.Object, "spec", "targetNamespace"); ok && ns != "" {
		return ns
	}
	// Fall back to spec.storageNamespace, then metadata.namespace
	if ns, ok, _ := unstructured.NestedString(hr.Object, "spec", "storageNamespace"); ok && ns != "" {
		return ns
	}
	return hr.GetNamespace()
}
