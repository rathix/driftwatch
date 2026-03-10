package pipeline

import (
	"context"

	"github.com/kennyandries/driftwatch/pkg/differ"
	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RendererInterface renders expected Kubernetes objects from a source path.
type RendererInterface interface {
	Render(ctx context.Context, path string) ([]*unstructured.Unstructured, error)
}

// FetcherInterface fetches live Kubernetes objects by identifier.
type FetcherInterface interface {
	Get(ctx context.Context, id types.ResourceIdentifier) (*unstructured.Unstructured, error)
}

// Pipeline orchestrates rendering, fetching, and diffing.
type Pipeline struct {
	Renderer      RendererInterface
	Fetcher       FetcherInterface
	Differ        *differ.Differ
	IgnoreFields  []string
	SeverityRules []differ.SeverityRule
	Source        types.SourceInfo
}

// Run renders expected objects, fetches live state, and diffs each pair.
func (p *Pipeline) Run(ctx context.Context, path string) ([]types.DriftResult, error) {
	objects, err := p.Renderer.Render(ctx, path)
	if err != nil {
		return nil, err
	}

	d := p.Differ
	if d == nil {
		ignoreFields := p.IgnoreFields
		if len(ignoreFields) == 0 {
			ignoreFields = differ.DefaultIgnoreFields()
		}
		severityRules := p.SeverityRules
		if len(severityRules) == 0 {
			severityRules = differ.DefaultSeverityRules()
		}
		d = differ.NewDiffer(ignoreFields, severityRules)
	}

	var results []types.DriftResult

	for _, obj := range objects {
		// Redact secrets from expected
		redacted := differ.RedactSecretValues(obj.Object)
		obj = &unstructured.Unstructured{Object: redacted}

		id := types.ResourceIdentifier{
			APIVersion: obj.GetAPIVersion(),
			Kind:       obj.GetKind(),
			Namespace:  obj.GetNamespace(),
			Name:       obj.GetName(),
		}

		live, err := p.Fetcher.Get(ctx, id)
		if err != nil {
			return nil, err
		}

		if live != nil {
			liveRedacted := differ.RedactSecretValues(live.Object)
			live = &unstructured.Unstructured{Object: liveRedacted}
		}

		pair := types.ResourcePair{
			ID:       id,
			Source:   p.Source,
			Expected: obj,
			Live:     live,
		}

		result := d.Diff(pair)
		results = append(results, result)
	}

	return results, nil
}
