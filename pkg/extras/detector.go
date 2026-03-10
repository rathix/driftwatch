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
	// HelmSkipped returns the number of resources skipped due to Helm ownership labels.
	HelmSkipped() int
}

// NamespaceAuditorInterface abstracts Layer 3.
type NamespaceAuditorInterface interface {
	Audit(ctx context.Context, managedNamespaces []string) ([]types.DriftResult, error)
}

// HelmNamespaceResolverInterface resolves namespaces targeted by HelmReleases.
type HelmNamespaceResolverInterface interface {
	// Resolve returns namespaces targeted by HelmReleases without inventory.
	Resolve(ctx context.Context) (map[string]bool, error)
	// ResolveAll returns namespaces targeted by all HelmReleases.
	ResolveAll(ctx context.Context) (map[string]bool, error)
}

// Detector orchestrates all three extras detection layers.
type Detector struct {
	InventoryChecker      InventoryCheckerInterface
	NamespaceScanner      NamespaceScannerInterface
	NamespaceAuditor      NamespaceAuditorInterface
	HelmNamespaceResolver HelmNamespaceResolverInterface
	IgnoreNamespaces      []string
}

// Detect runs all three layers and returns combined extra results plus a summary of skipped items.
// existingResults are from the normal scan pipeline — used to build the expected set.
func (d *Detector) Detect(ctx context.Context, existingResults []types.DriftResult) ([]types.DriftResult, *types.SkippedSummary, error) {
	expectedSet := make(map[string]bool)
	managedNamespaces := make(map[string]bool)
	for _, r := range existingResults {
		expectedSet[r.ID.String()] = true
		if r.ID.Namespace != "" {
			managedNamespaces[r.ID.Namespace] = true
		}
		// Namespace resources are cluster-scoped (empty namespace field),
		// but the namespace they represent should be considered managed.
		if r.ID.Kind == "Namespace" && r.ID.Name != "" {
			managedNamespaces[r.ID.Name] = true
		}
	}

	// Remove ignored namespaces from Layer 2 scanning
	ignoreMap := make(map[string]bool)
	for _, ns := range d.IgnoreNamespaces {
		ignoreMap[ns] = true
		delete(managedNamespaces, ns)
	}

	// Add HelmRelease target namespaces to managedNamespaces
	// (covers namespaces like reloader/sealed-secrets that are only referenced via targetNamespace)
	helmNamespaces := make(map[string]bool)
	if d.HelmNamespaceResolver != nil {
		allHelmNS, err := d.HelmNamespaceResolver.ResolveAll(ctx)
		if err == nil {
			for ns := range allHelmNS {
				if !ignoreMap[ns] {
					managedNamespaces[ns] = true
				}
			}
		}

		// No-inventory namespaces for Layer 2 filtering
		helmNamespaces, err = d.HelmNamespaceResolver.Resolve(ctx)
		if err != nil {
			helmNamespaces = make(map[string]bool)
		}
	}

	var allExtras []types.DriftResult
	skipped := &types.SkippedSummary{}

	// Layer 1: Flux inventory
	inventorySet := make(map[string]bool)
	if d.InventoryChecker != nil {
		invResults, err := d.InventoryChecker.Check(ctx, expectedSet)
		if err != nil {
			return nil, nil, err
		}
		for _, r := range invResults {
			if r.Source.FluxRef != nil && r.Source.FluxRef.Kind == "HelmRelease" {
				skipped.HelmManagedResources++
				// Still track in inventorySet so Layer 2 doesn't double-flag
				inventorySet[r.ID.String()] = true
				continue
			}
			allExtras = append(allExtras, r)
			inventorySet[r.ID.String()] = true
		}
	}

	// Layer 2: Namespace resource scan
	nsList := make([]string, 0, len(managedNamespaces))
	for ns := range managedNamespaces {
		nsList = append(nsList, ns)
	}

	if d.NamespaceScanner != nil {
		nsResults, err := d.NamespaceScanner.Scan(ctx, nsList, expectedSet, inventorySet)
		if err != nil {
			return nil, nil, err
		}
		skipped.HelmManagedResources += d.NamespaceScanner.HelmSkipped()
		for _, r := range nsResults {
			if isKubeAutoCreated(r.ID) {
				skipped.KubeDefaultResources++
				continue
			}
			if helmNamespaces[r.ID.Namespace] {
				skipped.HelmManagedResources++
				continue
			}
			allExtras = append(allExtras, r)
		}
	}

	// Layer 3: Unmanaged namespace audit
	if d.NamespaceAuditor != nil {
		auditResults, err := d.NamespaceAuditor.Audit(ctx, nsList)
		if err != nil {
			return nil, nil, err
		}
		allExtras = append(allExtras, auditResults...)
	}

	return allExtras, skipped, nil
}

// isKubeAutoCreated returns true for resources Kubernetes automatically creates in every namespace.
func isKubeAutoCreated(id types.ResourceIdentifier) bool {
	if id.Kind == "ConfigMap" && id.Name == "kube-root-ca.crt" {
		return true
	}
	if id.Kind == "ServiceAccount" && id.Name == "default" {
		return true
	}
	return false
}
