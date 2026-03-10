package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const maxConfigSize = 1024 * 1024 // 1MB

type Config struct {
	Sources []Source   `yaml:"sources"`
	Ignore  Ignore     `yaml:"ignore"`
	Severity Severity  `yaml:"severity"`
	Cluster Cluster    `yaml:"cluster"`
	Flux    Flux       `yaml:"flux"`
	FailOn  string     `yaml:"failOn"`
}

type Source struct {
	Path string `yaml:"path"`
	Type string `yaml:"type"`
}

type Ignore struct {
	Fields    []string          `yaml:"fields"`
	Resources []map[string]string `yaml:"resources"`
}

type Severity struct {
	Critical []string `yaml:"critical"`
	Warning  []string `yaml:"warning"`
}

type Cluster struct {
	Context string `yaml:"context"`
}

type Flux struct {
	Enabled bool `yaml:"enabled"`
}

var allowedKeys = map[string]bool{
	"sources": true,
	"ignore": true,
	"severity": true,
	"cluster": true,
	"flux": true,
	"failOn": true,
}

// Load reads and parses a YAML config file with strict unknown-key validation
func Load(configPath string) (*Config, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if len(data) > maxConfigSize {
		return nil, fmt.Errorf("config file exceeds maximum size of %d bytes", maxConfigSize)
	}

	// Check for unknown keys by unmarshaling to map first
	var rawMap map[string]interface{}
	if err := yaml.Unmarshal(data, &rawMap); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	for key := range rawMap {
		if !allowedKeys[key] {
			return nil, fmt.Errorf("unknown config key: %s", key)
		}
	}

	// Now unmarshal to struct
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	applyDefaults(&cfg)
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.FailOn == "" {
		cfg.FailOn = "critical"
	}

	if len(cfg.Ignore.Fields) == 0 {
		cfg.Ignore.Fields = []string{
			"metadata.managedFields",
			"metadata.resourceVersion",
			"status",
		}
	}

	if len(cfg.Ignore.Resources) == 0 {
		cfg.Ignore.Resources = []map[string]string{
			{"kind": "Secret"},
		}
	}
}
