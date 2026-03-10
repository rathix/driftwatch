package differ

import (
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDiff_InSync(t *testing.T) {
	d := NewDiffer(DefaultIgnoreFields(), DefaultSeverityRules())
	pair := types.ResourcePair{
		ID: types.ResourceIdentifier{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "default",
			Name:       "nginx",
		},
		Expected: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "nginx",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
			},
		},
		Live: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "nginx",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
			},
		},
	}

	result := d.Diff(pair)

	if result.Status != types.StatusInSync {
		t.Errorf("expected StatusInSync, got %s", result.Status)
	}
	if len(result.Diffs) != 0 {
		t.Errorf("expected no diffs, got %d: %v", len(result.Diffs), result.Diffs)
	}
}

func TestDiff_Drifted(t *testing.T) {
	d := NewDiffer(DefaultIgnoreFields(), DefaultSeverityRules())
	pair := types.ResourcePair{
		ID: types.ResourceIdentifier{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "default",
			Name:       "nginx",
		},
		Expected: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "nginx",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
			},
		},
		Live: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "nginx",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": int64(5),
				},
			},
		},
	}

	result := d.Diff(pair)

	if result.Status != types.StatusDrifted {
		t.Fatalf("expected StatusDrifted, got %s", result.Status)
	}
	if len(result.Diffs) != 1 {
		t.Fatalf("expected 1 diff, got %d", len(result.Diffs))
	}
	if result.Diffs[0].Path != "spec.replicas" {
		t.Errorf("expected path spec.replicas, got %s", result.Diffs[0].Path)
	}
	if result.Diffs[0].Expected != "3" {
		t.Errorf("expected value '3', got %q", result.Diffs[0].Expected)
	}
	if result.Diffs[0].Actual != "5" {
		t.Errorf("expected actual '5', got %q", result.Diffs[0].Actual)
	}
}

func TestDiff_Missing(t *testing.T) {
	d := NewDiffer(DefaultIgnoreFields(), DefaultSeverityRules())
	pair := types.ResourcePair{
		ID: types.ResourceIdentifier{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "default",
			Name:       "nginx",
		},
		Expected: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
			},
		},
		Live: nil,
	}

	result := d.Diff(pair)

	if result.Status != types.StatusMissing {
		t.Errorf("expected StatusMissing, got %s", result.Status)
	}
	if result.Severity != types.SeverityCritical {
		t.Errorf("expected SeverityCritical, got %s", result.Severity)
	}
}

func TestDiff_IgnoresManagedFields(t *testing.T) {
	d := NewDiffer(DefaultIgnoreFields(), DefaultSeverityRules())
	pair := types.ResourcePair{
		ID: types.ResourceIdentifier{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "default",
			Name:       "nginx",
		},
		Expected: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":      "nginx",
					"namespace": "default",
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
			},
		},
		Live: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
				"metadata": map[string]interface{}{
					"name":            "nginx",
					"namespace":       "default",
					"managedFields":   []interface{}{"something"},
					"resourceVersion": "12345",
					"uid":             "abc-123",
				},
				"spec": map[string]interface{}{
					"replicas": int64(3),
				},
			},
		},
	}

	result := d.Diff(pair)

	if result.Status != types.StatusInSync {
		t.Errorf("expected StatusInSync, got %s", result.Status)
	}
	if len(result.Diffs) != 0 {
		t.Errorf("expected no diffs, got %d: %v", len(result.Diffs), result.Diffs)
	}
}

func TestSeverity_ImageChange_IsCritical(t *testing.T) {
	rules := DefaultSeverityRules()
	sev := classifyField("spec.template.spec.containers.0.image", rules)
	if sev != types.SeverityCritical {
		t.Errorf("expected SeverityCritical for image change, got %s", sev)
	}
}

func TestSeverity_ReplicaChange_IsWarning(t *testing.T) {
	rules := DefaultSeverityRules()
	sev := classifyField("spec.replicas", rules)
	if sev != types.SeverityWarning {
		t.Errorf("expected SeverityWarning for replicas change, got %s", sev)
	}
}
