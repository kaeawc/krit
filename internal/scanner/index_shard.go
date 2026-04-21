package scanner

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

// crossFileShardVersion is bumped when the per-file shard payload shape
// changes. A mismatch on load is treated as a miss.
const crossFileShardVersion = 1

// crossFileShardsSubdir holds sharded, per-file index contributions
// under {CrossFileCacheDir}/{crossFileShardsSubdir}/{hash[:2]}/{hash[2:]}.gob.
const crossFileShardsSubdir = "shards"

// fileShard is one file's contribution to the cross-file index
// (declarations + references). Persisted per-file so a single-file
// edit invalidates only that shard; other files' shards are reused
// from disk.
//
// Symbols is empty for Java / XML shards (reference-only).
type fileShard struct {
	Version     uint32
	Path        string
	ContentHash string
	Symbols     []Symbol
	References  []Reference
}

// shardKey identifies a shard by the (path, content-hash) pair so two
// identical-content files at different paths still get separate
// shards. Keying on content alone would cause symbols.File (= path)
// collisions; keying on path alone would require a separate "is the
// content still the same?" check. Combining both keeps the hit check
// purely structural.
func shardKey(path, contentHash string) string {
	return hashutil.HashHex([]byte(path + "\x00" + contentHash))
}

// shardsRoot returns the shards subdirectory under the cross-file
// cache root. Empty cacheDir → empty result.
func shardsRoot(cacheDir string) string {
	if cacheDir == "" {
		return ""
	}
	return filepath.Join(cacheDir, crossFileShardsSubdir)
}

func fileShardPath(cacheDir, key string) string {
	return cacheutil.ShardedEntryPath(shardsRoot(cacheDir), key, ".gob")
}

// loadFileShard returns (shard, true) when a shard for (path, hash)
// exists on disk, decodes cleanly, and matches by path+hash+version.
// Any other outcome is a silent miss.
func loadFileShard(cacheDir, path, contentHash string) (*fileShard, bool) {
	if cacheDir == "" {
		return nil, false
	}
	p := fileShardPath(cacheDir, shardKey(path, contentHash))
	var s fileShard
	if err := decodeGob(p, &s); err != nil {
		return nil, false
	}
	if s.Version != crossFileShardVersion || s.Path != path || s.ContentHash != contentHash {
		return nil, false
	}
	return &s, true
}

// saveFileShard writes s atomically under its shard path. Errors are
// returned to the caller but callers typically log-and-continue:
// a failed persist just means the next run rebuilds the shard.
func saveFileShard(cacheDir string, s *fileShard) error {
	if cacheDir == "" {
		return fmt.Errorf("empty cache dir")
	}
	if s == nil {
		return fmt.Errorf("nil shard")
	}
	s.Version = crossFileShardVersion
	p := fileShardPath(cacheDir, shardKey(s.Path, s.ContentHash))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir shard dir: %w", err)
	}
	return fsutil.WriteFileAtomicStream(p, 0o644, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(s)
	})
}
