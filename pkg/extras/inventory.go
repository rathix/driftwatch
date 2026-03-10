package extras

import (
	"context"
	"fmt"
	"io"
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
	Stderr io.Writer // optional: for warning messages about API errors
}

// Check reads all Flux Kustomization and HelmRelease inventories and returns
// DriftResults for entries not found in expectedSet.
// expectedSet keys are ResourceIdentifier.String() values.
func (c *FluxInventoryChecker) Check(ctx context.Context, expectedSet map[string]bool) ([]types.DriftResult, error) {
	var results []types.DriftResult

	// Check Kustomization inventories
	ksList, ksErr := c.Client.Resource(kustomizationGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if ksErr == nil {
		for _, ks := range ksList.Items {
			extras := c.checkInventory(&ks, expectedSet)
			results = append(results, extras...)
		}
	} else {
		c.warnf("could not list Flux Kustomizations: %v\n", ksErr)
	}

	// Check HelmRelease inventories
	hrList, hrErr := c.Client.Resource(helmReleaseGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if hrErr == nil {
		for _, hr := range hrList.Items {
			extras := c.checkInventory(&hr, expectedSet)
			results = append(results, extras...)
		}
	} else {
		c.warnf("could not list Flux HelmReleases: %v\n", hrErr)
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
		version, _ := e["v"].(string)
		if version == "" {
			version = "v1"
		}

		rid := parseInventoryID(id, version)
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
// Parses from both ends to handle names containing underscores.
// Flux encodes colons in names as double underscores ("__"), e.g.
// "system:kube-vip-role" becomes "system__kube-vip-role" in the ID.
// version comes from the "v" field of the inventory entry.
func parseInventoryID(id, version string) *types.ResourceIdentifier {
	parts := strings.Split(id, "_")
	if len(parts) < 4 {
		return nil
	}

	namespace := parts[0]
	kind := parts[len(parts)-1]
	group := parts[len(parts)-2]
	name := strings.ReplaceAll(strings.Join(parts[1:len(parts)-2], "_"), "__", ":")

	if name == "" {
		return nil
	}

	var apiVersion string
	if group == "" {
		apiVersion = version
	} else {
		apiVersion = group + "/" + version
	}

	return &types.ResourceIdentifier{
		APIVersion: apiVersion,
		Kind:       kind,
		Namespace:  namespace,
		Name:       name,
	}
}

func (c *FluxInventoryChecker) warnf(format string, args ...interface{}) {
	if c.Stderr != nil {
		fmt.Fprintf(c.Stderr, "Warning: "+format, args...)
	}
}
