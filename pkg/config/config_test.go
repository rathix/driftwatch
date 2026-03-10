package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_ValidConfig(t *testing.T) {
	configPath := filepath.Join("testdata", "valid_config.yaml")
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if len(cfg.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(cfg.Sources))
	}
	if cfg.Cluster.Context != "production" {
		t.Errorf("expected context=production, got %s", cfg.Cluster.Context)
	}
	if !cfg.Flux.Enabled {
		t.Errorf("expected flux.enabled=true, got %v", cfg.Flux.Enabled)
	}
}

func TestLoad_InvalidConfig_UnknownKeys(t *testing.T) {
	configPath := filepath.Join("testdata", "invalid_config.yaml")
	_, err := Load(configPath)
	if err == nil {
		t.Fatal("expected error for unknown keys, got nil")
	}
}

func TestLoad_Defaults(t *testing.T) {
	// Create a temporary minimal config file
	tmpFile, err := os.CreateTemp(t.TempDir(), "*.yaml")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer tmpFile.Close()

	// Write empty config
	if _, err := tmpFile.WriteString("{}"); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	cfg, err := Load(tmpFile.Name())
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.FailOn != "critical" {
		t.Errorf("expected failOn=critical, got %s", cfg.FailOn)
	}
	if len(cfg.Ignore.Fields) == 0 {
		t.Error("expected default ignore.fields to be set")
	}
	if len(cfg.Ignore.Resources) == 0 {
		t.Error("expected default ignore.resources to be set")
	}
}

func TestValidatePath_RejectsAbsolute(t *testing.T) {
	err := ValidatePath("/etc/passwd", "/repo")
	if err == nil {
		t.Error("expected error for absolute path, got nil")
	}
}

func TestValidatePath_RejectsTraversal(t *testing.T) {
	err := ValidatePath("../../etc/passwd", "/repo")
	if err == nil {
		t.Error("expected error for path traversal, got nil")
	}
}

func TestValidatePath_AcceptsRelative(t *testing.T) {
	err := ValidatePath("./infrastructure", "/repo")
	if err != nil {
		t.Errorf("expected no error for valid relative path, got %v", err)
	}
}
