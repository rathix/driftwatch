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
	APIVersion string `json:"api_version"`
	Kind       string `json:"kind"`
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
}

func (r ResourceIdentifier) String() string {
	return fmt.Sprintf("%s/%s/%s/%s", r.APIVersion, r.Kind, r.Namespace, r.Name)
}

type FluxRef struct {
	Kind      string `json:"kind"`
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type FluxStatus struct {
	Ready          bool     `json:"ready"`
	Suspended      bool     `json:"suspended"`
	LastAppliedRev string   `json:"last_applied_rev"`
	ExpectedRev    string   `json:"expected_rev"`
	Conditions     []string `json:"conditions"`
}

type SourceInfo struct {
	Type    SourceType `json:"type"`
	Path    string     `json:"path"`
	FluxRef *FluxRef   `json:"flux_ref,omitempty"`
}

type ResourcePair struct {
	ID       ResourceIdentifier         `json:"id"`
	Source   SourceInfo                  `json:"source"`
	Expected *unstructured.Unstructured `json:"expected,omitempty"`
	Live     *unstructured.Unstructured `json:"live,omitempty"`
}

type FieldDiff struct {
	Path     string   `json:"path"`
	Expected string   `json:"expected"`
	Actual   string   `json:"actual"`
	Severity Severity `json:"severity"`
}

type DetectionLayer string

const (
	LayerFluxInventory  DetectionLayer = "flux_inventory"
	LayerNamespaceScan  DetectionLayer = "namespace_scan"
	LayerNamespaceAudit DetectionLayer = "namespace_audit"
)

func (d DetectionLayer) String() string {
	return string(d)
}

// SkippedSummary tracks resources filtered out during extras detection.
type SkippedSummary struct {
	HelmManagedResources int `json:"helm_managed_resources"`
	KubeDefaultResources int `json:"kube_default_resources"`
}

type DriftResult struct {
	ID             ResourceIdentifier `json:"id"`
	Source         SourceInfo         `json:"source"`
	Status         DriftStatus        `json:"status"`
	Diffs          []FieldDiff        `json:"diffs,omitempty"`
	Severity       Severity           `json:"severity"`
	FluxStatus     *FluxStatus        `json:"flux_status,omitempty"`
	DetectionLayer DetectionLayer     `json:"detection_layer,omitempty"`
}

func (d DriftResult) ExceedsThreshold(threshold Severity) bool {
	return d.Severity >= threshold
}
