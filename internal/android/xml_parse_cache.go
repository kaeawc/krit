package android

// On-disk cache of tree-sitter XML parse results keyed by
// hashutil(file_content). Invalidation is implicit: the content hash
// changes with any byte of the file, and the grammar-version sidecar
// makes a tree-sitter-xml bump nuke every entry it ever wrote.
// Parallels internal/scanner/parse_cache.go for Kotlin/Java but targets
// the *XMLNode tree produced by ParseXMLAST (rather than the FlatNode
// preorder) because every downstream consumer (layout view tree,
// manifest flattening) works directly off XMLNode — caching the semantic
// shape avoids re-running buildXMLNode on a warm hit.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/tsxml"
)

const (
	xmlParseCacheVersion    uint32 = 2
	xmlParseCacheVersionStr        = "2"

	// Most layout XML files in Android repos run 500B–8KB. A 512-byte
	// floor skips the tiniest drawables / shape files where gob encode
	// plus a filesystem round-trip would cost more than the tree-sitter
	// parse itself. Larger than the Kotlin threshold would leave most
	// layouts uncached — the whole point of this cache.
	xmlParseCacheMinFileSize = 512

	xmlParseCacheDirName  = "parse-cache"
	xmlParseCacheSubDir   = "xml"
	xmlParseCacheEntries  = "entries"
	xmlParseCacheExt      = ".gob"
	xmlParseCacheLRUIndex = "lru-index.gob"
	xmlParseCacheLRULock  = "lru.lock"
)

// xmlParseCacheEntry is the compressed on-disk gob payload. The XMLNode
// tree is stored directly — all fields are exported and gob handles the
// recursive Children slice natively. Version, grammar, content hash, and
// language live in sidecars/path rather than being duplicated per entry.
type xmlParseCacheEntry struct {
	Root *XMLNode
}

// XMLParseCache persists ParseXMLAST results keyed by content hash.
// A nil *XMLParseCache is a valid disabled cache — every method is a
// safe no-op so callers can unconditionally invoke Load/Save/Close.
type XMLParseCache struct {
	dir        string
	grammarVer string
	lru        *cacheutil.SizeCapLRU

	hits         atomic.Int64
	misses       atomic.Int64
	evictions    atomic.Int64
	lastWriteSec atomic.Int64
}

var (
	xmlTSDepVersionOnce sync.Once
	xmlTSDepVersion     string

	xmlGrammarVerOnce sync.Once
	xmlGrammarVer     string
)

func xmlTreeSitterDepVersion() string {
	xmlTSDepVersionOnce.Do(func() {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, dep := range info.Deps {
				if dep.Path == "github.com/smacker/go-tree-sitter" {
					xmlTSDepVersion = dep.Version
					return
				}
			}
		}
		xmlTSDepVersion = "unknown"
	})
	return xmlTSDepVersion
}

// XMLGrammarVersion returns a stable identifier for the in-tree
// tree-sitter-xml binding. The SymbolCount is appended so regenerating
// the grammar (even without bumping the go-tree-sitter dep) still
// invalidates cached entries.
func XMLGrammarVersion() string {
	xmlGrammarVerOnce.Do(func() {
		xmlGrammarVer = fmt.Sprintf("internal/tsxml@%s#xml:%d",
			xmlTreeSitterDepVersion(), tsxml.GetLanguage().SymbolCount())
	})
	return xmlGrammarVer
}

// NewXMLParseCache returns a cache rooted at
// repoDir/.krit/parse-cache/xml. Shares the parent parse-cache dir with
// the Kotlin/Java caches so --clear-cache wipes all three at once, but
// its own grammar-version sidecar guarantees a tree-sitter-xml bump
// does not invalidate Kotlin/Java entries and vice versa.
func NewXMLParseCache(repoDir string) (*XMLParseCache, error) {
	return NewXMLParseCacheWithCap(repoDir, cacheutil.DefaultParseCacheCapBytes)
}

// NewXMLParseCacheWithCap is NewXMLParseCache with an explicit byte
// cap. capBytes <= 0 disables eviction.
func NewXMLParseCacheWithCap(repoDir string, capBytes int64) (*XMLParseCache, error) {
	if repoDir == "" {
		return nil, errors.New("android: NewXMLParseCache requires a non-empty repoDir")
	}
	dir := filepath.Join(repoDir, ".krit", xmlParseCacheDirName, xmlParseCacheSubDir)
	grammar := XMLGrammarVersion()
	vd := cacheutil.VersionedDir{
		Root:       dir,
		EntriesDir: xmlParseCacheEntries,
		Tokens: []cacheutil.SchemaToken{
			{Name: "version", Value: xmlParseCacheVersionStr},
			{Name: "grammar-version", Value: grammar},
			{Name: "hash", Value: hashutil.HasherName()},
		},
	}
	entriesDir, err := vd.Open()
	if err != nil {
		return nil, fmt.Errorf("create xml parse cache dir: %w", err)
	}
	lru := &cacheutil.SizeCapLRU{
		EntriesRoot: entriesDir,
		IndexPath:   filepath.Join(dir, xmlParseCacheLRUIndex),
		LockPath:    filepath.Join(dir, xmlParseCacheLRULock),
		Ext:         xmlParseCacheExt,
		CapBytes:    capBytes,
	}
	if err := lru.Open(); err != nil {
		return nil, fmt.Errorf("open xml parse cache lru: %w", err)
	}
	pc := &XMLParseCache{dir: dir, grammarVer: grammar, lru: lru}
	activeXMLParseCache.Store(pc)
	return pc, nil
}

// Dir returns the cache root under .krit/parse-cache/xml. Empty when
// pc is nil.
func (pc *XMLParseCache) Dir() string {
	if pc == nil {
		return ""
	}
	return pc.dir
}

func (pc *XMLParseCache) entryPath(hash string) string {
	return cacheutil.ShardedEntryPath(filepath.Join(pc.dir, xmlParseCacheEntries), hash, xmlParseCacheExt)
}

// Load tries to load a cached XMLNode tree for content. Returns
// (root, true) on hit, (nil, false) on miss, small file, or any
// read/decode error. A nil XMLParseCache is always a miss.
func (pc *XMLParseCache) Load(content []byte) (*XMLNode, bool) {
	if pc == nil {
		return nil, false
	}
	if len(content) < xmlParseCacheMinFileSize {
		return nil, false
	}
	hash := hashutil.Default().HashContent("", content)
	return pc.loadByHash(hash)
}

func (pc *XMLParseCache) loadByHash(hash string) (*XMLNode, bool) {
	path := pc.entryPath(hash)
	f, err := os.Open(path)
	if err != nil {
		pc.misses.Add(1)
		return nil, false
	}
	defer f.Close()

	var entry xmlParseCacheEntry
	if err := cacheutil.DecodeZstdGob(f, &entry); err != nil {
		_ = os.Remove(path)
		if pc.lru != nil {
			pc.lru.Forget(hash)
		}
		pc.misses.Add(1)
		pc.evictions.Add(1)
		return nil, false
	}
	if entry.Root == nil {
		pc.misses.Add(1)
		return nil, false
	}

	if pc.lru != nil {
		pc.lru.Touch(hash)
	}
	pc.hits.Add(1)
	return entry.Root, true
}

// Save persists the parse result for content. Small files are skipped.
// A nil pc or nil root is a no-op.
func (pc *XMLParseCache) Save(content []byte, root *XMLNode) error {
	if pc == nil || root == nil {
		return nil
	}
	if len(content) < xmlParseCacheMinFileSize {
		return nil
	}
	hash := hashutil.Default().HashContent("", content)
	return pc.saveEntry(hash, root)
}

func (pc *XMLParseCache) saveEntry(hash string, root *XMLNode) error {
	target := pc.entryPath(hash)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create xml cache shard dir: %w", err)
	}
	entry := xmlParseCacheEntry{Root: root}

	blob, err := cacheutil.EncodeZstdGob(entry)
	if err != nil {
		return fmt.Errorf("encode xml cache entry: %w", err)
	}
	size := int64(len(blob))

	if err := fsutil.WriteFileAtomicStream(target, 0o644, func(w io.Writer) error {
		_, werr := w.Write(blob)
		return werr
	}); err != nil {
		return err
	}

	pc.lastWriteSec.Store(time.Now().Unix())
	if pc.lru != nil {
		pc.lru.Record(hash, size)
		if removed, err := pc.lru.MaybeEvict(); err != nil {
			// Non-fatal: the entry wrote OK, the cap just overshoots.
			return nil
		} else if removed > 0 {
			pc.evictions.Add(int64(removed))
		}
	}
	return nil
}

// Clear removes every cache entry. The version / grammar-version
// metadata files are left in place so a subsequent NewXMLParseCache
// call does not see a schema mismatch.
func (pc *XMLParseCache) Clear() error {
	if pc == nil {
		return nil
	}
	entries := filepath.Join(pc.dir, xmlParseCacheEntries)
	if err := os.RemoveAll(entries); err != nil {
		return fmt.Errorf("clear xml parse cache: %w", err)
	}
	if err := os.MkdirAll(entries, 0o755); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(pc.dir, xmlParseCacheLRUIndex))
	if pc.lru != nil {
		cap := pc.lru.CapBytes
		pc.lru = &cacheutil.SizeCapLRU{
			EntriesRoot: entries,
			IndexPath:   filepath.Join(pc.dir, xmlParseCacheLRUIndex),
			LockPath:    filepath.Join(pc.dir, xmlParseCacheLRULock),
			Ext:         xmlParseCacheExt,
			CapBytes:    cap,
		}
		_ = pc.lru.Open()
	}
	return nil
}

// Close flushes the LRU sidecar. Safe to call on a nil cache.
func (pc *XMLParseCache) Close() error {
	if pc == nil || pc.lru == nil {
		return nil
	}
	return pc.lru.Flush()
}

// LRUStats returns the current LRU snapshot.
func (pc *XMLParseCache) LRUStats() cacheutil.LRUStats {
	if pc == nil || pc.lru == nil {
		return cacheutil.LRUStats{}
	}
	return pc.lru.Stats()
}

// Stats returns a unified snapshot. Counter fields are running totals
// for the current process; Entries and Bytes come from the LRU sidecar.
func (pc *XMLParseCache) Stats() cacheutil.CacheStats {
	if pc == nil {
		return cacheutil.CacheStats{}
	}
	var out cacheutil.CacheStats
	if pc.lru != nil {
		s := pc.lru.Stats()
		out.Entries = s.Entries
		out.Bytes = s.Bytes
	}
	out.Hits = pc.hits.Load()
	out.Misses = pc.misses.Load()
	out.Evictions = pc.evictions.Load()
	out.LastWriteUnix = pc.lastWriteSec.Load()
	return out
}

// activeXMLParseCache is the most-recently-constructed cache so
// ParseXMLAST can consult it without every caller plumbing a handle
// through. Idempotent-by-Name replacement in the cacheutil registry
// takes care of the cold-start case.
var activeXMLParseCache atomic.Pointer[XMLParseCache]

// ActiveXMLParseCache returns the currently installed cache, or nil
// when caching is disabled. Exposed for diagnostics and tests.
func ActiveXMLParseCache() *XMLParseCache {
	return activeXMLParseCache.Load()
}

// SetActiveXMLParseCache installs pc as the process-wide cache used by
// ParseXMLAST. Passing nil clears the active cache (equivalent to
// disabling the cache). NewXMLParseCache installs itself automatically;
// this helper is for tests and for callers that construct a cache
// lazily.
func SetActiveXMLParseCache(pc *XMLParseCache) {
	activeXMLParseCache.Store(pc)
}

func init() {
	cacheutil.Register(xmlParseCacheRegistered{})
}

type xmlParseCacheRegistered struct{}

func (xmlParseCacheRegistered) Name() string { return "xml-parse-cache" }
func (xmlParseCacheRegistered) Clear(ctx cacheutil.ClearContext) error {
	return ClearXMLParseCache(ctx.RepoDir)
}
func (xmlParseCacheRegistered) Stats() cacheutil.CacheStats {
	pc := activeXMLParseCache.Load()
	if pc == nil {
		return cacheutil.CacheStats{}
	}
	return pc.Stats()
}

// ClearXMLParseCache removes the xml parse cache subtree. Used by
// --clear-cache at the CLI boundary; a no-op when the directory does
// not exist. The Kotlin/Java parse-cache.ClearParseCache also removes
// its parent dir (.krit/parse-cache) on invocation, so either entry
// point is sufficient — both are registered so order doesn't matter.
func ClearXMLParseCache(repoDir string) error {
	if repoDir == "" {
		return nil
	}
	dir := filepath.Join(repoDir, ".krit", xmlParseCacheDirName, xmlParseCacheSubDir)
	err := os.RemoveAll(dir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear xml parse cache: %w", err)
	}
	return nil
}
