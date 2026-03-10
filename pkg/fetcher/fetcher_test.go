package fetcher_test

import (
	"context"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/fetcher"
	"github.com/kennyandries/driftwatch/pkg/types"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/apimachinery/pkg/runtime"
)

type staticMapper struct {
	gvr schema.GroupVersionResource
}

func (m *staticMapper) ResourceFor(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	return m.gvr, nil
}

func TestFetcher_GetLiveResource(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"})
	obj.SetName("my-deploy")
	obj.SetNamespace("default")

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme, obj)

	f := fetcher.NewFetcher(client, &staticMapper{gvr: gvr})

	id := types.ResourceIdentifier{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  "default",
		Name:       "my-deploy",
	}

	result, err := f.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result == nil {
		t.Fatal("expected result, got nil")
	}
	if result.GetName() != "my-deploy" {
		t.Errorf("expected name %q, got %q", "my-deploy", result.GetName())
	}
}

func TestFetcher_MissingResource(t *testing.T) {
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	scheme := runtime.NewScheme()
	client := fake.NewSimpleDynamicClient(scheme)

	f := fetcher.NewFetcher(client, &staticMapper{gvr: gvr})

	id := types.ResourceIdentifier{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  "default",
		Name:       "nonexistent",
	}

	result, err := f.Get(context.Background(), id)
	if err != nil {
		t.Fatalf("expected no error for missing resource, got %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result for missing resource, got %v", result)
	}
}

// Ensure metav1 is used (suppress unused import).
var _ = metav1.Now
