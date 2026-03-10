package discovery

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const maxFileSize = 10 * 1024 * 1024 // 10 MB

// DiscoveredSource represents a discovered Kubernetes source.
type DiscoveredSource struct {
	Type string // "manifest", "helm", or "kustomize"
	Path string
}

// Discover walks root and returns all discovered Kubernetes sources.
// Pass 1: find kustomization.yaml and Chart.yaml directories.
// Pass 2: find standalone manifests not inside kustomize/helm dirs.
func Discover(root string) ([]DiscoveredSource, error) {
	var sources []DiscoveredSource

	kustomizeDirs := map[string]bool{}
	helmDirs := map[string]bool{}

	// Pass 1: find kustomize and helm directories.
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() && isHidden(d.Name()) && path != root {
			return filepath.SkipDir
		}

		// Skip symlinks.
		if d.Type()&fs.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		name := d.Name()

		if name == "kustomization.yaml" || name == "kustomization.yml" {
			if isPlainKustomization(path) {
				dir := filepath.Dir(path)
				kustomizeDirs[dir] = true
				sources = append(sources, DiscoveredSource{Type: "kustomize", Path: dir})
			}
		}

		if name == "Chart.yaml" || name == "Chart.yml" {
			dir := filepath.Dir(path)
			helmDirs[dir] = true
			sources = append(sources, DiscoveredSource{Type: "helm", Path: dir})
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Pass 2: find standalone manifests.
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() && isHidden(d.Name()) && path != root {
			return filepath.SkipDir
		}

		if d.Type()&fs.ModeSymlink != 0 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(path)
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		dir := filepath.Dir(path)

		// Skip files inside kustomize or helm directories.
		if isInsideManagedDir(dir, kustomizeDirs, helmDirs) {
			return nil
		}

		if isKubernetesManifest(path) {
			sources = append(sources, DiscoveredSource{Type: "manifest", Path: path})
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return sources, nil
}

func isHidden(name string) bool {
	return strings.HasPrefix(name, ".")
}

func isInsideManagedDir(dir string, kustomizeDirs, helmDirs map[string]bool) bool {
	for d := range kustomizeDirs {
		if dir == d || strings.HasPrefix(dir, d+string(filepath.Separator)) {
			return true
		}
	}
	for d := range helmDirs {
		if dir == d || strings.HasPrefix(dir, d+string(filepath.Separator)) {
			return true
		}
	}
	return false
}

func isPlainKustomization(path string) bool {
	data, err := readFileSafe(path)
	if err != nil {
		return false
	}

	var doc struct {
		APIVersion string `yaml:"apiVersion"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}

	return strings.HasPrefix(doc.APIVersion, "kustomize.config.k8s.io/")
}

func isKubernetesManifest(path string) bool {
	data, err := readFileSafe(path)
	if err != nil {
		return false
	}

	var doc struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return false
	}

	return doc.APIVersion != "" && doc.Kind != ""
}

func readFileSafe(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}

	// Reject symlinks.
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, fs.ErrPermission
	}

	// Reject files exceeding size limit.
	if info.Size() > maxFileSize {
		return nil, fs.ErrPermission
	}

	return os.ReadFile(path)
}
