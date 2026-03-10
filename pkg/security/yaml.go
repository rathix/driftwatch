package security

import (
	"fmt"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
)

const (
	maxYAMLSize  = 10 * 1024 * 1024 // 10 MB
	maxYAMLDepth = 100
)

// SafeYAMLDecode decodes YAML with size and depth limits to guard against
// YAML bombs and oversized input.
func SafeYAMLDecode(data []byte) (map[string]interface{}, error) {
	if len(data) > maxYAMLSize {
		return nil, fmt.Errorf("YAML input exceeds maximum allowed size of %d bytes", maxYAMLSize)
	}

	var node yaml.Node
	if err := yaml.Unmarshal(data, &node); err != nil {
		return nil, fmt.Errorf("YAML parse error: %w", err)
	}

	depth := measureDepth(&node)
	if depth > maxYAMLDepth {
		return nil, fmt.Errorf("YAML nesting depth %d exceeds maximum allowed depth of %d", depth, maxYAMLDepth)
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("YAML unmarshal error: %w", err)
	}

	return result, nil
}

// measureDepth returns the maximum nesting depth of a yaml.Node tree.
func measureDepth(node *yaml.Node) int {
	if node == nil || len(node.Content) == 0 {
		return 1
	}
	max := 0
	for _, child := range node.Content {
		d := measureDepth(child)
		if d > max {
			max = d
		}
	}
	return max + 1
}

// SanitizeString strips control characters from s, keeping only printable
// runes plus newline and tab.
func SanitizeString(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\n' || r == '\t':
			b.WriteRune(r)
		case r >= 32 && !unicode.IsControl(r):
			b.WriteRune(r)
		case unicode.IsControl(r) && r < 32:
			// Replace NUL with nothing, other control chars with space
			if r != 0 {
				b.WriteRune(' ')
			}
		}
	}
	return b.String()
}
