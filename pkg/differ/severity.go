package differ

import (
	"strings"

	"github.com/kennyandries/driftwatch/pkg/types"
)

// SeverityRule maps a field path pattern to a severity level.
type SeverityRule struct {
	Pattern  string
	Severity types.Severity
}

// DefaultSeverityRules returns the built-in severity classification rules.
func DefaultSeverityRules() []SeverityRule {
	return []SeverityRule{
		// Critical: container images
		{Pattern: "spec.template.spec.containers.*.image", Severity: types.SeverityCritical},
		{Pattern: "spec.template.spec.initContainers.*.image", Severity: types.SeverityCritical},
		// Critical: RBAC rules
		{Pattern: "rules", Severity: types.SeverityCritical},
		{Pattern: "rules.*", Severity: types.SeverityCritical},
		// Critical: ports
		{Pattern: "spec.template.spec.containers.*.ports.*", Severity: types.SeverityCritical},
		{Pattern: "spec.ports.*", Severity: types.SeverityCritical},
		// Critical: serviceAccountName
		{Pattern: "spec.template.spec.serviceAccountName", Severity: types.SeverityCritical},
		// Critical: securityContext
		{Pattern: "spec.template.spec.securityContext.*", Severity: types.SeverityCritical},
		{Pattern: "spec.template.spec.containers.*.securityContext.*", Severity: types.SeverityCritical},
		// Warning: replicas
		{Pattern: "spec.replicas", Severity: types.SeverityWarning},
		// Warning: resources
		{Pattern: "spec.template.spec.containers.*.resources.*", Severity: types.SeverityWarning},
	}
}

// classifyField returns the highest severity matching any rule, or SeverityInfo if none match.
func classifyField(fieldPath string, rules []SeverityRule) types.Severity {
	best := types.SeverityInfo
	for _, r := range rules {
		if matchPattern(r.Pattern, fieldPath) && r.Severity > best {
			best = r.Severity
		}
	}
	return best
}

// matchPattern checks if a field path matches a pattern with "*" wildcards.
// Both pattern and path are dot-separated.
func matchPattern(pattern, path string) bool {
	return matchParts(strings.Split(pattern, "."), strings.Split(path, "."))
}

// matchParts recursively matches pattern parts against path parts.
func matchParts(pattern, path []string) bool {
	if len(pattern) == 0 && len(path) == 0 {
		return true
	}
	if len(pattern) == 0 || len(path) == 0 {
		return false
	}
	if pattern[0] == "*" || pattern[0] == path[0] {
		return matchParts(pattern[1:], path[1:])
	}
	return false
}
