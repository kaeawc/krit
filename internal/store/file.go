package store

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
)

func init() {
	cacheutil.Register(storeRegistered{})
}

type storeRegistered struct{}

func (storeRegistered) Name() string { return "unified-store" }
func (storeRegistered) Clear() error {
	// Without a configurable root at init time, clearing must go through
	// the FileStore.Invalidate() method. Phase 2 will wire this up.
	return nil
}

// KindLabel returns a human-readable label for a StoreKind.
func KindLabel(k StoreKind) string {
	return kindName(k)
}

// kindName returns a human-readable label for a StoreKind.
func kindName(k StoreKind) string {
	switch k {
	case KindIncremental:
		return "incremental"
	case KindOracle:
		return "oracle"
	case KindMatrix:
		return "matrix"
	case KindBaseline:
		return "baseline"
	default:
		return fmt.Sprintf("kind%d", k)
	}
}

// FileStore is a file-backed implementation of Store.
//
// Layout under root:
//
//	{root}/{kind}/{fileHash[:2]}/{fileHash[2:]}-{ruleSetHash}.bin
//
// Two-level sharding on the file-content hash keeps no single directory
// larger than 256 children even in large repos.  The ruleSetHash suffix
// allows the same source file to have independent entries for different
// rule configurations.
type FileStore struct {
	root string
}

// New returns a FileStore rooted at dir.  The directory is created on the
// first Put call; New itself does no I/O.
func New(dir string) *FileStore {
	return &FileStore{root: dir}
}

// entryPath maps a Key to an absolute file path inside the store root.
func (s *FileStore) entryPath(key Key) string {
	fh := hex.EncodeToString(key.FileHash[:])
	rs := hex.EncodeToString(key.RuleSetHash[:])
	kindDir := fmt.Sprintf("%d", key.Kind)
	return filepath.Join(s.root, kindDir, fh[:2], fh[2:]+"-"+rs+".bin")
}

// Get retrieves a cached value. Returns (nil, false) on any miss or I/O error.
func (s *FileStore) Get(key Key) ([]byte, bool) {
	data, err := os.ReadFile(s.entryPath(key))
	if err != nil {
		return nil, false
	}
	return data, true
}

// Put stores value at key, atomically replacing any prior entry.
func (s *FileStore) Put(key Key, value []byte) error {
	target := s.entryPath(key)
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("store mkdir: %w", err)
	}
	if err := fsutil.WriteFileAtomic(target, value, 0o644); err != nil {
		return fmt.Errorf("store write: %w", err)
	}
	return nil
}

// Invalidate removes entries from the store.  With no arguments it clears
// everything.  With ruleIDs it removes every entry whose filename contains
// any of the given IDs as a substring of the ruleSetHash hex.
//
// Note: in Phase 0 the ruleSetHash is an opaque hash; per-rule targeting
// requires the codegen registry to embed rule checksums so each rule ID maps
// to a deterministic ruleSetHash prefix.  Until that lands, passing any
// ruleIDs clears the whole store (conservative but always correct).
func (s *FileStore) Invalidate(ruleIDs ...string) error {
	return filepath.Walk(s.root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".bin") {
			return nil
		}
		if len(ruleIDs) == 0 {
			os.Remove(path)
			return nil
		}
		// Conservative: any ruleID match clears the entry.
		for _, id := range ruleIDs {
			if strings.Contains(info.Name(), id) {
				os.Remove(path)
				return nil
			}
		}
		return nil
	})
}

// Stats returns entry count and total byte size by walking the store root,
// broken down by StoreKind (the top-level directory name encodes the kind).
func (s *FileStore) Stats() (StoreStats, error) {
	st := StoreStats{ByKind: make(map[StoreKind]KindStats)}
	err := filepath.Walk(s.root, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			if os.IsNotExist(werr) {
				return nil
			}
			return werr
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".bin") {
			return nil
		}
		st.EntryCount++
		st.TotalBytes += info.Size()

		// Derive kind from the first path component under root.
		rel, relErr := filepath.Rel(s.root, path)
		if relErr == nil {
			parts := strings.SplitN(rel, string(filepath.Separator), 2)
			if len(parts) > 0 {
				if n, parseErr := strconv.ParseUint(parts[0], 10, 8); parseErr == nil {
					k := StoreKind(n)
					ks := st.ByKind[k]
					ks.EntryCount++
					ks.TotalBytes += info.Size()
					st.ByKind[k] = ks
				}
			}
		}
		return nil
	})
	return st, err
}
