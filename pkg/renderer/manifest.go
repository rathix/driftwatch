package renderer

import (
	"bytes"
	"context"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const maxFileSize = 10 * 1024 * 1024

type ManifestRenderer struct {
	SkipSecrets bool
}

func (m *ManifestRenderer) Render(ctx context.Context, path string) ([]*unstructured.Unstructured, error) {
	// Check if file is a symlink
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return nil, fmt.Errorf("symlinks not allowed")
	}

	// Check file size
	if info.Size() > int64(maxFileSize) {
		return nil, fmt.Errorf("file size exceeds %d bytes", maxFileSize)
	}

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Decode YAML documents
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var objects []*unstructured.Unstructured

	for {
		obj := &unstructured.Unstructured{}
		err := decoder.Decode(obj)
		if err != nil {
			break
		}

		// Skip documents without apiVersion/kind
		if obj.GetAPIVersion() == "" || obj.GetKind() == "" {
			continue
		}

		// Skip Secrets if SkipSecrets is true
		if m.SkipSecrets && obj.GetKind() == "Secret" {
			continue
		}

		objects = append(objects, obj)
	}

	return objects, nil
}
