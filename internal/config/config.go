// Package config loads and validates .testwarden.yml configuration.
package config

import (
	"errors"
	"os"

	"gopkg.in/yaml.v3"
)

// Coverage configures coverage thresholds and report paths.
type Coverage struct {
	UnitThreshold           int    `yaml:"unit_threshold"`
	IntegrationGapThreshold int    `yaml:"integration_gap_threshold"`
	UnitPath                string `yaml:"unit_path"`
	IntegrationPath         string `yaml:"integration_path"`
	UnitCommand             string `yaml:"unit_command"`
	IntegrationCommand      string `yaml:"integration_command"`
}

// Mocks configures mock detection.
type Mocks struct {
	DetectOvermocking bool                `yaml:"detect_overmocking"`
	RealDependencies  map[string][]string `yaml:"real_dependencies"`
}

// AI configures the OpenAI-compatible endpoint.
type AI struct {
	Endpoint  string `yaml:"endpoint"`
	APIKey    string `yaml:"api_key"`
	Model     string `yaml:"model"`
	Timeout   int    `yaml:"timeout"`
	MaxTokens int    `yaml:"max_tokens"`
}

// Config is the root configuration loaded from .testwarden.yml.
type Config struct {
	Coverage  Coverage `yaml:"coverage"`
	Mocks     Mocks    `yaml:"mocks"`
	AI        AI       `yaml:"ai"`
	Languages []string `yaml:"languages"`
}

// Default returns a Config populated with sensible defaults.
func Default() *Config {
	return &Config{
		Coverage: Coverage{
			UnitThreshold:           80,
			IntegrationGapThreshold: 5,
			UnitPath:                "coverage.out",
			IntegrationPath:         "integration-coverage.out",
			UnitCommand:             "go test -coverprofile=coverage.out ./...",
			IntegrationCommand:      "go test -tags=integration -coverprofile=integration-coverage.out ./...",
		},
		Mocks: Mocks{
			DetectOvermocking: true,
			RealDependencies: map[string][]string{
				"go":         {"database/sql", "net/http", "os"},
				"typescript": {"fs", "http", "pg", "mysql2"},
			},
		},
		AI: AI{
			Endpoint:  "http://localhost:11434/v1",
			APIKey:    "",
			Model:     "qwen2.5-coder",
			Timeout:   120,
			MaxTokens: 4096,
		},
		Languages: []string{"go", "typescript"},
	}
}

// Load reads .testwarden.yml from cwd. If file is missing, returns Default().
func Load(path string) (*Config, error) {
	cfg := Default()

	if path == "" {
		return cfg, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return cfg, nil
		}
		return nil, err
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}
