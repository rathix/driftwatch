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
	ResourceTypes []schema.GroupVersionResource

	// HelmManagedSkipped is populated after Scan with the count of resources
	// skipped because they have Helm ownership labels.
	HelmManagedSkipped int
}

// Scan lists resources in managedNamespaces and flags any not in expectedSet or inventorySet.
// Resources with app.kubernetes.io/managed-by=Helm labels are skipped and counted.
func (s *NamespaceScanner) Scan(ctx context.Context, managedNamespaces []string, expectedSet map[string]bool, inventorySet map[string]bool) ([]types.DriftResult, error) {
	s.HelmManagedSkipped = 0
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

				if item.GetLabels()["app.kubernetes.io/managed-by"] == "Helm" {
					s.HelmManagedSkipped++
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

// HelmSkipped returns the number of resources skipped due to Helm ownership labels.
func (s *NamespaceScanner) HelmSkipped() int {
	return s.HelmManagedSkipped
}
