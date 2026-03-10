package renderer

import (
	"bytes"
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

type KustomizeRenderer struct {
	SkipSecrets bool
}

func (k *KustomizeRenderer) Render(ctx context.Context, path string) ([]*unstructured.Unstructured, error) {
	fSys := filesys.MakeFsOnDisk()

	opts := krusty.MakeDefaultOptions()
	opts.PluginConfig = types.DisabledPluginConfig()

	kustomizer := krusty.MakeKustomizer(opts)
	resMap, err := kustomizer.Run(fSys, path)
	if err != nil {
		return nil, fmt.Errorf("kustomize run failed: %w", err)
	}

	yamlBytes, err := resMap.AsYaml()
	if err != nil {
		return nil, fmt.Errorf("failed to marshal kustomize output: %w", err)
	}

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(yamlBytes), 4096)
	var objects []*unstructured.Unstructured

	for {
		obj := &unstructured.Unstructured{}
		if err := decoder.Decode(obj); err != nil {
			break
		}

		if obj.GetAPIVersion() == "" || obj.GetKind() == "" {
			continue
		}

		if k.SkipSecrets && obj.GetKind() == "Secret" {
			continue
		}

		objects = append(objects, obj)
	}

	return objects, nil
}
