package mocks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoAST_Imports(t *testing.T) {
	dir := t.TempDir()
	src := `package foo

import (
	"database/sql"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestFoo(t *testing.T) {
	_ = sql.Open
	_ = sqlmock.New()
}
`
	path := filepath.Join(dir, "foo_test.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	parsed, err := parseGoAST(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(parsed.imports) != 2 {
		t.Fatalf("expected 2 imports, got %d", len(parsed.imports))
	}
	if !parsed.usesMock {
		t.Error("expected mock library usage detected")
	}
}

func TestParseGoAST_NoMock(t *testing.T) {
	dir := t.TempDir()
	src := `package foo

import "fmt"

func TestFoo(t *testing.T) {
	fmt.Println("hi")
}
`
	path := filepath.Join(dir, "foo_test.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	parsed, err := parseGoAST(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if parsed.usesMock {
		t.Error("did not expect mock detection")
	}
}

func TestParseGoAST_InvalidFile(t *testing.T) {
	dir := t.TempDir()
	src := `package foo

func broken( { /* invalid syntax */`
	path := filepath.Join(dir, "bad_test.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := parseGoAST(path); err == nil {
		t.Error("expected parse error on invalid file")
	}
}

func TestRealDepForImport(t *testing.T) {
	real := map[string]bool{"database/sql": true, "net/http": true}

	tests := []struct {
		imp  astImport
		want string
	}{
		{astImport{path: "database/sql"}, "database/sql"},
		{astImport{path: "net/http"}, "net/http"},
		{astImport{path: "fmt"}, ""},
		{astImport{alias: "http", path: "net/http"}, "net/http"},
	}
	for _, tt := range tests {
		got := realDepForImport(tt.imp, real)
		if got != tt.want {
			t.Errorf("realDepForImport(%+v) = %q, want %q", tt.imp, got, tt.want)
		}
	}
}

func TestScanGoDir_ASTDetectsOvermocking(t *testing.T) {
	dir := t.TempDir()
	src := `package foo

import (
	"database/sql"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
)

func TestFoo(t *testing.T) {
	_ = sql.Open
	_ = sqlmock.New()
}
`
	if err := os.WriteFile(filepath.Join(dir, "foo_test.go"), []byte(src), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	d := New(map[string][]string{"go": {"database/sql"}})
	violations, err := d.ScanDir(dir, "go")
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	if len(violations) == 0 {
		t.Fatal("expected AST-based detection")
	}
	if violations[0].RealDep != "database/sql" {
		t.Errorf("expected database/sql, got %s", violations[0].RealDep)
	}
	if !strings.Contains(violations[0].MockLib, "sqlmock") {
		t.Errorf("expected sqlmock lib, got %s", violations[0].MockLib)
	}
}

func TestMatchMockLib(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"github.com/DATA-DOG/go-sqlmock", "github.com/DATA-DOG/go-sqlmock"},
		{"github.com/golang/mock/gomock", "github.com/golang/mock/gomock"},
		{"github.com/stretchr/testify/mock", "github.com/stretchr/testify/mock"},
		{"fmt", ""},
	}
	for _, tt := range tests {
		got := matchMockLib(tt.path)
		if got != tt.want {
			t.Errorf("matchMockLib(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}
