package integration

import (
	"context"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/discovery"
	"github.com/kennyandries/driftwatch/pkg/renderer"
)

func fixtureRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot determine test file path")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "fleet-infra")
}

func TestDiscovery_FleetInfra(t *testing.T) {
	root := fixtureRoot(t)
	sources, err := discovery.Discover(root)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	kustomizeCount := 0
	for _, s := range sources {
		if s.Type == "kustomize" {
			kustomizeCount++
		}
	}

	// infrastructure, infrastructure/reloader, apps, apps/uptime-kuma
	if kustomizeCount < 4 {
		t.Errorf("expected at least 4 kustomize sources, got %d", kustomizeCount)
		for _, s := range sources {
			t.Logf("  %s: %s", s.Type, s.Path)
		}
	}
}

func TestKustomizeRender_FleetInfra(t *testing.T) {
	root := fixtureRoot(t)
	r := &renderer.KustomizeRenderer{}

	// Render infrastructure/reloader
	reloaderPath := filepath.Join(root, "infrastructure", "reloader")
	objects, err := r.Render(context.Background(), reloaderPath)
	if err != nil {
		t.Fatalf("Render() error: %v", err)
	}

	if len(objects) == 0 {
		t.Fatal("expected at least one rendered object")
	}

	foundNamespace := false
	foundHelmRelease := false
	for _, obj := range objects {
		switch obj.GetKind() {
		case "Namespace":
			if obj.GetName() == "reloader" {
				foundNamespace = true
			}
		case "HelmRelease":
			if obj.GetName() == "reloader" {
				foundHelmRelease = true
			}
		}
	}

	if !foundNamespace {
		t.Error("expected Namespace 'reloader' in rendered output")
	}
	if !foundHelmRelease {
		t.Error("expected HelmRelease 'reloader' in rendered output")
	}
}
