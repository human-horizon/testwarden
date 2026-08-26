package mocks

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanGoDir_Overmocking(t *testing.T) {
	dir := t.TempDir()
	testFile := `package foo

import (
	"database/sql"
	"github.com/DATA-DOG/go-sqlmock"
)

func TestFoo(t *testing.T) {
	_ = sql.DB{}
	_ = sqlmock{}
}
`
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	d := New(map[string][]string{
		"go": {"database/sql"},
	})

	violations, err := d.ScanDir(dir, "go")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(violations) == 0 {
		t.Fatal("expected at least one violation")
	}
	if violations[0].RealDep != "database/sql" {
		t.Errorf("expected database/sql, got %s", violations[0].RealDep)
	}
}

func TestScanGoDir_NoMockLib(t *testing.T) {
	dir := t.TempDir()
	testFile := `package foo

import (
	"database/sql"
)

func TestFoo(t *testing.T) {
	_ = sql.DB{}
}
`
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	d := New(map[string][]string{"go": {"database/sql"}})
	violations, err := d.ScanDir(dir, "go")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations without mock lib, got %d", len(violations))
	}
}

func TestScanTSDir_Overmocking(t *testing.T) {
	dir := t.TempDir()
	testFile := `import { Client } from 'pg';

jest.mock('pg');

test('foo', () => {});
`
	if err := os.WriteFile(filepath.Join(dir, "foo.test.ts"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	d := New(map[string][]string{
		"typescript": {"pg"},
	})

	violations, err := d.ScanDir(dir, "typescript")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(violations) == 0 {
		t.Fatal("expected violation")
	}
	if violations[0].RealDep != "pg" {
		t.Errorf("expected pg, got %s", violations[0].RealDep)
	}
}

func TestScanDir_UnknownLanguage(t *testing.T) {
	d := New(map[string][]string{})
	violations, err := d.ScanDir(".", "rust")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}
	if len(violations) != 0 {
		t.Errorf("expected 0 violations, got %d", len(violations))
	}
}
