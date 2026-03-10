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
}

// NamespaceAuditorInterface abstracts Layer 3.
type NamespaceAuditorInterface interface {
	Audit(ctx context.Context, managedNamespaces []string) ([]types.DriftResult, error)
}

// Detector orchestrates all three extras detection layers.
type Detector struct {
	InventoryChecker InventoryCheckerInterface
	NamespaceScanner NamespaceScannerInterface
	NamespaceAuditor NamespaceAuditorInterface
}

// Detect runs all three layers and returns combined extra results.
// existingResults are from the normal scan pipeline — used to build the expected set.
func (d *Detector) Detect(ctx context.Context, existingResults []types.DriftResult) ([]types.DriftResult, error) {
	expectedSet := make(map[string]bool)
	managedNamespaces := make(map[string]bool)
	for _, r := range existingResults {
		expectedSet[r.ID.String()] = true
		if r.ID.Namespace != "" {
			managedNamespaces[r.ID.Namespace] = true
		}
	}

	var allExtras []types.DriftResult

	// Layer 1: Flux inventory
	inventorySet := make(map[string]bool)
	if d.InventoryChecker != nil {
		invResults, err := d.InventoryChecker.Check(ctx, expectedSet)
		if err != nil {
			return nil, err
		}
		allExtras = append(allExtras, invResults...)
		// Track inventory items so Layer 2 doesn't double-flag
		for _, r := range invResults {
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
			return nil, err
		}
		allExtras = append(allExtras, nsResults...)
	}

	// Layer 3: Unmanaged namespace audit
	if d.NamespaceAuditor != nil {
		auditResults, err := d.NamespaceAuditor.Audit(ctx, nsList)
		if err != nil {
			return nil, err
		}
		allExtras = append(allExtras, auditResults...)
	}

	return allExtras, nil
}
