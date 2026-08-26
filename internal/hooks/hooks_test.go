package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsGitRepo(t *testing.T) {
	dir := t.TempDir()
	if IsGitRepo(dir) {
		t.Error("empty dir should not be a git repo")
	}
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if !IsGitRepo(dir) {
		t.Error("expected dir with .git to be a git repo")
	}
}

func TestInstall_NotInRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := Install(dir)
	if err != ErrNotInGitRepo {
		t.Errorf("expected ErrNotInGitRepo, got %v", err)
	}
}

func TestInstall_CreatesHook(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	hookPath, err := Install(dir)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	if hookPath == "" {
		t.Fatal("expected non-empty hook path")
	}

	data, err := os.ReadFile(hookPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(data), "Installed by testwarden") {
		t.Error("expected marker in hook script")
	}

	fi, err := os.Stat(hookPath)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Mode()&0o111 == 0 {
		t.Error("expected hook to be executable")
	}
}

func TestInstall_PreservesExistingHook(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	hookPath := filepath.Join(dir, HookPath)
	original := "#!/bin/sh\necho user hook\n"
	if err := os.WriteFile(hookPath, []byte(original), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	if _, err := Install(dir); err != nil {
		t.Fatalf("install: %v", err)
	}

	if _, err := os.Stat(hookPath + ".bak"); err != nil {
		t.Errorf("expected backup created: %v", err)
	}
}

func TestInstall_ReinstallNoBackup(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	hookPath := filepath.Join(dir, HookPath)

	// First install
	if _, err := Install(dir); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// Second install should NOT create backup (it's our hook)
	if _, err := Install(dir); err != nil {
		t.Fatalf("second install: %v", err)
	}
	if _, err := os.Stat(hookPath + ".bak"); !os.IsNotExist(err) {
		t.Error("expected no backup on reinstall")
	}
}

func TestUninstall(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if _, err := Install(dir); err != nil {
		t.Fatalf("install: %v", err)
	}
	if err := Uninstall(dir); err != nil {
		t.Fatalf("uninstall: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, HookPath)); !os.IsNotExist(err) {
		t.Error("expected hook removed")
	}
}

func TestUninstall_NonTestwardenHook(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".git", "hooks"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	hookPath := filepath.Join(dir, HookPath)
	if err := os.WriteFile(hookPath, []byte("#!/bin/sh\necho user\n"), 0o755); err != nil {
		t.Fatalf("write: %v", err)
	}

	err := Uninstall(dir)
	if err == nil {
		t.Error("expected error when removing non-testwarden hook")
	}
}
