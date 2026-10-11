package orchestrator

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func trivyCacheDir() (string, error) {
	dir := os.Getenv("KURO_TRIVY_CACHE_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".kuro", "trivy")
	}
	return filepath.Abs(dir)
}

// Scans never download databases. Reject missing, incompatible or stale local data.
func validateTrivyDB(dir string, now time.Time) error {
	hint := "run scripts/update-trivy-db.sh on a networked host, then retry"
	db, err := os.Stat(filepath.Join(dir, "db", "trivy.db"))
	if err != nil || !db.Mode().IsRegular() || db.Size() == 0 {
		return fmt.Errorf("Trivy DB unavailable: %s", hint)
	}
	data, err := os.ReadFile(filepath.Join(dir, "db", "metadata.json"))
	if err != nil {
		return fmt.Errorf("Trivy DB metadata unavailable: %s", hint)
	}
	var meta struct {
		Version               int
		UpdatedAt, NextUpdate time.Time
	}
	if err := json.Unmarshal(data, &meta); err != nil {
		return fmt.Errorf("invalid Trivy DB metadata: %w", err)
	}
	if meta.Version != 2 || meta.UpdatedAt.IsZero() || meta.NextUpdate.IsZero() ||
		meta.UpdatedAt.After(now) || !meta.NextUpdate.After(meta.UpdatedAt) ||
		now.Sub(meta.UpdatedAt) > 24*time.Hour || !meta.NextUpdate.After(now) {
		return fmt.Errorf("Trivy DB incompatible or expired: %s", hint)
	}
	return nil
}
