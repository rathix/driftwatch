package renderer

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestHelmRenderer_LocalChart(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "Chart.yaml"), `apiVersion: v2
name: mychart
version: 0.1.0
`)
	writeFile(t, filepath.Join(dir, "values.yaml"), `replicaCount: 1
image: nginx:latest
`)
	writeFile(t, filepath.Join(dir, "templates", "deployment.yaml"), `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}-deployment
spec:
  replicas: {{ .Values.replicaCount }}
  selector:
    matchLabels:
      app: {{ .Release.Name }}
  template:
    metadata:
      labels:
        app: {{ .Release.Name }}
    spec:
      containers:
      - name: app
        image: {{ .Values.image }}
`)

	r := &HelmRenderer{ReleaseName: "test"}
	objs, err := r.Render(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) < 1 {
		t.Fatalf("expected at least 1 object, got %d", len(objs))
	}
	var found bool
	for _, o := range objs {
		if o.GetKind() == "Deployment" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected a Deployment in rendered objects")
	}
}

func TestHelmRenderer_SkipsSecrets(t *testing.T) {
	dir := t.TempDir()

	writeFile(t, filepath.Join(dir, "Chart.yaml"), `apiVersion: v2
name: mysecret
version: 0.1.0
`)
	writeFile(t, filepath.Join(dir, "values.yaml"), ``)
	writeFile(t, filepath.Join(dir, "templates", "secret.yaml"), `apiVersion: v1
kind: Secret
metadata:
  name: {{ .Release.Name }}-secret
type: Opaque
data:
  key: dmFsdWU=
`)

	r := &HelmRenderer{SkipSecrets: true, ReleaseName: "test"}
	objs, err := r.Render(context.Background(), dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 0 {
		t.Fatalf("expected 0 objects (secrets skipped), got %d", len(objs))
	}
}
