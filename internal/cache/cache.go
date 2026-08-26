// Package cache implements incremental analysis caching keyed by file hashes.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Entry holds cached analysis results for a single file.
type Entry struct {
	Path     string    `json:"path"`
	Hash     string    `json:"hash"`
	Modified time.Time `json:"modified"`

	// Mock violations
	Violations []MockViolation `json:"violations,omitempty"`
}

// MockViolation is a cached mock detection result.
type MockViolation struct {
	Line    int    `json:"line"`
	RealDep string `json:"real_dep"`
	MockLib string `json:"mock_lib"`
}

// Manifest is the cache index.
type Manifest struct {
	Version string  `json:"version"`
	Entries []Entry `json:"entries"`
}

// Cache manages an on-disk cache directory.
type Cache struct {
	dir    string
	suffix string // optional suffix for manifest filename (e.g. "go", "ts")
}

// New opens (or creates) a cache rooted at .testwarden/cache/ inside projectDir.
func New(projectDir string) (*Cache, error) {
	dir := filepath.Join(projectDir, ".testwarden", "cache")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir cache: %w", err)
	}
	return &Cache{dir: dir}, nil
}

// UseSuffix sets a per-language manifest suffix so each language keeps its own index.
// For example, UseSuffix("go") makes the manifest file manifest-go.json.
func (c *Cache) UseSuffix(suffix string) {
	c.suffix = suffix
}

func (c *Cache) manifestPath() string {
	if c.suffix == "" {
		return filepath.Join(c.dir, "manifest.json")
	}
	return filepath.Join(c.dir, "manifest-"+c.suffix+".json")
}

// HashFile returns the sha256 hash of the file contents.
func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

// LoadManifest reads the cache manifest from disk. Returns empty manifest if missing.
func (c *Cache) LoadManifest() (*Manifest, error) {
	m := &Manifest{Version: "1"}
	data, err := os.ReadFile(c.manifestPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return m, nil
		}
		return nil, err
	}
	if err := json.Unmarshal(data, m); err != nil {
		return nil, fmt.Errorf("decode manifest: %w", err)
	}
	return m, nil
}

// Save writes the manifest to disk atomically.
func (c *Cache) Save(m *Manifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	target := c.manifestPath()
	tmp := target + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, target)
}

// Lookup returns the cached entry for a file path, or nil if missing/stale.
func (c *Cache) Lookup(filePath, currentHash string) (*Entry, error) {
	m, err := c.LoadManifest()
	if err != nil {
		return nil, err
	}
	for i := range m.Entries {
		e := &m.Entries[i]
		if e.Path == filePath && e.Hash == currentHash {
			return e, nil
		}
	}
	return nil, nil
}

// Store adds or updates an entry in the in-memory manifest.
// Returns the updated manifest so the caller can persist it.
func (c *Cache) Store(m *Manifest, entry Entry) *Manifest {
	for i := range m.Entries {
		if m.Entries[i].Path == entry.Path {
			m.Entries[i] = entry
			return m
		}
	}
	m.Entries = append(m.Entries, entry)
	return m
}

// IsFresh reports whether the cached hash matches current hash.
func (c *Cache) IsFresh(e *Entry, currentHash string) bool {
	return e != nil && e.Hash == currentHash
}

// Clear removes all cache entries.
func (c *Cache) Clear() error {
	return os.RemoveAll(c.dir)
}
