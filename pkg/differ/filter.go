package differ

import "strings"

// DefaultIgnoreFields returns field path patterns that should be ignored during diffing.
// These are typically cluster-managed metadata fields.
func DefaultIgnoreFields() []string {
	return []string{
		"metadata.managedFields",
		"metadata.resourceVersion",
		"metadata.uid",
		"metadata.generation",
		"metadata.creationTimestamp",
		"metadata.annotations.kubectl.kubernetes.io/last-applied-configuration",
		"status",
	}
}

// shouldIgnore returns true if the field path matches any of the ignore patterns.
// Supports prefix matching and "/*" suffix for wildcard children.
func shouldIgnore(fieldPath string, patterns []string) bool {
	for _, p := range patterns {
		if strings.HasSuffix(p, "/*") {
			prefix := strings.TrimSuffix(p, "/*")
			if fieldPath == prefix || strings.HasPrefix(fieldPath, prefix+".") {
				return true
			}
		} else {
			if fieldPath == p || strings.HasPrefix(fieldPath, p+".") {
				return true
			}
		}
	}
	return false
}
