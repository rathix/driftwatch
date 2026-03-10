package discovery

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDiscover_DetectsAllSourceTypes(t *testing.T) {
	root := t.TempDir()

	// Plain Kubernetes manifest
	manifestDir := filepath.Join(root, "manifests")
	must(t, os.MkdirAll(manifestDir, 0o755))
	must(t, os.WriteFile(filepath.Join(manifestDir, "deploy.yaml"), []byte(
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test\n",
	), 0o644))

	// Helm chart
	chartDir := filepath.Join(root, "charts", "myapp")
	must(t, os.MkdirAll(chartDir, 0o755))
	must(t, os.WriteFile(filepath.Join(chartDir, "Chart.yaml"), []byte(
		"apiVersion: v2\nname: myapp\nversion: 0.1.0\n",
	), 0o644))

	// Kustomize overlay
	kustomizeDir := filepath.Join(root, "overlays", "prod")
	must(t, os.MkdirAll(kustomizeDir, 0o755))
	must(t, os.WriteFile(filepath.Join(kustomizeDir, "kustomization.yaml"), []byte(
		"apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - ../../base\n",
	), 0o644))

	sources, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	found := map[string]bool{}
	for _, s := range sources {
		found[s.Type] = true
	}

	for _, want := range []string{"manifest", "helm", "kustomize"} {
		if !found[want] {
			t.Errorf("expected source type %q not found; got %v", want, sources)
		}
	}
}

func TestDiscover_IgnoresHiddenDirs(t *testing.T) {
	root := t.TempDir()

	hiddenDir := filepath.Join(root, ".git", "objects")
	must(t, os.MkdirAll(hiddenDir, 0o755))
	must(t, os.WriteFile(filepath.Join(hiddenDir, "deploy.yaml"), []byte(
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test\n",
	), 0o644))

	sources, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	if len(sources) != 0 {
		t.Errorf("expected no sources, got %v", sources)
	}
}

func TestDiscover_RejectsSymlinks(t *testing.T) {
	root := t.TempDir()

	// Create a real directory with a manifest
	realDir := filepath.Join(root, "real")
	must(t, os.MkdirAll(realDir, 0o755))
	must(t, os.WriteFile(filepath.Join(realDir, "deploy.yaml"), []byte(
		"apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: test\n",
	), 0o644))

	// Create a symlink to the real directory
	symlink := filepath.Join(root, "linked")
	must(t, os.Symlink(realDir, symlink))

	sources, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	// Should only find the manifest in real/, not via the symlink
	for _, s := range sources {
		if filepath.Dir(s.Path) == symlink {
			t.Errorf("symlinked source should not be discovered: %v", s)
		}
	}

	// Should find exactly 1 source (the real one)
	if len(sources) != 1 {
		t.Errorf("expected 1 source, got %d: %v", len(sources), sources)
	}
}

func TestDiscover_DeduplicatesNestedKustomizeDirs(t *testing.T) {
	root := t.TempDir()

	kustomContent := "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - reloader/\n  - cert-manager/\n"

	// Parent kustomization
	infraDir := filepath.Join(root, "infrastructure")
	must(t, os.MkdirAll(infraDir, 0o755))
	must(t, os.WriteFile(filepath.Join(infraDir, "kustomization.yaml"), []byte(kustomContent), 0o644))

	// Child kustomizations (nested under infrastructure/)
	for _, sub := range []string{"reloader", "cert-manager"} {
		subDir := filepath.Join(infraDir, sub)
		must(t, os.MkdirAll(subDir, 0o755))
		must(t, os.WriteFile(filepath.Join(subDir, "kustomization.yaml"), []byte(
			"apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n  - deploy.yaml\n",
		), 0o644))
	}

	sources, err := Discover(root)
	if err != nil {
		t.Fatalf("Discover() error: %v", err)
	}

	// Should only find the parent kustomize dir, not the children.
	var kustomizeSources []DiscoveredSource
	for _, s := range sources {
		if s.Type == "kustomize" {
			kustomizeSources = append(kustomizeSources, s)
		}
	}

	if len(kustomizeSources) != 1 {
		t.Errorf("expected 1 kustomize source, got %d: %v", len(kustomizeSources), kustomizeSources)
	}
	if len(kustomizeSources) > 0 && kustomizeSources[0].Path != infraDir {
		t.Errorf("expected kustomize path %s, got %s", infraDir, kustomizeSources[0].Path)
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
