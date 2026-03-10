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
