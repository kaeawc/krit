package scanner

import (
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

// Hot-path counters for the cross-file shards cache. Stats are a
// running view of hits, misses, and bytes/entries observed this run
// (loaded or newly written). Pre-existing shards that were never
// touched in the run do not contribute — --verbose would probe the
// disk to supplement.
var (
	shardsHits      atomic.Int64
	shardsMisses    atomic.Int64
	shardsLastWrite atomic.Int64
	shardsBytes     atomic.Int64
	shardsObserved  sync.Map // key -> struct{}: unique shard keys seen this run
	shardsEntries   atomic.Int64
)

func observeShard(key string, size int64) {
	if _, loaded := shardsObserved.LoadOrStore(key, struct{}{}); !loaded {
		shardsEntries.Add(1)
		shardsBytes.Add(size)
	}
}

func init() {
	cacheutil.Register(crossFileShardsRegistered{})
}

type crossFileShardsRegistered struct{}

func (crossFileShardsRegistered) Name() string { return "cross-file-shards" }
func (crossFileShardsRegistered) Clear(ctx cacheutil.ClearContext) error {
	// Shards live under the cross-file cache dir, which the cross-file
	// cache's Clear already removes wholesale. Reset counters so a
	// subsequent Stats() call in the same process reflects the empty
	// state.
	shardsObserved = sync.Map{}
	shardsEntries.Store(0)
	shardsBytes.Store(0)
	return nil
}
func (crossFileShardsRegistered) Stats() cacheutil.CacheStats {
	return cacheutil.CacheStats{
		Entries:       int(shardsEntries.Load()),
		Bytes:         shardsBytes.Load(),
		Hits:          shardsHits.Load(),
		Misses:        shardsMisses.Load(),
		LastWriteUnix: shardsLastWrite.Load(),
	}
}

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
		shardsMisses.Add(1)
		return nil, false
	}
	key := shardKey(path, contentHash)
	p := fileShardPath(cacheDir, key)
	var s fileShard
	if err := decodeGob(p, &s); err != nil {
		shardsMisses.Add(1)
		return nil, false
	}
	if s.Version != crossFileShardVersion || s.Path != path || s.ContentHash != contentHash {
		shardsMisses.Add(1)
		return nil, false
	}
	if fi, err := os.Stat(p); err == nil {
		observeShard(key, fi.Size())
	}
	shardsHits.Add(1)
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
	key := shardKey(s.Path, s.ContentHash)
	p := fileShardPath(cacheDir, key)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return fmt.Errorf("mkdir shard dir: %w", err)
	}
	if err := fsutil.WriteFileAtomicStream(p, 0o644, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(s)
	}); err != nil {
		return err
	}
	shardsLastWrite.Store(time.Now().Unix())
	if fi, err := os.Stat(p); err == nil {
		observeShard(key, fi.Size())
	}
	return nil
}
