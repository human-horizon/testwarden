package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Coverage.UnitThreshold != 80 {
		t.Errorf("expected unit_threshold 80, got %d", cfg.Coverage.UnitThreshold)
	}
	if !cfg.Mocks.DetectOvermocking {
		t.Error("expected detect_overmocking true")
	}
	if cfg.AI.Model != "qwen2.5-coder" {
		t.Errorf("expected model qwen2.5-coder, got %s", cfg.AI.Model)
	}
}

func TestLoadMissing(t *testing.T) {
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Coverage.UnitThreshold != 80 {
		t.Errorf("expected defaults, got %d", cfg.Coverage.UnitThreshold)
	}
}

func TestLoadFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".testwarden.yml")

	yaml := `coverage:
  unit_threshold: 90
ai:
  model: custom-model
`
	if err := os.WriteFile(path, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Coverage.UnitThreshold != 90 {
		t.Errorf("expected 90, got %d", cfg.Coverage.UnitThreshold)
	}
	if cfg.AI.Model != "custom-model" {
		t.Errorf("expected custom-model, got %s", cfg.AI.Model)
	}
	if !cfg.Mocks.DetectOvermocking {
		t.Error("expected default overmocking true after partial load")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".testwarden.yml")
	if err := os.WriteFile(path, []byte("not: valid: yaml: :::"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Error("expected error on invalid yaml")
	}
}
