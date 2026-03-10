package renderer

import (
	"bytes"
	"context"
	"fmt"
	"log"

	"helm.sh/helm/v3/pkg/action"
	"helm.sh/helm/v3/pkg/chart/loader"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

type HelmRenderer struct {
	SkipSecrets    bool
	ReleaseName    string
	Namespace      string
	ValueOverrides map[string]interface{}
}

func (h *HelmRenderer) Render(ctx context.Context, chartPath string) ([]*unstructured.Unstructured, error) {
	chart, err := loader.Load(chartPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load chart: %w", err)
	}

	cfg := &action.Configuration{}
	if err := cfg.Init(nil, "", "", func(format string, v ...interface{}) {
		log.Printf(format, v...)
	}); err != nil {
		return nil, fmt.Errorf("failed to init helm config: %w", err)
	}

	install := action.NewInstall(cfg)
	install.DryRun = true
	install.ClientOnly = true
	install.Replace = true

	releaseName := h.ReleaseName
	if releaseName == "" {
		releaseName = chart.Metadata.Name
	}
	install.ReleaseName = releaseName

	namespace := h.Namespace
	if namespace == "" {
		namespace = "default"
	}
	install.Namespace = namespace

	vals := chart.Values
	if vals == nil {
		vals = map[string]interface{}{}
	}
	for k, v := range h.ValueOverrides {
		vals[k] = v
	}

	rel, err := install.Run(chart, vals)
	if err != nil {
		return nil, fmt.Errorf("helm render failed: %w", err)
	}

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewBufferString(rel.Manifest), 4096)
	var objects []*unstructured.Unstructured

	for {
		obj := &unstructured.Unstructured{}
		if err := decoder.Decode(obj); err != nil {
			break
		}
		if obj.GetAPIVersion() == "" || obj.GetKind() == "" {
			continue
		}
		if h.SkipSecrets && obj.GetKind() == "Secret" {
			continue
		}
		objects = append(objects, obj)
	}

	return objects, nil
}
