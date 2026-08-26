package runner

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/HumanHorizon/testwarden/internal/config"
)

func TestDetectGoMockViolations_CacheHit(t *testing.T) {
	dir := t.TempDir()

	// Write a Go test file
	testFile := `package foo

import (
	"database/sql"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestFoo(t *testing.T) {
	_ = sql.Open
	_ = sqlmock.New()
}
`
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := config.Default()
	cfg.Mocks.RealDependencies = map[string][]string{"go": {"database/sql"}}

	// First run: should populate cache
	v1, err := detectGoMockViolations(dir, cfg)
	if err != nil {
		t.Fatalf("first scan: %v", err)
	}
	if len(v1) == 0 {
		t.Fatal("expected at least one violation")
	}

	// Cache should now exist
	cacheDir := filepath.Join(dir, ".testwarden", "cache")
	if _, err := os.Stat(filepath.Join(cacheDir, "manifest-go.json")); err != nil {
		t.Errorf("expected manifest-go.json, got: %v", err)
	}

	// Second run: should use cache, still return same violations
	v2, err := detectGoMockViolations(dir, cfg)
	if err != nil {
		t.Fatalf("second scan: %v", err)
	}
	if len(v2) != len(v1) {
		t.Errorf("expected same violations count, got %d vs %d", len(v2), len(v1))
	}
}

func TestDetectGoMockViolations_CacheStaleOnChange(t *testing.T) {
	dir := t.TempDir()

	testFile := `package foo

import (
	"database/sql"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
)
`
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := config.Default()
	cfg.Mocks.RealDependencies = map[string][]string{"go": {"database/sql"}}

	if _, err := detectGoMockViolations(dir, cfg); err != nil {
		t.Fatalf("first: %v", err)
	}

	// Modify file → hash changes → cache stale
	newContent := testFile + "\n// added a comment\n"
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(newContent), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	v, err := detectGoMockViolations(dir, cfg)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	// Should still find the violation (via fresh scan, not stale cache).
	if len(v) == 0 {
		t.Error("expected violation after file change")
	}
}

func TestCollectGoTestFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a_test.go"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.go"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "c_test.go"), []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	files, err := collectGoTestFiles(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(files) != 2 {
		t.Errorf("expected 2 test files, got %d", len(files))
	}
}

func TestDetectGoMockViolations_Disabled(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Mocks.DetectOvermocking = false

	v, err := detectGoMockViolations(dir, cfg)
	if err != nil {
		t.Fatalf("disabled: %v", err)
	}
	if v != nil {
		t.Errorf("expected nil when disabled, got %v", v)
	}
}

func TestDetectGoMockViolations_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Mocks.RealDependencies = map[string][]string{"go": {"database/sql"}}

	v, err := detectGoMockViolations(dir, cfg)
	if err != nil {
		t.Fatalf("empty: %v", err)
	}
	if len(v) != 0 {
		t.Errorf("expected 0 violations in empty dir, got %d", len(v))
	}
}

func TestDetectTSMockViolations_CacheHit(t *testing.T) {
	dir := t.TempDir()

	testFile := `import { Client } from 'pg';

jest.mock('pg');

test('foo', () => {});
`
	if err := os.WriteFile(filepath.Join(dir, "foo.test.ts"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := config.Default()
	cfg.Mocks.RealDependencies = map[string][]string{"typescript": {"pg"}}

	v1, err := detectTSMockViolations(dir, cfg)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	if len(v1) == 0 {
		t.Fatal("expected at least one violation")
	}
	if v1[0].RealDep != "pg" {
		t.Errorf("expected pg, got %s", v1[0].RealDep)
	}

	cacheDir := filepath.Join(dir, ".testwarden", "cache")
	if _, err := os.Stat(filepath.Join(cacheDir, "manifest-ts.json")); err != nil {
		t.Errorf("expected manifest-ts.json, got: %v", err)
	}

	v2, err := detectTSMockViolations(dir, cfg)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(v2) != len(v1) {
		t.Errorf("expected same violations, got %d vs %d", len(v2), len(v1))
	}
}

func TestDetectTSMockViolations_StaleOnChange(t *testing.T) {
	dir := t.TempDir()

	testFile := `import { Client } from 'pg';

jest.mock('pg');
`
	if err := os.WriteFile(filepath.Join(dir, "foo.test.ts"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg := config.Default()
	cfg.Mocks.RealDependencies = map[string][]string{"typescript": {"pg"}}

	if _, err := detectTSMockViolations(dir, cfg); err != nil {
		t.Fatalf("first: %v", err)
	}

	newContent := testFile + "\ntest('extra', () => {});\n"
	if err := os.WriteFile(filepath.Join(dir, "foo.test.ts"), []byte(newContent), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}

	v, err := detectTSMockViolations(dir, cfg)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if len(v) == 0 {
		t.Error("expected violation after file change")
	}
}

func TestDetectTSMockViolations_Disabled(t *testing.T) {
	dir := t.TempDir()
	cfg := config.Default()
	cfg.Mocks.DetectOvermocking = false

	v, err := detectTSMockViolations(dir, cfg)
	if err != nil {
		t.Fatalf("disabled: %v", err)
	}
	if v != nil {
		t.Errorf("expected nil when disabled, got %v", v)
	}
}

func TestCollectTSTestFiles(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"a.test.ts":  "x",
		"b.ts":       "x",
		"c.spec.ts":  "x",
		"d.test.tsx": "x",
		"FooTest.ts": "x",
		"e.tsx":      "x",
	}
	for name, content := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	got, err := collectTSTestFiles(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	if len(got) != 4 {
		t.Errorf("expected 4 test files, got %d: %v", len(got), got)
	}
}

func TestGoAndTSCachesAreSeparate(t *testing.T) {
	dir := t.TempDir()

	goTest := `package foo

import (
	"database/sql"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
)
`
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(goTest), 0o644); err != nil {
		t.Fatalf("write go: %v", err)
	}
	tsTest := `import { Client } from 'pg';
jest.mock('pg');
`
	if err := os.WriteFile(filepath.Join(dir, "foo.test.ts"), []byte(tsTest), 0o644); err != nil {
		t.Fatalf("write ts: %v", err)
	}

	cfg := config.Default()
	cfg.Mocks.RealDependencies = map[string][]string{
		"go":         {"database/sql"},
		"typescript": {"pg"},
	}

	goV, err := detectGoMockViolations(dir, cfg)
	if err != nil {
		t.Fatalf("go: %v", err)
	}
	tsV, err := detectTSMockViolations(dir, cfg)
	if err != nil {
		t.Fatalf("ts: %v", err)
	}

	if len(goV) == 0 || len(tsV) == 0 {
		t.Errorf("expected both violations: go=%d ts=%d", len(goV), len(tsV))
	}

	cacheDir := filepath.Join(dir, ".testwarden", "cache")
	if _, err := os.Stat(filepath.Join(cacheDir, "manifest-go.json")); err != nil {
		t.Errorf("missing manifest-go.json: %v", err)
	}
	if _, err := os.Stat(filepath.Join(cacheDir, "manifest-ts.json")); err != nil {
		t.Errorf("missing manifest-ts.json: %v", err)
	}
}

var _ = context.Background // keep context import for tests
