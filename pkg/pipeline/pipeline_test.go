package pipeline

import (
	"context"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type mockRenderer struct {
	objects []*unstructured.Unstructured
}

func (m *mockRenderer) Render(_ context.Context, _ string) ([]*unstructured.Unstructured, error) {
	return m.objects, nil
}

type mockFetcher struct {
	live map[string]*unstructured.Unstructured
}

func (m *mockFetcher) Get(_ context.Context, id types.ResourceIdentifier) (*unstructured.Unstructured, error) {
	if obj, ok := m.live[id.String()]; ok {
		return obj, nil
	}
	return nil, nil
}

func TestPipeline_DetectsDrift(t *testing.T) {
	expected := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "nginx",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": int64(3),
			},
		},
	}

	live := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "nginx",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
			},
		},
	}

	id := types.ResourceIdentifier{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  "default",
		Name:       "nginx",
	}

	p := &Pipeline{
		Renderer: &mockRenderer{objects: []*unstructured.Unstructured{expected}},
		Fetcher:  &mockFetcher{live: map[string]*unstructured.Unstructured{id.String(): live}},
	}

	results, err := p.Run(context.Background(), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != types.StatusDrifted {
		t.Errorf("expected StatusDrifted, got %s", results[0].Status)
	}
}

func TestPipeline_DetectsMissing(t *testing.T) {
	expected := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "my-config",
				"namespace": "default",
			},
			"data": map[string]interface{}{
				"key": "value",
			},
		},
	}

	p := &Pipeline{
		Renderer: &mockRenderer{objects: []*unstructured.Unstructured{expected}},
		Fetcher:  &mockFetcher{live: map[string]*unstructured.Unstructured{}},
	}

	results, err := p.Run(context.Background(), ".")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Status != types.StatusMissing {
		t.Errorf("expected StatusMissing, got %s", results[0].Status)
	}
}
