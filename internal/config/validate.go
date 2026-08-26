package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
)

// ValidationError collects multiple config issues.
type ValidationError struct {
	Issues []string
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if len(e.Issues) == 0 {
		return "config validation failed"
	}
	if len(e.Issues) == 1 {
		return "config validation: " + e.Issues[0]
	}
	return fmt.Sprintf("config validation failed (%d issues):\n  - %s",
		len(e.Issues), strings.Join(e.Issues, "\n  - "))
}

// Validate checks a Config for semantic correctness.
// Returns nil on success, *ValidationError with all issues otherwise.
func Validate(cfg *Config) error {
	if cfg == nil {
		return &ValidationError{Issues: []string{"config is nil"}}
	}

	var issues []string

	// Coverage thresholds must be 0..100.
	if cfg.Coverage.UnitThreshold < 0 || cfg.Coverage.UnitThreshold > 100 {
		issues = append(issues, fmt.Sprintf(
			"coverage.unit_threshold must be in [0, 100], got %d", cfg.Coverage.UnitThreshold,
		))
	}
	if cfg.Coverage.IntegrationGapThreshold < 0 || cfg.Coverage.IntegrationGapThreshold > 100 {
		issues = append(issues, fmt.Sprintf(
			"coverage.integration_gap_threshold must be in [0, 100], got %d", cfg.Coverage.IntegrationGapThreshold,
		))
	}

	// Languages must be non-empty and recognized.
	if len(cfg.Languages) == 0 {
		issues = append(issues, "languages must be non-empty")
	}
	for _, lang := range cfg.Languages {
		if lang != "go" && lang != "typescript" {
			issues = append(issues, fmt.Sprintf(
				"unsupported language %q (supported: go, typescript)", lang,
			))
		}
	}

	// AI endpoint must be a valid URL.
	if cfg.AI.Endpoint == "" {
		issues = append(issues, "ai.endpoint must not be empty")
	} else if u, err := url.Parse(cfg.AI.Endpoint); err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		issues = append(issues, fmt.Sprintf(
			"ai.endpoint %q must be a valid http(s) URL", cfg.AI.Endpoint,
		))
	}

	// AI model must be non-empty.
	if cfg.AI.Model == "" {
		issues = append(issues, "ai.model must not be empty")
	}

	// AI timeout must be positive.
	if cfg.AI.Timeout <= 0 {
		issues = append(issues, fmt.Sprintf(
			"ai.timeout must be > 0 seconds, got %d", cfg.AI.Timeout,
		))
	}

	if cfg.AI.MaxTokens <= 0 {
		issues = append(issues, fmt.Sprintf(
			"ai.max_tokens must be > 0, got %d", cfg.AI.MaxTokens,
		))
	}

	// Real dependencies map consistency.
	if cfg.Mocks.DetectOvermocking {
		for lang, deps := range cfg.Mocks.RealDependencies {
			if len(deps) == 0 {
				issues = append(issues, fmt.Sprintf(
					"mocks.real_dependencies.%s is empty but detect_overmocking is true", lang,
				))
			}
			for _, dep := range deps {
				if strings.TrimSpace(dep) == "" {
					issues = append(issues, fmt.Sprintf(
						"mocks.real_dependencies.%s contains empty string", lang,
					))
				}
			}
		}
	}

	if len(issues) > 0 {
		return &ValidationError{Issues: issues}
	}
	return nil
}

// IsValidationError reports whether err is a *ValidationError.
func IsValidationError(err error) bool {
	var ve *ValidationError
	return errors.As(err, &ve)
}
