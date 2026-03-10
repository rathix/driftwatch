package types

import (
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type Severity int

const (
	SeverityInfo Severity = iota
	SeverityWarning
	SeverityCritical
)

func (s Severity) String() string {
	switch s {
	case SeverityCritical:
		return "critical"
	case SeverityWarning:
		return "warning"
	default:
		return "info"
	}
}

func ParseSeverity(s string) (Severity, error) {
	switch s {
	case "critical":
		return SeverityCritical, nil
	case "warning":
		return SeverityWarning, nil
	case "info":
		return SeverityInfo, nil
	default:
		return SeverityInfo, fmt.Errorf("unknown severity: %q", s)
	}
}

type DriftStatus string

const (
	StatusInSync  DriftStatus = "in_sync"
	StatusDrifted DriftStatus = "drifted"
	StatusMissing DriftStatus = "missing"
	StatusExtra   DriftStatus = "extra"
)

type SourceType string

const (
	SourceManifest  SourceType = "manifest"
	SourceHelm      SourceType = "helm"
	SourceKustomize SourceType = "kustomize"
)

type ResourceIdentifier struct {
	APIVersion string
	Kind       string
	Namespace  string
	Name       string
}

func (r ResourceIdentifier) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", r.APIVersion, r.Kind, r.Namespace, r.Name)
}

type FluxRef struct {
	Kind      string
	Name      string
	Namespace string
}

type FluxStatus struct {
	Ready          bool
	Suspended      bool
	LastAppliedRev string
	ExpectedRev    string
	Conditions     []string
}

type SourceInfo struct {
	Type    SourceType
	Path    string
	FluxRef *FluxRef
}

type ResourcePair struct {
	ID       ResourceIdentifier
	Source   SourceInfo
	Expected *unstructured.Unstructured
	Live     *unstructured.Unstructured
}

type FieldDiff struct {
	Path     string
	Expected string
	Actual   string
	Severity Severity
}

type DriftResult struct {
	ID         ResourceIdentifier
	Source     SourceInfo
	Status     DriftStatus
	Diffs      []FieldDiff
	Severity   Severity
	FluxStatus *FluxStatus
}

func (d DriftResult) ExceedsThreshold(threshold Severity) bool {
	return d.Severity >= threshold
}
