// Package patcher writes AI-generated fixes with backup/rollback support.
package patcher

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

// Patcher manages backups under .testwarden/backup/.
type Patcher struct {
	root string
}

// New creates a Patcher rooted at the given project directory.
func New(root string) *Patcher {
	return &Patcher{root: root}
}

// BackupPath returns the backup location for filePath.
func (p *Patcher) BackupPath(filePath string) string {
	abs, _ := filepath.Abs(filePath)
	h := sha1.Sum([]byte(abs))
	name := hex.EncodeToString(h[:]) + ".bak"
	return filepath.Join(p.root, ".testwarden", "backup", name)
}

// Backup copies filePath to the backup location.
func (p *Patcher) Backup(filePath string) error {
	backupPath := p.BackupPath(filePath)
	if err := os.MkdirAll(filepath.Dir(backupPath), 0o755); err != nil {
		return fmt.Errorf("mkdir backup: %w", err)
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		return fmt.Errorf("write backup: %w", err)
	}
	return nil
}

// Apply writes content to filePath atomically via temp file + rename.
// On any error, the original file is preserved unchanged.
// Caller must Backup first if rollback is desired.
func (p *Patcher) Apply(filePath, content string) error {
	dir := filepath.Dir(filePath)
	tmp, err := os.CreateTemp(dir, ".testwarden-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()

	// Ensure temp file is removed on failure.
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write([]byte(content)); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync temp: %w", err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Chmod(tmpName, 0o644); err != nil {
		return fmt.Errorf("chmod temp: %w", err)
	}

	// Atomic rename. On most filesystems this is atomic.
	if err := os.Rename(tmpName, filePath); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	success = true
	return nil
}

// Rollback restores filePath from its backup. If no backup exists, returns nil.
func (p *Patcher) Rollback(filePath string) error {
	backupPath := p.BackupPath(filePath)
	data, err := os.ReadFile(backupPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if err := os.WriteFile(filePath, data, 0o644); err != nil {
		return fmt.Errorf("rollback write: %w", err)
	}
	return nil
}

// HasBackup reports whether a backup exists for filePath.
func (p *Patcher) HasBackup(filePath string) bool {
	_, err := os.Stat(p.BackupPath(filePath))
	return err == nil
}
