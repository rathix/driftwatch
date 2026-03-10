package renderer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestKustomizeRenderer_BasicBuild(t *testing.T) {
	dir := t.TempDir()

	deployYAML := `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
      - name: test
        image: nginx:latest
`
	kustomizationYAML := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- deploy.yaml
`

	if err := os.WriteFile(filepath.Join(dir, "deploy.yaml"), []byte(deployYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(kustomizationYAML), 0644); err != nil {
		t.Fatal(err)
	}

	r := &KustomizeRenderer{}
	objects, err := r.Render(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objects))
	}
	if objects[0].GetKind() != "Deployment" {
		t.Errorf("expected Deployment, got %s", objects[0].GetKind())
	}
}

func TestKustomizeRenderer_SkipsSecrets(t *testing.T) {
	dir := t.TempDir()

	secretYAML := `apiVersion: v1
kind: Secret
metadata:
  name: test-secret
type: Opaque
data:
  key: dmFsdWU=
`
	kustomizationYAML := `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
- secret.yaml
`

	if err := os.WriteFile(filepath.Join(dir, "secret.yaml"), []byte(secretYAML), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(kustomizationYAML), 0644); err != nil {
		t.Fatal(err)
	}

	r := &KustomizeRenderer{SkipSecrets: true}
	objects, err := r.Render(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(objects) != 0 {
		t.Fatalf("expected 0 objects, got %d", len(objects))
	}
}
