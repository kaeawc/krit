package cacheutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kaeawc/krit/internal/buildid"
	"github.com/kaeawc/krit/internal/fsutil"
)

// buildIdentityFile names the sidecar that records which krit build wrote a cache directory.
const buildIdentityFile = "build-identity"

var (
	buildResetMu   sync.Mutex
	buildResetSeen sync.Map // root -> token already reconciled in this process
)

// BuildScopedVersion appends the running build's identity to a cache schema version, for caches that
// already compare a version sidecar. Entries written by any other build then read as a version mismatch.
func BuildScopedVersion(version string) string {
	return version + "|build=" + buildid.Token()
}

// ResetOnBuildChange empties root when it was written by a different krit build, then records the running
// build so the next call is a no-op. Use it for caches whose entries come from krit's own analysis code and
// so go stale when that code changes. A directory that has entries but no record predates this scheme and is
// emptied too, because the build that wrote it is unknown. It returns an error when stale entries could not
// be removed; the caller must then skip the cache.
func ResetOnBuildChange(root string) error {
	return resetOnBuildChange(root, buildid.Token())
}

func resetOnBuildChange(root, token string) error {
	if root == "" {
		return nil
	}
	if seen, ok := buildResetSeen.Load(root); ok && seen == token {
		return nil
	}
	buildResetMu.Lock()
	defer buildResetMu.Unlock()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("cacheutil: mkdir %s: %w", root, err)
	}
	sidecar := filepath.Join(root, buildIdentityFile)
	recorded, readErr := os.ReadFile(sidecar)
	if readErr != nil || string(recorded) != token {
		entries, err := os.ReadDir(root)
		if err != nil {
			return fmt.Errorf("cacheutil: read %s: %w", root, err)
		}
		for _, entry := range entries {
			if entry.Name() == buildIdentityFile {
				continue
			}
			if err := os.RemoveAll(filepath.Join(root, entry.Name())); err != nil {
				return fmt.Errorf("cacheutil: remove stale %s: %w", entry.Name(), err)
			}
		}
		if err := fsutil.WriteFileAtomic(sidecar, []byte(token), 0o644); err != nil {
			return fmt.Errorf("cacheutil: write %s: %w", buildIdentityFile, err)
		}
	}
	buildResetSeen.Store(root, token)
	return nil
}
