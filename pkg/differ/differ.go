package differ

import (
	"fmt"
	"strings"

	"github.com/kennyandries/driftwatch/pkg/types"
	"k8s.io/apimachinery/pkg/api/resource"
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
		expStr := formatValue(expected)
		liveStr := formatValue(live)
		if expStr != liveStr && !quantitiesEqual(expStr, liveStr) {
			return []types.FieldDiff{{
				Path:     path,
				Expected: expStr,
				Actual:   liveStr,
				Severity: classifyField(path, d.severityRules),
			}}
		}
		return nil
	}
}

// compareSlices compares two slices. For slices of maps with a "name" key,
// it matches by name instead of index to avoid cascading false diffs when
// Kubernetes injects extra items.
func (d *Differ) compareSlices(expected, live []interface{}, prefix string) []types.FieldDiff {
	if isNamedList(expected) && isNamedList(live) {
		return d.compareNamedSlices(expected, live, prefix)
	}

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

// isNamedList returns true if all items are maps containing a "name" key.
func isNamedList(items []interface{}) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		m, ok := item.(map[string]interface{})
		if !ok {
			return false
		}
		if _, hasName := m["name"]; !hasName {
			return false
		}
	}
	return true
}

// compareNamedSlices matches slice items by their "name" field rather than index.
func (d *Differ) compareNamedSlices(expected, live []interface{}, prefix string) []types.FieldDiff {
	var diffs []types.FieldDiff

	liveByName := map[string]map[string]interface{}{}
	for _, item := range live {
		m := item.(map[string]interface{})
		name, _ := m["name"].(string)
		liveByName[name] = m
	}

	for i, item := range expected {
		expectedMap := item.(map[string]interface{})
		name, _ := expectedMap["name"].(string)
		path := fmt.Sprintf("%s.%d", prefix, i)

		liveMap, exists := liveByName[name]
		if !exists {
			diffs = append(diffs, types.FieldDiff{
				Path:     path + ".name",
				Expected: name,
				Actual:   "<missing>",
				Severity: classifyField(prefix, d.severityRules),
			})
			continue
		}

		diffs = append(diffs, d.compareObjects(expectedMap, liveMap, path)...)
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

// quantitiesEqual returns true if both strings parse as equal Kubernetes resource quantities.
func quantitiesEqual(a, b string) bool {
	qa, errA := resource.ParseQuantity(a)
	if errA != nil {
		return false
	}
	qb, errB := resource.ParseQuantity(b)
	if errB != nil {
		return false
	}
	return qa.Cmp(qb) == 0
}

func formatValue(v interface{}) string {
	if v == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", v)
}
