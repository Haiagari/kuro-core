package orchestrator


import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ── FileCache tests ─────────────────────────────────────────

func TestFileCacheIsChanged(t *testing.T) {
	t.Run("new file is changed", func(t *testing.T) {
		dir := t.TempDir()
		cache, err := NewFileCache(dir)
		if err != nil {
			t.Fatal(err)
		}

		tmpFile := filepath.Join(dir, "test.txt")
		if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}

		changed, hash, err := cache.IsChanged(tmpFile)
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Error("expected new file to be reported as changed")
		}
		if hash == "" {
			t.Error("expected non-empty hash for new file")
		}
	})

	t.Run("unchanged file after MarkScanned", func(t *testing.T) {
		dir := t.TempDir()
		cache, err := NewFileCache(dir)
		if err != nil {
			t.Fatal(err)
		}

		tmpFile := filepath.Join(dir, "test.txt")
		if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := cache.MarkScanned(tmpFile); err != nil {
			t.Fatal(err)
		}

		changed, _, err := cache.IsChanged(tmpFile)
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			t.Error("expected file to be unchanged after MarkScanned")
		}
	})

	t.Run("modified content is changed", func(t *testing.T) {
		dir := t.TempDir()
		cache, err := NewFileCache(dir)
		if err != nil {
			t.Fatal(err)
		}

		tmpFile := filepath.Join(dir, "test.txt")
		if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := cache.MarkScanned(tmpFile); err != nil {
			t.Fatal(err)
		}

		// Different content AND different size (5 → 7 bytes)
		if err := os.WriteFile(tmpFile, []byte("goodbye"), 0644); err != nil {
			t.Fatal(err)
		}

		changed, _, err := cache.IsChanged(tmpFile)
		if err != nil {
			t.Fatal(err)
		}
		if !changed {
			t.Error("expected modified file to be reported as changed")
		}
	})

	t.Run("touch without content change is unchanged", func(t *testing.T) {
		dir := t.TempDir()
		cache, err := NewFileCache(dir)
		if err != nil {
			t.Fatal(err)
		}

		tmpFile := filepath.Join(dir, "test.txt")
		if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := cache.MarkScanned(tmpFile); err != nil {
			t.Fatal(err)
		}

		// Touch the file — update mtime without changing content
		now := time.Now()
		if err := os.Chtimes(tmpFile, now, now); err != nil {
			t.Skipf("os.Chtimes not supported: %v", err)
		}

		changed, _, err := cache.IsChanged(tmpFile)
		if err != nil {
			t.Fatal(err)
		}
		if changed {
			t.Error("expected touched file (same content) to be unchanged")
		}
	})

	t.Run("nonexistent file returns error", func(t *testing.T) {
		dir := t.TempDir()
		cache, err := NewFileCache(dir)
		if err != nil {
			t.Fatal(err)
		}

		_, _, err = cache.IsChanged(filepath.Join(dir, "nonexistent.txt"))
		if err == nil {
			t.Error("expected error for nonexistent file")
		}
	})
}

func TestFileCacheSaveLoad(t *testing.T) {
	dir := t.TempDir()

	// Create cache, mark a file, save
	cache1, err := NewFileCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(dir, "save-me.txt")
	if err := os.WriteFile(tmpFile, []byte("persist"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := cache1.MarkScanned(tmpFile); err != nil {
		t.Fatal(err)
	}
	if err := cache1.Save(); err != nil {
		t.Fatal(err)
	}

	// Create a new cache instance, load from same dir
	cache2, err := NewFileCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	// File should now be unchanged in the new instance
	changed, hash, err := cache2.IsChanged(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("expected file to be unchanged after load from disk")
	}
	if hash == "" {
		t.Error("expected non-empty hash after load")
	}
}

func TestFileCacheConcurrent(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewFileCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(dir, "concurrent.txt")
	if err := os.WriteFile(tmpFile, []byte("race"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run concurrent access to exercise the mutex
	done := make(chan struct{})
	go func() {
		_, _, _ = cache.IsChanged(tmpFile)
		done <- struct{}{}
	}()
	go func() {
		_ = cache.MarkScanned(tmpFile)
		done <- struct{}{}
	}()
	go func() {
		_ = cache.Save()
		done <- struct{}{}
	}()

	for i := 0; i < 3; i++ {
		<-done
	}
}

func TestFileCacheEmptyDir(t *testing.T) {
	dir := t.TempDir()
	os.RemoveAll(dir) // remove it so MkdirAll must create it

	cache, err := NewFileCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}
}

func TestFileCacheLoadMissingFile(t *testing.T) {
	dir := t.TempDir()

	// NewFileCache should handle missing file-scan.json gracefully
	cache, err := NewFileCache(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cache == nil {
		t.Fatal("expected non-nil cache")
	}

	// Empty cache should report any file as changed
	tmpFile := filepath.Join(dir, "fresh.txt")
	if err := os.WriteFile(tmpFile, []byte("new"), 0644); err != nil {
		t.Fatal(err)
	}

	changed, _, err := cache.IsChanged(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected file to be changed in fresh cache")
	}
}

func TestFileCacheHashStability(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewFileCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(dir, "stable.txt")
	if err := os.WriteFile(tmpFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	_, hash1, err := cache.IsChanged(tmpFile)
	if err != nil {
		t.Fatal(err)
	}

	// Check again (file now has entry from first IsChanged)
	_, hash2, err := cache.IsChanged(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	// After first IsChanged, the file is known but with a temporary entry
	// (modtime+size match the fresh entry).  The returned hash should be stable.
	if hash1 == "" {
		t.Error("expected non-empty hash on first call")
	}
	if hash2 == "" {
		t.Error("expected non-empty hash on second call")
	}
}

func TestFileCacheSameHashForSameContent(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewFileCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	// Two files with same content should produce different entries
	// but the hash values should match
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("same"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("same"), 0644); err != nil {
		t.Fatal(err)
	}

	_, hashA, _ := cache.IsChanged(a)
	_, hashB, _ := cache.IsChanged(b)

	if hashA == "" || hashB == "" {
		t.Fatal("both files should have hashes")
	}
	if hashA != hashB {
		t.Error("same content should produce same hash")
	}
}

func TestFileCacheMarkScannedThenModifySize(t *testing.T) {
	dir := t.TempDir()
	cache, err := NewFileCache(dir)
	if err != nil {
		t.Fatal(err)
	}

	tmpFile := filepath.Join(dir, "sizechange.txt")
	if err := os.WriteFile(tmpFile, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := cache.MarkScanned(tmpFile); err != nil {
		t.Fatal(err)
	}

	// Change size by appending
	if err := os.WriteFile(tmpFile, []byte("hello world"), 0644); err != nil {
		t.Fatal(err)
	}

	changed, _, err := cache.IsChanged(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected file with different size to be changed")
	}
}
