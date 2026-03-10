package security

import (
	"strconv"
	"strings"
	"testing"
)

func TestSafeYAMLDecode_RejectsOversized(t *testing.T) {
	// Generate input exceeding 10MB limit
	chunk := "a: b\n"
	repeated := strings.Repeat(chunk, 11*1024*1024/len(chunk)+1)
	data := []byte(repeated)

	_, err := SafeYAMLDecode(data)
	if err == nil {
		t.Fatal("expected error for oversized YAML input, got nil")
	}
}

func TestSafeYAMLDecode_RejectsDeeplyNested(t *testing.T) {
	// Build 200 levels of nesting: "l0:\n  l1:\n    l2:\n ..."
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString(strings.Repeat("  ", i))
		b.WriteString("l")
		b.WriteString(strconv.Itoa(i))
		b.WriteString(":\n")
	}
	// Add a leaf value
	b.WriteString(strings.Repeat("  ", 200))
	b.WriteString("val: true\n")

	_, err := SafeYAMLDecode([]byte(b.String()))
	if err == nil {
		t.Fatal("expected error for deeply nested YAML, got nil")
	}
}

func TestSafeYAMLDecode_AcceptsNormalYAML(t *testing.T) {
	input := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n")

	result, err := SafeYAMLDecode(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["kind"] != "ConfigMap" {
		t.Fatalf("expected kind=ConfigMap, got %v", result["kind"])
	}
}

func TestSanitizeString(t *testing.T) {
	input := "hello\x00world\ttab\nnewline\x01gone"
	got := SanitizeString(input)
	want := "helloworld\ttab\nnewline gone"
	if got != want {
		t.Fatalf("SanitizeString(%q) = %q, want %q", input, got, want)
	}
}
