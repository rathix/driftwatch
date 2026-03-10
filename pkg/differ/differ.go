package differ

import (
	"fmt"
	"strings"

	"github.com/kennyandries/driftwatch/pkg/types"
)

// Differ compares expected and live Kubernetes resources to detect drift.
type Differ struct {
	ignoreFields  []string
	severityRules []SeverityRule
}

// NewDiffer creates a Differ with the given ignore patterns and severity rules.
func NewDiffer(ignoreFields []string, severityRules []SeverityRule) *Differ {
	return &Differ{
		ignoreFields:  ignoreFields,
		severityRules: severityRules,
	}
}

// Diff compares a ResourcePair and returns a DriftResult.
func (d *Differ) Diff(pair types.ResourcePair) types.DriftResult {
	result := types.DriftResult{
		ID:     pair.ID,
		Source: pair.Source,
	}

	// Missing: expected exists but live does not
	if pair.Live == nil {
		result.Status = types.StatusMissing
		result.Severity = types.SeverityCritical
		return result
	}

	// Extra: live exists but expected does not
	if pair.Expected == nil {
		result.Status = types.StatusExtra
		result.Severity = types.SeverityWarning
		return result
	}

	diffs := d.compareObjects(pair.Expected.Object, pair.Live.Object, "")

	if len(diffs) == 0 {
		result.Status = types.StatusInSync
		result.Severity = types.SeverityInfo
		return result
	}

	result.Status = types.StatusDrifted
	result.Diffs = diffs

	// Overall severity is the max across all field diffs
	maxSev := types.SeverityInfo
	for _, fd := range diffs {
		if fd.Severity > maxSev {
			maxSev = fd.Severity
		}
	}
	result.Severity = maxSev
	return result
}

// compareObjects recursively compares two maps, only checking keys present in expected.
func (d *Differ) compareObjects(expected, live map[string]interface{}, prefix string) []types.FieldDiff {
	var diffs []types.FieldDiff

	for key, expVal := range expected {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}

		if shouldIgnore(path, d.ignoreFields) {
			continue
		}

		liveVal, exists := live[key]
		if !exists {
			diffs = append(diffs, types.FieldDiff{
				Path:     path,
				Expected: formatValue(expVal),
				Actual:   "<missing>",
				Severity: classifyField(path, d.severityRules),
			})
			continue
		}

		diffs = append(diffs, d.compareValues(expVal, liveVal, path)...)
	}

	return diffs
}

// compareValues compares two values at a given path.
func (d *Differ) compareValues(expected, live interface{}, path string) []types.FieldDiff {
	switch exp := expected.(type) {
	case map[string]interface{}:
		if liveMap, ok := live.(map[string]interface{}); ok {
			return d.compareObjects(exp, liveMap, path)
		}
		return []types.FieldDiff{{
			Path:     path,
			Expected: formatValue(expected),
			Actual:   formatValue(live),
			Severity: classifyField(path, d.severityRules),
		}}

	case []interface{}:
		if liveSlice, ok := live.([]interface{}); ok {
			return d.compareSlices(exp, liveSlice, path)
		}
		return []types.FieldDiff{{
			Path:     path,
			Expected: formatValue(expected),
			Actual:   formatValue(live),
			Severity: classifyField(path, d.severityRules),
		}}

	default:
		if formatValue(expected) != formatValue(live) {
			return []types.FieldDiff{{
				Path:     path,
				Expected: formatValue(expected),
				Actual:   formatValue(live),
				Severity: classifyField(path, d.severityRules),
			}}
		}
		return nil
	}
}

// compareSlices does index-based comparison of two slices.
func (d *Differ) compareSlices(expected, live []interface{}, prefix string) []types.FieldDiff {
	var diffs []types.FieldDiff

	for i := 0; i < len(expected); i++ {
		path := fmt.Sprintf("%s.%d", prefix, i)
		if i >= len(live) {
			diffs = append(diffs, types.FieldDiff{
				Path:     path,
				Expected: formatValue(expected[i]),
				Actual:   "<missing>",
				Severity: classifyField(path, d.severityRules),
			})
			continue
		}
		diffs = append(diffs, d.compareValues(expected[i], live[i], path)...)
	}

	return diffs
}

// RedactSecretValues removes sensitive data from a Kubernetes object map.
// For Secret resources, it clears .data and .stringData.
// It also redacts values for keys matching sensitive patterns.
func RedactSecretValues(obj map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(obj))
	for k, v := range obj {
		result[k] = v
	}

	// If this is a Secret, redact data and stringData
	if kind, _ := result["kind"].(string); kind == "Secret" {
		if _, ok := result["data"]; ok {
			result["data"] = map[string]interface{}{"<redacted>": "<redacted>"}
		}
		if _, ok := result["stringData"]; ok {
			result["stringData"] = map[string]interface{}{"<redacted>": "<redacted>"}
		}
	}

	// Redact fields matching sensitive patterns
	redactSensitiveFields(result, "")
	return result
}

var sensitivePatterns = []string{"secret", "password", "token", "key", "credential"}

func redactSensitiveFields(obj map[string]interface{}, prefix string) {
	for k, v := range obj {
		lower := strings.ToLower(k)
		isSensitive := false
		for _, p := range sensitivePatterns {
			if strings.Contains(lower, p) {
				isSensitive = true
				break
			}
		}

		if isSensitive {
			if _, isMap := v.(map[string]interface{}); !isMap {
				if _, isSlice := v.([]interface{}); !isSlice {
					obj[k] = "<redacted>"
				}
			}
		}

		if nested, ok := v.(map[string]interface{}); ok {
			redactSensitiveFields(nested, prefix+k+".")
		}
	}
}

func formatValue(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", v)
}
