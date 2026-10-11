package orchestrator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// ── Cache entry ─────────────────────────────────────────────

type fileCacheEntry struct {
	ModTime int64  `json:"mtime"`
	Size    int64  `json:"size"`
	Hash    string `json:"hash"`
}

// ── FileCache ───────────────────────────────────────────────

// FileCache stores file metadata to detect changes between scans.
// This cache provides statistics only; it never bypasses security scanners.
// If the SHA-256 hash matches (e.g. a touch),
// the entry is updated and the file is still considered unchanged.
type FileCache struct {
	mu       sync.Mutex
	path     string
	entries  map[string]fileCacheEntry
	modified bool
}

// NewFileCache creates or loads a file cache in the given directory.
// The cache is persisted at <dir>/file-scan.json.
func NewFileCache(cacheDir string) (*FileCache, error) {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, err
	}
	cachePath := filepath.Join(cacheDir, "file-scan.json")
	fc := &FileCache{
		path:    cachePath,
		entries: make(map[string]fileCacheEntry),
	}
	// Best-effort load: start fresh if the file doesn't exist or is corrupt
	if err := fc.Load(); err != nil {
		fc.entries = make(map[string]fileCacheEntry)
	}
	return fc, nil
}

// IsChanged checks whether the file at the given path has changed since
// the last time it was cached. Returns (true, hash, nil) if the file is new
// or its content has changed.
//
// Every check hashes the content, including same-size changes with preserved mtime.
// If the hash matches, the entry is updated with the new modtime/size.
func (c *FileCache) IsChanged(path string) (bool, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	info, err := os.Stat(path)
	if err != nil {
		return false, "", err
	}

	entry, exists := c.entries[path]
	modTime := info.ModTime().UnixNano()
	size := info.Size()

	// Always compute the content hash
	hash, err := computeFileHash(path)
	if err != nil {
		return false, "", err
	}

	if exists && entry.Hash == hash {
		// Content unchanged — only metadata changed (touch, atime, etc.)
		// Refresh metadata for statistics; future checks still hash content.
		c.entries[path] = fileCacheEntry{
			ModTime: modTime,
			Size:    size,
			Hash:    hash,
		}
		c.modified = true
		return false, hash, nil
	}

	// Content changed or new file
	return true, hash, nil
}

// MarkScanned records the current state of the file in the cache,
// marking it as already scanned.
func (c *FileCache) MarkScanned(path string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	hash, err := computeFileHash(path)
	if err != nil {
		return err
	}

	c.entries[path] = fileCacheEntry{
		ModTime: info.ModTime().UnixNano(),
		Size:    info.Size(),
		Hash:    hash,
	}
	c.modified = true
	return nil
}

// Save persists the cache to disk if it has been modified.
func (c *FileCache) Save() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.modified {
		return nil
	}

	data, err := json.MarshalIndent(c.entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.path, data, 0644)
}

// Load reads the cache from disk, replacing all in-memory entries.
func (c *FileCache) Load() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	data, err := os.ReadFile(c.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &c.entries)
}

// ── helpers ──────────────────────────────────────────────────

// computeFileHash returns the hex-encoded SHA-256 of the file contents.
func computeFileHash(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}
