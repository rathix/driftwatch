//go:build integration

package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/config"
	"github.com/kennyandries/driftwatch/pkg/differ"
	"github.com/kennyandries/driftwatch/pkg/discovery"
	"github.com/kennyandries/driftwatch/pkg/pipeline"
	"github.com/kennyandries/driftwatch/pkg/renderer"
	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDiscovery_FleetInfraFixture(t *testing.T) {
	root := fixtureRoot(t)
	sources, err := discovery.Discover(root)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	kustomizeCount := 0
	paths := map[string]bool{}
	for _, s := range sources {
		if s.Type == "kustomize" {
			kustomizeCount++
			paths[s.Path] = true
		}
	}

	if kustomizeCount < 4 {
		t.Errorf("expected at least 4 kustomize sources, got %d", kustomizeCount)
	}

	infraPath := filepath.Join(root, "infrastructure")
	appsPath := filepath.Join(root, "apps")
	if !paths[infraPath] {
		t.Errorf("expected kustomize source at %s", infraPath)
	}
	if !paths[appsPath] {
		t.Errorf("expected kustomize source at %s", appsPath)
	}
}

func TestKustomizeRenderer_FleetInfraFixture(t *testing.T) {
	root := fixtureRoot(t)
	r := &renderer.KustomizeRenderer{}

	objects, err := r.Render(context.Background(), filepath.Join(root, "infrastructure", "reloader"))
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	if len(objects) < 2 {
		t.Fatalf("expected at least 2 objects, got %d", len(objects))
	}

	kinds := map[string]bool{}
	for _, obj := range objects {
		kinds[obj.GetKind()] = true
	}

	if !kinds["Namespace"] {
		t.Error("expected Namespace in rendered output")
	}
	if !kinds["HelmRelease"] {
		t.Error("expected HelmRelease in rendered output")
	}
}

// mockFetcher returns slightly modified live objects to simulate drift.
type mockFetcher struct {
	objects map[string]*unstructured.Unstructured
}

func (m *mockFetcher) Get(_ context.Context, id types.ResourceIdentifier) (*unstructured.Unstructured, error) {
	key := id.String()
	if obj, ok := m.objects[key]; ok {
		return obj, nil
	}
	return nil, nil
}

func TestFullPipeline_WithMockFetcher(t *testing.T) {
	root := fixtureRoot(t)
	r := &renderer.KustomizeRenderer{}

	reloaderPath := filepath.Join(root, "infrastructure", "reloader")
	expected, err := r.Render(context.Background(), reloaderPath)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	// Build mock fetcher with modified live objects
	liveObjects := map[string]*unstructured.Unstructured{}
	for _, obj := range expected {
		live := obj.DeepCopy()
		id := types.ResourceIdentifier{
			APIVersion: obj.GetAPIVersion(),
			Kind:       obj.GetKind(),
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
		}

		// Modify the name to simulate drift (the differ checks expected keys)
		if live.GetKind() == "Namespace" {
			annotations := map[string]string{"drifted": "true"}
			live.SetAnnotations(annotations)
			// Change the namespace name in metadata to trigger a diff
			md, _, _ := unstructured.NestedMap(live.Object, "metadata")
			md["name"] = "reloader-modified"
			_ = unstructured.SetNestedMap(live.Object, md, "metadata")
		}

		liveObjects[id.String()] = live
	}

	p := &pipeline.Pipeline{
		Renderer: r,
		Fetcher:  &mockFetcher{objects: liveObjects},
		Differ:   differ.NewDiffer(differ.DefaultIgnoreFields(), differ.DefaultSeverityRules()),
		Source: types.SourceInfo{
			Type: types.SourceKustomize,
			Path: reloaderPath,
		},
	}

	results, err := p.Run(context.Background(), reloaderPath)
	if err != nil {
		t.Fatalf("Pipeline.Run() error: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("expected at least one drift result")
	}

	// At least one should be drifted (because we added labels to live)
	driftedCount := 0
	for _, r := range results {
		if r.Status == types.StatusDrifted {
			driftedCount++
		}
	}

	if driftedCount == 0 {
		t.Error("expected at least one drifted result")
		for _, r := range results {
			t.Logf("  %s: status=%s severity=%s", r.ID, r.Status, r.Severity)
		}
	}
}

func TestConfig_FleetInfraCompatible(t *testing.T) {
	root := fixtureRoot(t)

	// Create a temporary driftwatch.yaml for the fixture
	cfgContent := []byte(`sources:
  - path: ` + filepath.Join(root, "infrastructure") + `
    type: kustomize
  - path: ` + filepath.Join(root, "apps") + `
    type: kustomize
ignore:
  fields:
    - metadata.managedFields
    - metadata.resourceVersion
    - status
failOn: warning
`)

	tmpFile := filepath.Join(t.TempDir(), "driftwatch.yaml")
	if err := os.WriteFile(tmpFile, cfgContent, 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(tmpFile)
	if err != nil {
		t.Fatalf("config.Load() error: %v", err)
	}

	if len(cfg.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(cfg.Sources))
	}

	if cfg.FailOn != "warning" {
		t.Errorf("expected failOn=warning, got %s", cfg.FailOn)
	}

	// Validate source paths exist
	for _, src := range cfg.Sources {
		if _, err := os.Stat(src.Path); err != nil {
			t.Errorf("source path %s does not exist: %v", src.Path, err)
		}
	}
}
