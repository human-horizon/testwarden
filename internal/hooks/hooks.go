// Package hooks installs and manages git pre-commit hooks.
package hooks

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// HookPath is the conventional pre-commit hook location relative to .git/.
const HookPath = ".git/hooks/pre-commit"

// preCommitScript is the shell script installed as the git pre-commit hook.
// It runs testwarden in check mode and aborts the commit if any issues are found.
const preCommitScript = `#!/usr/bin/env bash
# Installed by testwarden — do not edit manually.
set -e

if ! command -v testwarden >/dev/null 2>&1; then
  echo "testwarden not found in PATH; skipping pre-commit checks"
  exit 0
fi

echo "testwarden: running pre-commit checks..."
if ! testwarden check; then
  echo "testwarden: pre-commit checks failed. Commit aborted."
  echo "  Run 'testwarden check' to see issues."
  echo "  Run 'testwarden fix' to attempt auto-remediation."
  exit 1
fi
`

// ErrNotInGitRepo is returned when .git is missing in the project root.
var ErrNotInGitRepo = errors.New("project is not a git repository")

// Install creates or updates the pre-commit hook in the project root.
// If a hook already exists at the path, it's preserved and a backup is created.
func Install(projectRoot string) (string, error) {
	gitDir := filepath.Join(projectRoot, ".git")
	if _, err := os.Stat(gitDir); err != nil {
		if os.IsNotExist(err) {
			return "", ErrNotInGitRepo
		}
		return "", err
	}

	hookPath := filepath.Join(projectRoot, HookPath)

	if err := os.MkdirAll(filepath.Dir(hookPath), 0o755); err != nil {
		return "", fmt.Errorf("mkdir hooks: %w", err)
	}

	// Backup existing hook if present and not ours.
	if existing, err := os.ReadFile(hookPath); err == nil {
		if !containsInstalledBy(existing) {
			backupPath := hookPath + ".bak"
			if err := os.WriteFile(backupPath, existing, 0o755); err != nil {
				return "", fmt.Errorf("backup existing hook: %w", err)
			}
		}
	}

	if err := os.WriteFile(hookPath, []byte(preCommitScript), 0o755); err != nil {
		return "", fmt.Errorf("write hook: %w", err)
	}
	return hookPath, nil
}

// Uninstall removes the pre-commit hook if it was installed by testwarden.
func Uninstall(projectRoot string) error {
	hookPath := filepath.Join(projectRoot, HookPath)
	data, err := os.ReadFile(hookPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if !containsInstalledBy(data) {
		return fmt.Errorf("hook at %s was not installed by testwarden", hookPath)
	}
	return os.Remove(hookPath)
}

// IsGitRepo reports whether the directory contains a .git folder.
func IsGitRepo(projectRoot string) bool {
	_, err := os.Stat(filepath.Join(projectRoot, ".git"))
	return err == nil
}

// GitRoot returns the project root (parent of .git) or empty string.
// Useful when invoked from a subdirectory.
func GitRoot() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return filepath.Clean(string(out)), nil
}

func containsInstalledBy(content []byte) bool {
	return contains(content, "Installed by testwarden")
}

func contains(haystack []byte, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == needle {
			return true
		}
	}
	return false
}
