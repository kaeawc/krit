package snapshot

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"github.com/kaeawc/krit/internal/fsutil"
)

// IndexSchemaVersion versions the index.json layout. Bumped when an
// older reader couldn't tolerate a new field; readers that see a
// higher version fall back to the per-manifest scan to stay correct
// across rolling deploys.
const IndexSchemaVersion = 1

const indexFileName = "index.json"

// Index is the rollup of every captured manifest, persisted as a
// single JSON file at <root>/index.json so `snapshot status` and the
// MCP `status` op are O(1) reads instead of O(N) directory walks.
//
// Missing or unreadable index files are not fatal: callers fall back
// to LoadManifests' per-sha scan, so older repos and partially-
// captured snapshot trees keep working.
type Index struct {
	SchemaVersion int        `json:"schema_version"`
	Entries       []Manifest `json:"entries"`
}

// indexPath returns the canonical location of the rollup file.
func indexPath(root string) string {
	return filepath.Join(root, indexFileName)
}

// indexMu serializes index.json read-modify-writes within the same
// process. SaveResult holds it across LoadIndex + write so a backfill
// worker doesn't lose another worker's just-appended entry.
var indexMu sync.Mutex

// LoadIndex reads the rollup at <root>/index.json. Returns (nil, nil)
// when the file does not exist or carries a higher schema version
// than this binary understands — callers fall back to LoadManifests.
func LoadIndex(root string) (*Index, error) {
	data, err := os.ReadFile(indexPath(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("snapshot: read index: %w", err)
	}
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, fmt.Errorf("snapshot: parse index: %w", err)
	}
	if idx.SchemaVersion > IndexSchemaVersion {
		return nil, nil
	}
	return &idx, nil
}

// upsertIndex loads the rollup, replaces or appends the entry for
// m.CommitSHA, sorts by sha, and atomically rewrites the file. Held
// under indexMu so concurrent backfill workers serialise on the
// rollup edit without serialising the rest of capture.
func upsertIndex(root string, m *Manifest) error {
	if m == nil || m.CommitSHA == "" {
		return errors.New("snapshot: upsertIndex: nil or empty-sha manifest")
	}
	indexMu.Lock()
	defer indexMu.Unlock()

	existing, err := LoadIndex(root)
	if err != nil {
		return err
	}
	idx := Index{SchemaVersion: IndexSchemaVersion}
	if existing != nil {
		idx.Entries = existing.Entries
	}
	replaced := false
	for i := range idx.Entries {
		if idx.Entries[i].CommitSHA == m.CommitSHA {
			idx.Entries[i] = *m
			replaced = true
			break
		}
	}
	if !replaced {
		idx.Entries = append(idx.Entries, *m)
	}
	sort.Slice(idx.Entries, func(i, j int) bool {
		return idx.Entries[i].CommitSHA < idx.Entries[j].CommitSHA
	})

	payload, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return fmt.Errorf("snapshot: marshal index: %w", err)
	}
	payload = append(payload, '\n')
	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("snapshot: mkdir %s: %w", root, err)
	}
	if err := fsutil.WriteFileAtomic(indexPath(root), payload, 0o644); err != nil {
		return fmt.Errorf("snapshot: write index: %w", err)
	}
	return nil
}

// removeFromIndex drops the given shas from the rollup atomically.
// Missing entries are silently skipped — Prune calls this after the
// per-sha directory is already gone, and a rerun should be idempotent.
func removeFromIndex(root string, shas ...string) error {
	if len(shas) == 0 {
		return nil
	}
	indexMu.Lock()
	defer indexMu.Unlock()
	existing, err := LoadIndex(root)
	if err != nil {
		return err
	}
	if existing == nil {
		return nil
	}
	drop := make(map[string]struct{}, len(shas))
	for _, s := range shas {
		drop[s] = struct{}{}
	}
	kept := existing.Entries[:0]
	for _, e := range existing.Entries {
		if _, gone := drop[e.CommitSHA]; gone {
			continue
		}
		kept = append(kept, e)
	}
	existing.Entries = kept
	payload, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("snapshot: marshal index: %w", err)
	}
	payload = append(payload, '\n')
	if err := fsutil.WriteFileAtomic(indexPath(root), payload, 0o644); err != nil {
		return fmt.Errorf("snapshot: write index: %w", err)
	}
	return nil
}
