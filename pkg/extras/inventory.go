package extras

import (
	"context"
	"strings"

	"github.com/kennyandries/driftwatch/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	kustomizationGVR = schema.GroupVersionResource{
		Group: "kustomize.toolkit.fluxcd.io", Version: "v1", Resource: "kustomizations",
	}
	helmReleaseGVR = schema.GroupVersionResource{
		Group: "helm.toolkit.fluxcd.io", Version: "v2", Resource: "helmreleases",
	}
)

// FluxInventoryChecker finds resources in Flux inventory that aren't in the expected set.
type FluxInventoryChecker struct {
	Client dynamic.Interface
}

// Check reads all Flux Kustomization and HelmRelease inventories and returns
// DriftResults for entries not found in expectedSet.
// expectedSet keys are ResourceIdentifier.String() values.
func (c *FluxInventoryChecker) Check(ctx context.Context, expectedSet map[string]bool) ([]types.DriftResult, error) {
	var results []types.DriftResult

	// Check Kustomization inventories
	ksList, err := c.Client.Resource(kustomizationGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, ks := range ksList.Items {
			extras := c.checkInventory(&ks, expectedSet)
			results = append(results, extras...)
		}
	}

	// Check HelmRelease inventories
	hrList, err := c.Client.Resource(helmReleaseGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err == nil {
		for _, hr := range hrList.Items {
			extras := c.checkInventory(&hr, expectedSet)
			results = append(results, extras...)
		}
	}

	return results, nil
}

func (c *FluxInventoryChecker) checkInventory(obj *unstructured.Unstructured, expectedSet map[string]bool) []types.DriftResult {
	var results []types.DriftResult

	entries, found, _ := unstructured.NestedSlice(obj.Object, "status", "inventory", "entries")
	if !found {
		return nil
	}

	for _, entry := range entries {
		e, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := e["id"].(string)
		if id == "" {
			continue
		}

		rid := parseInventoryID(id)
		if rid == nil {
			continue
		}

		if !expectedSet[rid.String()] {
			results = append(results, types.DriftResult{
				ID:             *rid,
				Status:         types.StatusExtra,
				Severity:       types.SeverityCritical,
				DetectionLayer: types.LayerFluxInventory,
				Source: types.SourceInfo{
					FluxRef: &types.FluxRef{
						Kind:      obj.GetKind(),
						Name:      obj.GetName(),
						Namespace: obj.GetNamespace(),
					},
				},
			})
		}
	}

	return results
}

// parseInventoryID parses Flux inventory ID format: "namespace_name_group_Kind"
// Example: "default_nginx_apps_Deployment"
func parseInventoryID(id string) *types.ResourceIdentifier {
	parts := strings.Split(id, "_")
	if len(parts) < 4 {
		return nil
	}
	namespace := parts[0]
	name := parts[1]
	group := parts[2]
	kind := parts[3]

	apiVersion := group
	if apiVersion == "" {
		apiVersion = "v1"
	} else {
		apiVersion = group + "/v1"
	}

	return &types.ResourceIdentifier{
		APIVersion: apiVersion,
		Kind:       kind,
		Namespace:  namespace,
		Name:       name,
	}
}
