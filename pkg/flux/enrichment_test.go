package flux_test

import (
	"context"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/flux"
	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestEnricher_DetectsFluxCRDs(t *testing.T) {
	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)
	enricher := flux.NewEnricher(client)

	available := enricher.Available(context.Background())
	if available {
		t.Fatal("expected Available() to return false when Flux CRDs are absent")
	}
}

func TestEnricher_EnrichesHelmRelease(t *testing.T) {
	scheme := runtime.NewScheme()
	helmReleaseGVR := schema.GroupVersionResource{
		Group:    "helm.toolkit.fluxcd.io",
		Version:  "v2",
		Resource: "helmreleases",
	}
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "helm.toolkit.fluxcd.io",
		Version: "v2",
		Kind:    "HelmReleaseList",
	}, &unstructured.UnstructuredList{})

	hr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name":      "my-release",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"suspend": false,
			},
			"status": map[string]interface{}{
				"lastAppliedRevision": "v2.10.0",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		},
	}

	client := fake.NewSimpleDynamicClient(scheme, hr)
	_ = helmReleaseGVR

	enricher := flux.NewEnricher(client)
	status := enricher.ExtractFluxStatus(hr)

	if !status.Ready {
		t.Error("expected Ready=true")
	}
	if status.LastAppliedRev != "v2.10.0" {
		t.Errorf("expected LastAppliedRev=v2.10.0, got %q", status.LastAppliedRev)
	}
	if status.Suspended {
		t.Error("expected Suspended=false")
	}
}

func TestEnricher_DetectsSuspended(t *testing.T) {
	hr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name":      "my-release",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"suspend": true,
			},
			"status": map[string]interface{}{},
		},
	}

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)
	enricher := flux.NewEnricher(client)
	status := enricher.ExtractFluxStatus(hr)

	if !status.Suspended {
		t.Error("expected Suspended=true")
	}
}

func TestEnricher_EnrichSetsFluxStatus(t *testing.T) {
	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "helm.toolkit.fluxcd.io",
		Version: "v2",
		Kind:    "HelmReleaseList",
	}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "kustomize.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    "KustomizationList",
	}, &unstructured.UnstructuredList{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    "GitRepositoryList",
	}, &unstructured.UnstructuredList{})

	hr := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "helm.toolkit.fluxcd.io/v2",
			"kind":       "HelmRelease",
			"metadata": map[string]interface{}{
				"name":      "my-release",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"suspend": false,
			},
			"status": map[string]interface{}{
				"lastAppliedRevision": "v1.0.0",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Ready",
						"status": "True",
					},
				},
			},
		},
	}

	client := fake.NewSimpleDynamicClient(scheme, hr)
	enricher := flux.NewEnricher(client)

	results := []types.DriftResult{
		{
			ID: types.ResourceIdentifier{
				Namespace: "default",
				Name:      "my-release",
			},
			Source: types.SourceInfo{
				Type: types.SourceHelm,
				FluxRef: &types.FluxRef{
					Kind:      "HelmRelease",
					Name:      "my-release",
					Namespace: "default",
				},
			},
		},
	}

	enricher.Enrich(context.Background(), results)

	if results[0].FluxStatus == nil {
		t.Fatal("expected FluxStatus to be set")
	}
	if !results[0].FluxStatus.Ready {
		t.Error("expected Ready=true after enrichment")
	}
}
