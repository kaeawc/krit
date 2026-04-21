package scanner

// On-disk cache of tree-sitter FlatTree results keyed by
// SHA-256(file_content). Invalidation is implicit: the content hash
// changes with any byte of the file, and the grammar version stored on
// each entry makes a tree-sitter-kotlin bump nuke every entry it ever
// wrote.

import (
	"bytes"
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

const (
	parseCacheVersion    uint32 = 2
	parseCacheVersionStr        = "2"

	// Files below this threshold parse in under a millisecond; the gob
	// serialization + filesystem round-trip dominates the savings.
	// Confirmed by BenchmarkParseCacheSweep_* (issue #299): 1024 B is
	// the knee where caching amortizes within ~10 runs. Lower thresholds
	// need 20–40 runs to pay back the fsync cost.
	parseCacheMinFileSize = 1024

	parseCacheDirName  = "parse-cache"
	parseCacheEntries  = "entries"
	parseCacheExt      = ".gob"
	parseCacheLRUIndex = "lru-index.gob"
	parseCacheLRULock  = "lru.lock"
)

// parseCacheEntry is the on-disk gob payload. NodeTypeTable maps the
// entry's local FlatNode.Type indices back to node-type strings so a
// reader can re-intern them into its own global NodeTypeTable — crucial
// because the type table grows lazily and a fresh process's global
// indices won't match the writer's.
type parseCacheEntry struct {
	Version       uint32
	GrammarVer    string
	ContentHash   string
	NodeTypeTable []string
	Nodes         []FlatNode
}

// ParseCache persists FlatTree parse results keyed by content hash.
// A nil *ParseCache is a valid disabled cache — every method is a
// safe no-op.
//
// The optional size cap is enforced via an LRU sidecar index; when the
// on-disk total exceeds lru.CapBytes, Save evicts the least-recently
// accessed entries down to LowWaterFrac (80%) of the cap.
type ParseCache struct {
	dir string
	lru *cacheutil.SizeCapLRU
}

var (
	grammarVerOnce sync.Once
	grammarVer     string
)

// GrammarVersion returns a stable identifier for the tree-sitter grammar
// binding in use. Included on every cache entry so a grammar upgrade
// silently invalidates prior entries.
func GrammarVersion() string {
	grammarVerOnce.Do(func() {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, dep := range info.Deps {
				if dep.Path == "github.com/smacker/go-tree-sitter" {
					grammarVer = "smacker/go-tree-sitter@" + dep.Version
					return
				}
			}
		}
		grammarVer = "smacker/go-tree-sitter@unknown"
	})
	return grammarVer
}

// NewParseCache returns a ParseCache rooted at repoDir/.krit/parse-cache.
// A schema-version or grammar-version mismatch in the existing metadata
// clears the entries subtree. The default size cap
// (cacheutil.DefaultParseCacheCapBytes) is applied.
func NewParseCache(repoDir string) (*ParseCache, error) {
	return NewParseCacheWithCap(repoDir, cacheutil.DefaultParseCacheCapBytes)
}

// NewParseCacheWithCap is NewParseCache with an explicit byte cap.
// capBytes <= 0 disables the cap (no eviction).
func NewParseCacheWithCap(repoDir string, capBytes int64) (*ParseCache, error) {
	if repoDir == "" {
		return nil, errors.New("scanner: NewParseCache requires a non-empty repoDir")
	}
	dir := filepath.Join(repoDir, ".krit", parseCacheDirName)
	vd := cacheutil.VersionedDir{
		Root:       dir,
		EntriesDir: parseCacheEntries,
		Tokens: []cacheutil.SchemaToken{
			{Name: "version", Value: parseCacheVersionStr},
			{Name: "grammar-version", Value: GrammarVersion()},
			{Name: "hash", Value: hashutil.HasherName()},
		},
	}
	entriesDir, err := vd.Open()
	if err != nil {
		return nil, fmt.Errorf("create parse cache dir: %w", err)
	}
	lru := &cacheutil.SizeCapLRU{
		EntriesRoot: entriesDir,
		IndexPath:   filepath.Join(dir, parseCacheLRUIndex),
		LockPath:    filepath.Join(dir, parseCacheLRULock),
		Ext:         parseCacheExt,
		CapBytes:    capBytes,
	}
	if err := lru.Open(); err != nil {
		return nil, fmt.Errorf("open parse cache lru: %w", err)
	}
	return &ParseCache{dir: dir, lru: lru}, nil
}

// Dir returns the root directory of the on-disk cache.
func (pc *ParseCache) Dir() string {
	if pc == nil {
		return ""
	}
	return pc.dir
}

// entryPath returns the sharded on-disk path for a content hash.
// Layout: entries/{hash[:2]}/{hash[2:]}.gob — two-level sharding so no
// single directory grows past 256 shards even on huge repos.
func (pc *ParseCache) entryPath(hash string) string {
	return cacheutil.ShardedEntryPath(filepath.Join(pc.dir, parseCacheEntries), hash, parseCacheExt)
}

// Load tries to load a cached FlatTree for the given content. Returns
// (tree, true) on hit, (nil, false) on miss, small file, or any
// read/decode error. A nil ParseCache is always a miss. When path is
// non-empty, the content hash is also recorded in the shared
// hashutil.Memo so downstream subsystems (cross-file index, oracle,
// incremental cache) reuse it without re-reading or re-hashing.
func (pc *ParseCache) Load(path string, content []byte) (*FlatTree, bool) {
	if pc == nil {
		return nil, false
	}
	if len(content) < parseCacheMinFileSize {
		return nil, false
	}
	hash := hashutil.Default().HashContent(path, content)
	return pc.loadByHash(hash)
}

func (pc *ParseCache) loadByHash(hash string) (*FlatTree, bool) {
	path := pc.entryPath(hash)
	f, err := os.Open(path)
	if err != nil {
		return nil, false
	}
	defer f.Close()

	var entry parseCacheEntry
	if err := gob.NewDecoder(f).Decode(&entry); err != nil {
		// Corrupt entry: drop it so we don't keep re-reading a doomed
		// payload on every run.
		_ = os.Remove(path)
		if pc.lru != nil {
			pc.lru.Forget(hash)
		}
		return nil, false
	}
	if entry.Version != parseCacheVersion ||
		entry.GrammarVer != GrammarVersion() ||
		entry.ContentHash != hash {
		return nil, false
	}

	if pc.lru != nil {
		pc.lru.Touch(hash)
	}
	remapEntryNodes(entry.Nodes, entry.NodeTypeTable)
	return &FlatTree{Nodes: entry.Nodes}, true
}

// remapEntryNodes rewrites each node's Type from the entry's local
// index (into localTable) to the current process's global NodeTypeTable
// index. Done in place on the node slice returned in the entry.
func remapEntryNodes(nodes []FlatNode, localTable []string) {
	if len(localTable) == 0 {
		return
	}
	remap := make([]uint16, len(localTable))
	for i, name := range localTable {
		remap[i] = internNodeType(name)
	}
	for i := range nodes {
		if int(nodes[i].Type) < len(remap) {
			nodes[i].Type = remap[nodes[i].Type]
		}
	}
}

// Save persists the parse result for content under its SHA-256. Small
// files are skipped. A returned error means the write failed and the
// next run will miss; callers typically discard it.
func (pc *ParseCache) Save(path string, content []byte, tree *FlatTree) error {
	if pc == nil || tree == nil {
		return nil
	}
	if len(content) < parseCacheMinFileSize {
		return nil
	}
	return pc.saveEntry(hashutil.Default().HashContent(path, content), tree)
}

func (pc *ParseCache) saveEntry(hash string, tree *FlatTree) error {
	local, cloned := buildLocalTableAndNodes(tree.Nodes)

	target := pc.entryPath(hash)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create cache shard dir: %w", err)
	}
	entry := parseCacheEntry{
		Version:       parseCacheVersion,
		GrammarVer:    GrammarVersion(),
		ContentHash:   hash,
		NodeTypeTable: local,
		Nodes:         cloned,
	}

	// Encode into a buffer first so we know the byte size before
	// touching disk: the LRU bookkeeping needs the final on-disk size
	// without a post-write stat round-trip.
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(entry); err != nil {
		return fmt.Errorf("encode cache entry: %w", err)
	}
	size := int64(buf.Len())

	if err := fsutil.WriteFileAtomicStream(target, 0o644, func(w io.Writer) error {
		_, werr := w.Write(buf.Bytes())
		return werr
	}); err != nil {
		return err
	}

	if pc.lru != nil {
		pc.lru.Record(hash, size)
		if _, err := pc.lru.MaybeEvict(); err != nil {
			// Eviction failure is non-fatal: the entry was written,
			// the cap is just overshooting. Next run will retry.
			return nil
		}
	}
	return nil
}

// buildLocalTableAndNodes walks nodes, collects the set of global Type
// IDs actually used, and produces a parallel array of their string names
// plus a clone of the node slice with Type rewritten to indices into
// that local table.
func buildLocalTableAndNodes(nodes []FlatNode) ([]string, []FlatNode) {
	var maxType uint16
	for _, n := range nodes {
		if n.Type > maxType {
			maxType = n.Type
		}
	}
	// Dense lookup: globalToLocal[g] == 0 is the "unseen" sentinel, so
	// stored slots are offset by 1 and subtracted on read.
	globalToLocal := make([]uint16, int(maxType)+1)
	local := make([]string, 0, 32)
	cloned := make([]FlatNode, len(nodes))
	copy(cloned, nodes)
	for i := range cloned {
		g := cloned[i].Type
		slot := globalToLocal[g]
		if slot == 0 {
			local = append(local, nodeTypeName(g))
			slot = uint16(len(local))
			globalToLocal[g] = slot
		}
		cloned[i].Type = slot - 1
	}
	return local, cloned
}

// Clear removes every cache entry. The version / grammar-version
// metadata files are left in place so a subsequent NewParseCache call
// does not see a schema mismatch.
func (pc *ParseCache) Clear() error {
	if pc == nil {
		return nil
	}
	entries := filepath.Join(pc.dir, parseCacheEntries)
	if err := os.RemoveAll(entries); err != nil {
		return fmt.Errorf("clear parse cache: %w", err)
	}
	if err := os.MkdirAll(entries, 0o755); err != nil {
		return err
	}
	// Drop the sidecar index so it doesn't retain phantom entries.
	_ = os.Remove(filepath.Join(pc.dir, parseCacheLRUIndex))
	if pc.lru != nil {
		pc.lru = &cacheutil.SizeCapLRU{
			EntriesRoot: entries,
			IndexPath:   filepath.Join(pc.dir, parseCacheLRUIndex),
			LockPath:    filepath.Join(pc.dir, parseCacheLRULock),
			Ext:         parseCacheExt,
			CapBytes:    pc.lru.CapBytes,
		}
		_ = pc.lru.Open()
	}
	return nil
}

// Close flushes the LRU sidecar. Safe to call multiple times; a nil
// ParseCache Close is a no-op so callers can always defer it.
func (pc *ParseCache) Close() error {
	if pc == nil || pc.lru == nil {
		return nil
	}
	return pc.lru.Flush()
}

// Stats returns the current LRU snapshot. Empty when the cache is
// disabled (nil receiver) or has no LRU attached.
func (pc *ParseCache) Stats() cacheutil.LRUStats {
	if pc == nil || pc.lru == nil {
		return cacheutil.LRUStats{}
	}
	return pc.lru.Stats()
}

func init() {
	cacheutil.Register(parseCacheRegistered{})
}

type parseCacheRegistered struct{}

func (parseCacheRegistered) Name() string                              { return parseCacheDirName }
func (parseCacheRegistered) Clear(ctx cacheutil.ClearContext) error    { return ClearParseCache(ctx.RepoDir) }

// ClearParseCache removes the parse-cache directory under repoDir.
// Used by --clear-cache at the CLI boundary; a no-op when the cache
// directory does not exist.
func ClearParseCache(repoDir string) error {
	if repoDir == "" {
		return nil
	}
	dir := filepath.Join(repoDir, ".krit", parseCacheDirName)
	err := os.RemoveAll(dir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear parse cache: %w", err)
	}
	return nil
}
