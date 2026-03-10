package config

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidatePath checks that a path is safe: not absolute, no traversal, stays within repo root
func ValidatePath(p, repoRoot string) error {
	if filepath.IsAbs(p) {
		return fmt.Errorf("path must be relative, got absolute path: %s", p)
	}

	if strings.Contains(p, "..") {
		return fmt.Errorf("path traversal not allowed: %s", p)
	}

	// Resolve the full path and check it stays within repo root
	fullPath := filepath.Join(repoRoot, p)
	absRepoRoot, err := filepath.Abs(repoRoot)
	if err != nil {
		return fmt.Errorf("failed to resolve repo root: %w", err)
	}

	absPath, err := filepath.Abs(fullPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Ensure the resolved path starts with repo root
	if !strings.HasPrefix(absPath, absRepoRoot) {
		return fmt.Errorf("path escapes repo root: %s", p)
	}

	return nil
}
