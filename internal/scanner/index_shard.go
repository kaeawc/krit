package scanner

import (
	"bytes"
	"compress/gzip"
	"encoding/gob"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
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
//
// v2: added per-shard Bloom payload (gzip-compressed
// bloom.MarshalBinary) so warm-load can union shards' blooms instead of
// re-adding every reference name to a corpus-wide bloom. The shard
// version is versioned independently of CrossFileCacheVersion so a
// shard-layout bump does not force a monolithic rebuild, and vice
// versa.
// v3: force rebuild after FlatFindChild sentinel-collision fix. Pre-fix
// shards contain Symbol.Name values corrupted to the entire file source
// whenever the property/function/class name lookup missed the direct
// simple_identifier child (measured at 49% of symbols on Signal-Android,
// 74% of shard disk bytes).
const crossFileShardVersion = 3

// crossFileShardsSubdir holds sharded, per-file index contributions
// under {CrossFileCacheDir}/{crossFileShardsSubdir}/{hash[:2]}/{hash[2:]}.gob.
const crossFileShardsSubdir = "shards"

// Per-shard bloom sizing. Every shard's bloom uses these exact
// (m, k) parameters so they can be unioned with BloomFilter.Merge,
// which requires identical bit-array sizes and hash counts. The
// aggregate bloom built at warm-load is the same size.
//
// Capacity is sized for a corpus upper bound (~1M unique reference
// names). On Signal (~500K unique) and kotlin/kotlin (~2M unique)
// the post-union FPR lands in the single-digit-percent range; the
// exact false-positive rate is a function of total items stored,
// not shard count, and is documented in the PR body.
const (
	shardBloomCapacity = 1_000_000
	shardBloomFPR      = 0.01
)

// fileShard is one file's contribution to the cross-file index
// (declarations + references). Persisted per-file so a single-file
// edit invalidates only that shard; other files' shards are reused
// from disk.
//
// Symbols is empty for Java / XML shards (reference-only).
//
// Bloom is the gzip-compressed MarshalBinary of a bloom filter sized
// with (shardBloomCapacity, shardBloomFPR). Empty when References is
// empty. Gzip keeps disk footprint small for sparse shards: a bloom
// with ~100 items stored in a 9.6M-bit array compresses to a few
// kilobytes.
type fileShard struct {
	Version     uint32
	Path        string
	ContentHash string
	Symbols     []Symbol
	References  []Reference
	Bloom       []byte
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

// newShardBloom returns a fresh empty bloom sized for union. Every
// shard bloom (and the aggregate produced at load time) uses these
// parameters, so Merge is always legal.
func newShardBloom() *bloom.BloomFilter {
	return bloom.NewWithEstimates(shardBloomCapacity, shardBloomFPR)
}

// buildShardBloomFromRefs returns a bloom populated with each ref's
// Name. Returns nil when refs is empty so empty shards don't pay the
// ~MB empty-bitset cost on disk.
func buildShardBloomFromRefs(refs []Reference) *bloom.BloomFilter {
	if len(refs) == 0 {
		return nil
	}
	bf := newShardBloom()
	for _, r := range refs {
		bf.AddString(r.Name)
	}
	return bf
}

// encodeShardBloom returns gzip(bloom.MarshalBinary). Nil bloom → nil,
// so callers can omit the field for empty shards. The compression is
// a big win for sparse shards (most shards set <0.01% of bits) whose
// raw MarshalBinary is nearly all zeros.
func encodeShardBloom(bf *bloom.BloomFilter) ([]byte, error) {
	if bf == nil {
		return nil, nil
	}
	raw, err := bf.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("marshal bloom: %w", err)
	}
	var buf bytes.Buffer
	gz, err := gzip.NewWriterLevel(&buf, gzip.BestSpeed)
	if err != nil {
		return nil, fmt.Errorf("gzip writer: %w", err)
	}
	if _, err := gz.Write(raw); err != nil {
		_ = gz.Close()
		return nil, fmt.Errorf("gzip write: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("gzip close: %w", err)
	}
	return buf.Bytes(), nil
}

// decodeShardBloom inverts encodeShardBloom. An empty input means the
// shard held no references; returns (nil, nil). A decode failure is a
// real error so callers can treat the shard as unusable.
func decodeShardBloom(data []byte) (*bloom.BloomFilter, error) {
	if len(data) == 0 {
		return nil, nil
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()
	raw, err := io.ReadAll(gz)
	if err != nil {
		return nil, fmt.Errorf("gzip read: %w", err)
	}
	bf := &bloom.BloomFilter{}
	if err := bf.UnmarshalBinary(raw); err != nil {
		return nil, fmt.Errorf("unmarshal bloom: %w", err)
	}
	// Defensive: reject a blob that was written with different
	// (m, k) than the current constants, so Merge doesn't error
	// mid-union on a version-skewed cache.
	probe := newShardBloom()
	if bf.Cap() != probe.Cap() || bf.K() != probe.K() {
		return nil, fmt.Errorf("shard bloom (m=%d,k=%d) != current (m=%d,k=%d)",
			bf.Cap(), bf.K(), probe.Cap(), probe.K())
	}
	return bf, nil
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
