package flux

import (
	"context"

	"github.com/kennyandries/driftwatch/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

var (
	kustomizationGVR = schema.GroupVersionResource{
		Group:    "kustomize.toolkit.fluxcd.io",
		Version:  "v1",
		Resource: "kustomizations",
	}
	helmReleaseGVR = schema.GroupVersionResource{
		Group:    "helm.toolkit.fluxcd.io",
		Version:  "v2",
		Resource: "helmreleases",
	}
)

// Enricher overlays Flux status onto DriftResults.
type Enricher struct {
	Client dynamic.Interface
}

// NewEnricher creates an Enricher with the given dynamic client.
func NewEnricher(client dynamic.Interface) *Enricher {
	return &Enricher{Client: client}
}

// Available returns true if Flux CRDs are present (kustomizations can be listed).
func (e *Enricher) Available(ctx context.Context) (result bool) {
	defer func() {
		if r := recover(); r != nil {
			result = false
		}
	}()
	_, err := e.Client.Resource(kustomizationGVR).Namespace("flux-system").List(ctx, metav1.ListOptions{})
	return err == nil
}

// Enrich fetches HelmReleases and Kustomizations and overlays FluxStatus onto matching results.
func (e *Enricher) Enrich(ctx context.Context, results []types.DriftResult) {
	helmMap := e.buildHelmReleaseMap(ctx)
	kustomMap := e.buildKustomizationMap(ctx)

	for i := range results {
		r := &results[i]
		if r.Source.FluxRef != nil && r.Source.FluxRef.Kind == "HelmRelease" {
			key := r.Source.FluxRef.Namespace + "/" + r.Source.FluxRef.Name
			if obj, ok := helmMap[key]; ok {
				status := e.ExtractFluxStatus(obj)
				r.FluxStatus = &status
			}
		} else if r.Source.Type == types.SourceKustomize && r.Source.Path != "" {
			if obj := e.matchesKustomizationPath(kustomMap, r.Source.Path); obj != nil {
				status := e.ExtractFluxStatus(obj)
				r.FluxStatus = &status
			}
		}
	}
}

// ExtractFluxStatus reads spec.suspend, status.conditions, status.lastAppliedRevision from an unstructured object.
func (e *Enricher) ExtractFluxStatus(obj *unstructured.Unstructured) types.FluxStatus {
	var fs types.FluxStatus

	suspended, found, _ := unstructured.NestedBool(obj.Object, "spec", "suspend")
	if found {
		fs.Suspended = suspended
	}

	lastRev, _, _ := unstructured.NestedString(obj.Object, "status", "lastAppliedRevision")
	fs.LastAppliedRev = lastRev

	conditions, found, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if found {
		for _, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			fs.Conditions = append(fs.Conditions, condType+"="+condStatus)
			if condType == "Ready" && condStatus == "True" {
				fs.Ready = true
			}
		}
	}

	return fs
}

func (e *Enricher) buildHelmReleaseMap(ctx context.Context) map[string]*unstructured.Unstructured {
	m := make(map[string]*unstructured.Unstructured)
	list, err := e.Client.Resource(helmReleaseGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return m
	}
	for i := range list.Items {
		obj := &list.Items[i]
		key := obj.GetNamespace() + "/" + obj.GetName()
		m[key] = obj
	}
	return m
}

func (e *Enricher) buildKustomizationMap(ctx context.Context) map[string]*unstructured.Unstructured {
	m := make(map[string]*unstructured.Unstructured)
	list, err := e.Client.Resource(kustomizationGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return m
	}
	for i := range list.Items {
		obj := &list.Items[i]
		key := obj.GetNamespace() + "/" + obj.GetName()
		m[key] = obj
	}
	return m
}

func (e *Enricher) matchesKustomizationPath(kustomMap map[string]*unstructured.Unstructured, path string) *unstructured.Unstructured {
	for _, obj := range kustomMap {
		p, _, _ := unstructured.NestedString(obj.Object, "spec", "path")
		if p == path {
			return obj
		}
	}
	return nil
}

