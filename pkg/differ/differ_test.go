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

func TestDiff_QuantityNormalization(t *testing.T) {
	d := NewDiffer(DefaultIgnoreFields(), DefaultSeverityRules())

	tests := []struct {
		name     string
		expVal   interface{}
		liveVal  interface{}
		wantSync bool
	}{
		{"cpu 0.2 vs 200m", "0.2", "200m", true},
		{"cpu 1000m vs 1", "1000m", "1", true},
		{"memory 128Mi vs 128Mi", "128Mi", "128Mi", true},
		{"different values", "100m", "200m", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pair := types.ResourcePair{
				ID: types.ResourceIdentifier{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Namespace:  "default",
					Name:       "test",
				},
				Expected: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name": "app",
											"resources": map[string]interface{}{
												"requests": map[string]interface{}{
													"cpu": tt.expVal,
												},
											},
										},
									},
								},
							},
						},
					},
				},
				Live: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"spec": map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name": "app",
											"resources": map[string]interface{}{
												"requests": map[string]interface{}{
													"cpu": tt.liveVal,
												},
											},
										},
									},
								},
							},
						},
					},
				},
			}

			result := d.Diff(pair)
			if tt.wantSync && result.Status != types.StatusInSync {
				t.Errorf("expected InSync, got %s with diffs: %v", result.Status, result.Diffs)
			}
			if !tt.wantSync && result.Status == types.StatusInSync {
				t.Errorf("expected drift, got InSync")
			}
		})
	}
}

func TestDiff_NamedSliceComparison(t *testing.T) {
	d := NewDiffer(nil, nil)

	// Expected has 2 env vars; live has an extra injected one at the start
	pair := types.ResourcePair{
		ID: types.ResourceIdentifier{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Namespace:  "default",
			Name:       "test",
		},
		Expected: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name": "app",
							"env": []interface{}{
								map[string]interface{}{"name": "FOO", "value": "bar"},
								map[string]interface{}{"name": "BAZ", "value": "qux"},
							},
						},
					},
				},
			},
		},
		Live: &unstructured.Unstructured{
			Object: map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name": "app",
							"env": []interface{}{
								map[string]interface{}{"name": "STAKATER_HOMEPAGE_CONFIGMAP", "value": "injected"},
								map[string]interface{}{"name": "FOO", "value": "bar"},
								map[string]interface{}{"name": "BAZ", "value": "qux"},
							},
						},
					},
				},
			},
		},
	}

	result := d.Diff(pair)

	if result.Status != types.StatusInSync {
		t.Errorf("expected InSync (injected env should not cause drift), got %s with diffs: %v", result.Status, result.Diffs)
	}
}
