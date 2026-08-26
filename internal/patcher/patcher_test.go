package patcher

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBackupAndRollback(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "foo.go")
	original := "package foo\n\nfunc A() {}\n"
	if err := os.WriteFile(src, []byte(original), 0o644); err != nil {
		t.Fatalf("write src: %v", err)
	}

	p := New(dir)
	if err := p.Backup(src); err != nil {
		t.Fatalf("backup: %v", err)
	}
	if !p.HasBackup(src) {
		t.Fatal("expected backup to exist")
	}

	modified := "package foo\n\nfunc A() { println(1) }\n"
	if err := p.Apply(src, modified); err != nil {
		t.Fatalf("apply: %v", err)
	}

	got, _ := os.ReadFile(src)
	if !strings.Contains(string(got), "println(1)") {
		t.Errorf("file not modified: %s", got)
	}

	if err := p.Rollback(src); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	got, _ = os.ReadFile(src)
	if string(got) != original {
		t.Errorf("rollback failed: got %q, want %q", got, original)
	}
}

func TestRollbackNoBackup(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "foo.go")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	p := New(dir)
	if err := p.Rollback(src); err != nil {
		t.Errorf("expected nil on missing backup, got %v", err)
	}
}

func TestBackupPathDeterministic(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "foo.go")
	p := New(dir)

	p1 := p.BackupPath(src)
	p2 := p.BackupPath(src)
	if p1 != p2 {
		t.Errorf("expected deterministic backup path, got %s and %s", p1, p2)
	}
	if !strings.HasSuffix(p1, ".bak") {
		t.Errorf("expected .bak suffix, got %s", p1)
	}
}

func TestApply_NoTempLeftover(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "foo.go")
	if err := os.WriteFile(src, []byte("original"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	p := New(dir)
	if err := p.Apply(src, "updated"); err != nil {
		t.Fatalf("apply: %v", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".testwarden-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}

	got, _ := os.ReadFile(src)
	if string(got) != "updated" {
		t.Errorf("expected 'updated', got %q", got)
	}
}
