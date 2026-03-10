package renderer

import (
	"context"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Renderer interface {
	Render(ctx context.Context, path string) ([]*unstructured.Unstructured, error)
}
