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
