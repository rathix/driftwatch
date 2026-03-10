package renderer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestManifestRenderer_SingleDoc(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "single.yaml")
	content := `apiVersion: v1
kind: Deployment
metadata:
  name: my-app
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	renderer := &ManifestRenderer{}
	objects, err := renderer.Render(context.Background(), filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}
	if objects[0].GetName() != "my-app" {
		t.Fatalf("expected name 'my-app', got '%s'", objects[0].GetName())
	}
}

func TestManifestRenderer_MultiDoc(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "multi.yaml")
	content := `apiVersion: v1
kind: Deployment
metadata:
  name: app1
---
apiVersion: v1
kind: Service
metadata:
  name: app2
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	renderer := &ManifestRenderer{}
	objects, err := renderer.Render(context.Background(), filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objects) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objects))
	}
}

func TestManifestRenderer_SkipsSecrets(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "secrets.yaml")
	content := `apiVersion: v1
kind: Secret
metadata:
  name: my-secret
data:
  password: dGVzdA==
`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	renderer := &ManifestRenderer{SkipSecrets: true}
	objects, err := renderer.Render(context.Background(), filePath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objects) != 0 {
		t.Fatalf("expected 0 objects (secret filtered), got %d", len(objects))
	}
}

func TestManifestRenderer_RejectsOversized(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "oversized.yaml")

	// Create an 11MB file
	content := make([]byte, 11*1024*1024)
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	renderer := &ManifestRenderer{}
	_, err := renderer.Render(context.Background(), filePath)
	if err == nil {
		t.Fatalf("expected error for oversized file, got nil")
	}
}
