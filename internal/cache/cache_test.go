package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	h, err := HashFile(path)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if len(h) != 64 {
		t.Errorf("expected 64-char hex hash, got %d", len(h))
	}
}

func TestHashFile_Deterministic(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	h1, _ := HashFile(path)
	h2, _ := HashFile(path)
	if h1 != h2 {
		t.Errorf("expected same hash, got %s vs %s", h1, h2)
	}
}

func TestHashFile_ChangesWithContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "foo.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	h1, _ := HashFile(path)
	if err := os.WriteFile(path, []byte("world"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	h2, _ := HashFile(path)
	if h1 == h2 {
		t.Error("expected different hash for different content")
	}
}

func TestCache_New(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}
	if _, err := os.Stat(c.dir); err != nil {
		t.Errorf("cache dir not created: %v", err)
	}
}

func TestCache_ManifestRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c, err := New(dir)
	if err != nil {
		t.Fatalf("new: %v", err)
	}

	m := &Manifest{Version: "1"}
	entry := Entry{Path: "foo.go", Hash: "abc123"}
	c.Store(m, entry)

	if err := c.Save(m); err != nil {
		t.Fatalf("save: %v", err)
	}

	loaded, err := c.LoadManifest()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(loaded.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(loaded.Entries))
	}
	if loaded.Entries[0].Hash != "abc123" {
		t.Errorf("hash mismatch: %s", loaded.Entries[0].Hash)
	}
}

func TestCache_Lookup_Fresh(t *testing.T) {
	dir := t.TempDir()
	c, _ := New(dir)
	entry := Entry{Path: "foo.go", Hash: "abc123"}
	m := &Manifest{}
	c.Store(m, entry)
	_ = c.Save(m)

	got, err := c.Lookup("foo.go", "abc123")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got == nil {
		t.Fatal("expected entry")
	}
	if !c.IsFresh(got, "abc123") {
		t.Error("expected fresh")
	}
}

func TestCache_Lookup_Stale(t *testing.T) {
	dir := t.TempDir()
	c, _ := New(dir)
	entry := Entry{Path: "foo.go", Hash: "abc123"}
	m := &Manifest{}
	c.Store(m, entry)
	_ = c.Save(m)

	got, err := c.Lookup("foo.go", "different-hash")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got != nil {
		t.Error("expected stale entry to be nil")
	}
}

func TestCache_Lookup_Missing(t *testing.T) {
	dir := t.TempDir()
	c, _ := New(dir)

	got, err := c.Lookup("nonexistent.go", "any")
	if err != nil {
		t.Fatalf("lookup: %v", err)
	}
	if got != nil {
		t.Error("expected nil for missing entry")
	}
}

func TestCache_Clear(t *testing.T) {
	dir := t.TempDir()
	c, _ := New(dir)
	m := &Manifest{}
	c.Store(m, Entry{Path: "foo.go", Hash: "abc"})
	_ = c.Save(m)

	if err := c.Clear(); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if _, err := os.Stat(c.dir); !os.IsNotExist(err) {
		t.Error("expected cache dir removed")
	}
}
