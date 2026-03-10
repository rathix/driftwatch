package reporter

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/kennyandries/driftwatch/pkg/types"
)

func TestTerminalReporter_InSync(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewTerminalReporter(&buf, false)

	results := []types.DriftResult{
		{
			ID: types.ResourceIdentifier{
				APIVersion: "v1",
				Kind:       "ConfigMap",
				Namespace:  "default",
				Name:       "app-config",
			},
			Status: types.StatusInSync,
		},
	}

	err := reporter.Report(results)
	if err != nil {
		t.Fatalf("Report() error: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestTerminalReporter_Drifted(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewTerminalReporter(&buf, false)

	results := []types.DriftResult{
		{
			ID: types.ResourceIdentifier{
				APIVersion: "v1",
				Kind:       "Deployment",
				Namespace:  "default",
				Name:       "app",
			},
			Status:   types.StatusDrifted,
			Severity: types.SeverityWarning,
			Diffs: []types.FieldDiff{
				{
					Path:     "spec.template.spec.containers[0].image",
					Expected: "app:v1.0",
					Actual:   "app:v1.1",
					Severity: types.SeverityWarning,
				},
			},
			FluxStatus: &types.FluxStatus{
				Ready:      true,
				Suspended:  false,
				Conditions: []string{"Applied"},
			},
		},
	}

	err := reporter.Report(results)
	if err != nil {
		t.Fatalf("Report() error: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected non-empty output")
	}
}

func TestJSONReporter(t *testing.T) {
	var buf bytes.Buffer
	reporter := NewJSONReporter(&buf)

	results := []types.DriftResult{
		{
			ID: types.ResourceIdentifier{
				APIVersion: "v1",
				Kind:       "Deployment",
				Namespace:  "default",
				Name:       "app",
			},
			Status:   types.StatusDrifted,
			Severity: types.SeverityCritical,
			Diffs: []types.FieldDiff{
				{
					Path:     "spec.replicas",
					Expected: "3",
					Actual:   "2",
					Severity: types.SeverityCritical,
				},
			},
		},
	}

	err := reporter.Report(results)
	if err != nil {
		t.Fatalf("Report() error: %v", err)
	}

	var output map[string]interface{}
	err = json.Unmarshal(buf.Bytes(), &output)
	if err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	if _, ok := output["results"]; !ok {
		t.Error("expected 'results' key in JSON output")
	}
	if _, ok := output["metadata"]; !ok {
		t.Error("expected 'metadata' key in JSON output")
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name      string
		results   []types.DriftResult
		threshold types.Severity
		expected  int
	}{
		{
			name:      "no drift",
			results:   []types.DriftResult{{Status: types.StatusInSync}},
			threshold: types.SeverityWarning,
			expected:  0,
		},
		{
			name: "drift below threshold",
			results: []types.DriftResult{
				{Status: types.StatusDrifted, Severity: types.SeverityInfo},
			},
			threshold: types.SeverityWarning,
			expected:  0,
		},
		{
			name: "drift at threshold",
			results: []types.DriftResult{
				{Status: types.StatusDrifted, Severity: types.SeverityWarning},
			},
			threshold: types.SeverityWarning,
			expected:  1,
		},
		{
			name: "drift exceeds threshold",
			results: []types.DriftResult{
				{Status: types.StatusDrifted, Severity: types.SeverityCritical},
			},
			threshold: types.SeverityWarning,
			expected:  1,
		},
		{
			name: "missing exceeds threshold",
			results: []types.DriftResult{
				{Status: types.StatusMissing, Severity: types.SeverityCritical},
			},
			threshold: types.SeverityWarning,
			expected:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCode(tt.results, tt.threshold)
			if got != tt.expected {
				t.Errorf("ExitCode() = %d, want %d", got, tt.expected)
			}
		})
	}
}
