package types

import (
	"testing"
)

func TestResourceIdentifier_String(t *testing.T) {
	id := ResourceIdentifier{
		APIVersion: "apps/v1",
		Kind:       "Deployment",
		Namespace:  "default",
		Name:       "nginx",
	}
	expected := "apps/v1/Deployment/default/nginx"
	if id.String() != expected {
		t.Errorf("got %q, want %q", id.String(), expected)
	}
}

func TestResourceIdentifier_String_ClusterScoped(t *testing.T) {
	id := ResourceIdentifier{
		APIVersion: "rbac.authorization.k8s.io/v1",
		Kind:       "ClusterRole",
		Name:       "admin",
	}
	expected := "rbac.authorization.k8s.io/v1/ClusterRole//admin"
	if id.String() != expected {
		t.Errorf("got %q, want %q", id.String(), expected)
	}
}

func TestDriftResult_ExceedsThreshold(t *testing.T) {
	tests := []struct {
		name      string
		severity  Severity
		threshold Severity
		want      bool
	}{
		{"critical exceeds critical", SeverityCritical, SeverityCritical, true},
		{"warning does not exceed critical", SeverityWarning, SeverityCritical, false},
		{"info does not exceed warning", SeverityInfo, SeverityWarning, false},
		{"warning exceeds warning", SeverityWarning, SeverityWarning, true},
		{"critical exceeds info", SeverityCritical, SeverityInfo, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := DriftResult{Severity: tt.severity}
			if got := r.ExceedsThreshold(tt.threshold); got != tt.want {
				t.Errorf("ExceedsThreshold() = %v, want %v", got, tt.want)
			}
		})
	}
}
