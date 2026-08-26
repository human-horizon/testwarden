package config

import (
	"strings"
	"testing"
)

func TestValidate_Default(t *testing.T) {
	if err := Validate(Default()); err != nil {
		t.Errorf("default config should be valid, got: %v", err)
	}
}

func TestValidate_NilConfig(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Error("expected error for nil config")
	}
}

func TestValidate_BadThreshold(t *testing.T) {
	cfg := Default()
	cfg.Coverage.UnitThreshold = 150
	err := Validate(cfg)
	if err == nil || !IsValidationError(err) {
		t.Fatal("expected ValidationError")
	}
	if !strings.Contains(err.Error(), "unit_threshold") {
		t.Error("expected unit_threshold message")
	}
}

func TestValidate_BadEndpoint(t *testing.T) {
	cfg := Default()
	cfg.AI.Endpoint = "not-a-url"
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "ai.endpoint") {
		t.Error("expected endpoint message")
	}
}

func TestValidate_UnsupportedLanguage(t *testing.T) {
	cfg := Default()
	cfg.Languages = []string{"rust"}
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rust") {
		t.Error("expected unsupported language message")
	}
}

func TestValidate_MultipleIssues(t *testing.T) {
	cfg := Default()
	cfg.Coverage.UnitThreshold = -10
	cfg.AI.Endpoint = "broken"
	cfg.AI.Model = ""
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, want := range []string{"unit_threshold", "ai.endpoint", "ai.model"} {
		if !strings.Contains(msg, want) {
			t.Errorf("expected %q in error, got: %s", want, msg)
		}
	}
}
