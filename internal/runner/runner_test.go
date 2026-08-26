package runner

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/HumanHorizon/testwarden/internal/config"
)

func TestRunCheck_LowCoverage_Exit1(t *testing.T) {
	dir := t.TempDir()

	// Create fake coverage.out with low coverage
	coverageOut := `mode: set
github.com/example/foo.go:10.12,15.16 1 0
github.com/example/foo.go:16.20,20.5 1 0
`
	if err := os.WriteFile(filepath.Join(dir, "coverage.out"), []byte(coverageOut), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := config.Default()
	cfg.Coverage.UnitCommand = "" // don't actually run tests
	cfg.Languages = []string{"go"}

	var buf bytes.Buffer
	opts := Options{Cfg: cfg, Root: dir, Out: &buf}

	code, err := RunCheck(context.Background(), opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	out := buf.String()
	if !contains(out, "FAIL") {
		t.Error("expected FAIL in output")
	}
	if !contains(out, "below threshold") {
		t.Error("expected 'below threshold' message")
	}
}

func TestRunCheck_HighCoverage_Exit0(t *testing.T) {
	dir := t.TempDir()

	coverageOut := `mode: set
github.com/example/foo.go:10.12,15.16 1 1
github.com/example/foo.go:16.20,20.5 1 1
`
	if err := os.WriteFile(filepath.Join(dir, "coverage.out"), []byte(coverageOut), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := config.Default()
	cfg.Coverage.UnitCommand = ""
	cfg.Languages = []string{"go"}

	var buf bytes.Buffer
	opts := Options{Cfg: cfg, Root: dir, Out: &buf}

	code, err := RunCheck(context.Background(), opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	out := buf.String()
	if !contains(out, "PASS") {
		t.Error("expected PASS in output")
	}
}

func TestRunCheck_GapDetection(t *testing.T) {
	dir := t.TempDir()

	// Unit covers foo.go:10, but neither covers bar.go:5.
	unitOut := `mode: set
github.com/example/foo.go:10.12,15.16 1 1
github.com/example/bar.go:5.10,8.2 1 0
`
	integrationOut := `mode: set
github.com/example/foo.go:10.12,15.16 1 1
`
	if err := os.WriteFile(filepath.Join(dir, "coverage.out"), []byte(unitOut), 0o644); err != nil {
		t.Fatalf("write unit: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "integration-coverage.out"), []byte(integrationOut), 0o644); err != nil {
		t.Fatalf("write integration: %v", err)
	}

	cfg := config.Default()
	cfg.Coverage.UnitCommand = ""
	cfg.Coverage.IntegrationCommand = ""
	cfg.Coverage.IntegrationGapThreshold = 1 // tight threshold to ensure flag
	cfg.Languages = []string{"go"}

	var buf bytes.Buffer
	opts := Options{Cfg: cfg, Root: dir, Out: &buf}

	code, _ := RunCheck(context.Background(), opts)
	if code != 1 {
		t.Errorf("expected exit code 1 due to gap, got %d", code)
	}
	if !contains(buf.String(), "coverage gap") {
		t.Error("expected gap message in output")
	}
	if !contains(buf.String(), "github.com/example/bar.go") {
		t.Error("expected bar.go in gap output")
	}
}

func TestRunCheck_MockViolation(t *testing.T) {
	dir := t.TempDir()

	coverageOut := `mode: set
github.com/example/foo.go:10.12,15.16 1 1
`
	if err := os.WriteFile(filepath.Join(dir, "coverage.out"), []byte(coverageOut), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Create a test file with over-mocking
	testFile := `package foo

import (
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
)
`
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("write test: %v", err)
	}

	cfg := config.Default()
	cfg.Coverage.UnitCommand = ""
	cfg.Languages = []string{"go"}

	var buf bytes.Buffer
	opts := Options{Cfg: cfg, Root: dir, Out: &buf}

	code, _ := RunCheck(context.Background(), opts)
	if code != 1 {
		t.Errorf("expected exit code 1 due to over-mocking, got %d", code)
	}
	if !contains(buf.String(), "over-mocking") {
		t.Error("expected over-mocking message")
	}
}

func TestRunFix_DryRunNoChanges(t *testing.T) {
	dir := t.TempDir()

	testFile := `package foo

import (
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
)
`
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	coverageOut := `mode: set
github.com/example/foo.go:10.12,15.16 1 1
`
	if err := os.WriteFile(filepath.Join(dir, "coverage.out"), []byte(coverageOut), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := config.Default()
	cfg.Coverage.UnitCommand = ""
	cfg.Languages = []string{"go"}

	var buf bytes.Buffer
	opts := Options{Cfg: cfg, Root: dir, DryRun: true, Out: &buf}

	_, err := RunFix(context.Background(), opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	// File should be unchanged after dry-run.
	got, _ := os.ReadFile(filepath.Join(dir, "foo_test.go"))
	if string(got) != testFile {
		t.Error("dry-run modified the file")
	}
}

func TestRunCheck_JSONOutput(t *testing.T) {
	dir := t.TempDir()
	coverageOut := `mode: set
github.com/example/foo.go:10.12,15.16 1 1
`
	if err := os.WriteFile(filepath.Join(dir, "coverage.out"), []byte(coverageOut), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := config.Default()
	cfg.Coverage.UnitCommand = ""
	cfg.Languages = []string{"go"}

	var buf bytes.Buffer
	opts := Options{Cfg: cfg, Root: dir, Out: &buf, JSON: true}

	_, err := RunCheck(context.Background(), opts)
	if err != nil {
		t.Fatalf("run: %v", err)
	}

	if !contains(buf.String(), `"language"`) {
		t.Error("expected JSON output with language field")
	}
	if !contains(buf.String(), `"passed"`) {
		t.Error("expected JSON output with passed field")
	}
}

func contains(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}
