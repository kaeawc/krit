package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/scanner"
)

// handleClearCache implements the clear-cache verb. It mirrors the
// in-process semantics of `krit --clear-cache`: every registered
// cacheutil cache is cleared, the analysis cache file is removed,
// and — uniquely to the daemon — the resident WorkspaceState slots
// are dropped so the next analyze rebuilds from cold rather than
// resurrecting state from in-memory snapshots that no longer have a
// disk-cache backing.
//
// The matrix-cache subsystem (internal/cli/scan/matrix_cache.go) only
// auto-registers itself when the scan package is linked. The daemon
// shim binary doesn't link scan, so the matrix cache stays a CLI-side
// concern; --clear-matrix-cache is intentionally NOT routed through
// the daemon. The daemon's clear-cache verb still calls ClearAll()
// because other subsystems that DO get registered from packages the
// daemon imports get cleared this way.
func handleClearCache(_ context.Context, state *daemonState, raw json.RawMessage) (any, error) {
	var args daemon.ClearCacheArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
	}
	if daemonHash := daemonBinaryHash(); args.ClientBinaryHash != "" && daemonHash != "" && args.ClientBinaryHash != daemonHash {
		return nil, fmt.Errorf("%s (daemon=%s client=%s)", daemon.ErrBinaryHashMismatchPrefix, daemonHash, args.ClientBinaryHash)
	}

	// Serialise against any concurrent analyze-project so we don't
	// race a write-back into the file we're about to delete.
	state.analyzeMu.Lock()
	defer state.analyzeMu.Unlock()

	var allErrs []error
	if err := cacheutil.ClearAll(cacheutil.ClearContext{RepoDir: state.repoDir}); err != nil {
		allErrs = append(allErrs, fmt.Errorf("cacheutil.ClearAll: %w", err))
	}

	// Drop the resident analysis-cache table; the next analyze re-
	// loads from disk (which we're about to wipe) and finds nothing,
	// so it rebuilds from scratch.
	state.analysisCacheMu.Lock()
	for _, entry := range state.analysisCacheByKey {
		if entry == nil {
			continue
		}
		if err := cache.Clear(entry.path); err != nil {
			allErrs = append(allErrs, fmt.Errorf("cache.Clear %s: %w", entry.path, err))
		}
	}
	state.analysisCacheByKey = make(map[string]*analysisCacheEntry)
	state.analysisCacheMu.Unlock()

	// Close + reset the resident parse cache so the next analyze
	// reconstructs it against (now-empty) on-disk state.
	state.resetParseCache()

	// Drop every resident WorkspaceState slot so the daemon can't
	// resurrect cached cross-file or per-file state from memory after
	// the disk cache has been wiped.
	state.workspace.InvalidateAll()
	state.coldDone.Store(false)

	state.manifestMu.Lock()
	state.manifestCache = make(map[string]scanner.FindingsBundleManifest)
	state.manifestMu.Unlock()

	if len(allErrs) > 0 {
		return daemon.ClearCacheResult{Cleared: false, ResidentInvalidated: true}, errors.Join(allErrs...)
	}
	return daemon.ClearCacheResult{Cleared: true, ResidentInvalidated: true}, nil
}

// resetParseCache closes the resident parse cache (if any) and rearms
// the sync.Once so a subsequent parseCacheFor call rebuilds from
// scratch. Used by clear-cache to ensure the next analyze sees an
// empty parse cache instead of the in-memory copy from before the
// clear.
func (s *daemonState) resetParseCache() {
	s.closeParseCache()
	s.parseCacheOnce = sync.Once{}
	s.parseCacheErr = nil
}
