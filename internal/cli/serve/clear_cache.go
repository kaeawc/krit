package serve

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/cli/scan"
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
// The matrix-cache subsystem (internal/cli/scan/matrix_cache.go)
// auto-registers itself via init() and is linked into this package
// transitively through internal/cli/serve/meta_verbs.go's scan
// import, so cacheutil.ClearAll() below also wipes the host-wide
// experiment-matrix baseline cache. handleClearMatrixCache exposes
// the same delete as a standalone verb for --clear-matrix-cache,
// which intentionally does NOT also drop resident WorkspaceState.
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

// handleClearMatrixCache implements the clear-matrix-cache verb. It
// removes every entry under ~/.cache/krit/matrix-baseline (the
// host-wide experiment-matrix baseline cache) by delegating to
// scan.ClearMatrixCache, which the in-process --clear-matrix-cache
// path uses too.
//
// Unlike clear-cache, this verb does NOT touch the daemon's resident
// WorkspaceState, analysis cache, parse cache, or manifest cache:
// the matrix cache has no resident counterpart and the experiment-
// matrix runner re-execs the krit binary for every case, so wiping
// the host directory is sufficient to force a recompute on the next
// run.
//
// Concurrency: the matrix cache lives at a host-wide path that may
// be shared with other per-repo daemons. The clear itself is a
// directory scan plus per-entry os.Remove; cross-daemon races are
// non-fatal by design because matrixSave/Load already tolerates
// missing or partial entries (saveBaseline swallows write errors and
// tryLoadBaseline reports any unreadable / mismatched entry as a
// miss, which the matrix runner handles by recomputing). We still
// take state.analyzeMu so this daemon's own clear cannot race with
// an in-flight analyze that might enumerate the cacheutil registry
// at an awkward moment.
func handleClearMatrixCache(_ context.Context, state *daemonState, raw json.RawMessage) (any, error) {
	var args daemon.ClearMatrixCacheArgs
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &args); err != nil {
			return nil, fmt.Errorf("decode args: %w", err)
		}
	}
	if daemonHash := daemonBinaryHash(); args.ClientBinaryHash != "" && daemonHash != "" && args.ClientBinaryHash != daemonHash {
		return nil, fmt.Errorf("%s (daemon=%s client=%s)", daemon.ErrBinaryHashMismatchPrefix, daemonHash, args.ClientBinaryHash)
	}

	state.analyzeMu.Lock()
	defer state.analyzeMu.Unlock()

	if err := scan.ClearMatrixCache(); err != nil {
		return daemon.ClearMatrixCacheResult{Cleared: false}, fmt.Errorf("clear matrix cache: %w", err)
	}
	return daemon.ClearMatrixCacheResult{Cleared: true}, nil
}
