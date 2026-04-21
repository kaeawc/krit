package scanner

// On-disk cache of tree-sitter FlatTree results keyed by
// hashutil(file_content). Invalidation is implicit: the content hash
// changes with any byte of the file, and the grammar version stored on
// each entry makes a tree-sitter grammar bump nuke every entry it ever
// wrote. Kotlin and Java live in sibling per-language subdirs so a
// tree-sitter-java bump doesn't evict cached Kotlin trees (and vice
// versa). The on-disk entry payload is language-agnostic — FlatNode
// is the same shape for every tree-sitter grammar.

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

	"github.com/smacker/go-tree-sitter/java"
	"github.com/smacker/go-tree-sitter/kotlin"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

const (
	parseCacheVersion    uint32 = 3
	parseCacheVersionStr        = "3"

	// Files below this threshold parse in under a millisecond; the gob
	// serialization + filesystem round-trip dominates the savings.
	// Confirmed by BenchmarkParseCacheSweep_* (issue #299): 1024 B is
	// the knee where caching amortizes within ~10 runs. Lower thresholds
	// need 20–40 runs to pay back the fsync cost.
	parseCacheMinFileSize = 1024

	parseCacheDirName   = "parse-cache"
	parseCacheEntries   = "entries"
	parseCacheExt       = ".gob"
	parseCacheKotlinDir = "kotlin"
	parseCacheJavaDir   = "java"
	parseCacheLRUIndex  = "lru-index.gob"
	parseCacheLRULock   = "lru.lock"
)

// parseCacheEntry is the on-disk gob payload. NodeTypeTable maps the
// entry's local FlatNode.Type indices back to node-type strings so a
// reader can re-intern them into its own global NodeTypeTable — crucial
// because the type table grows lazily and a fresh process's global
// indices won't match the writer's. Language is stored so a corrupted
// cross-language shard (e.g. a caller mistakenly feeding a Java hash
// into the Kotlin loader) is caught before the tree is handed back.
type parseCacheEntry struct {
	Version       uint32
	GrammarVer    string
	ContentHash   string
	Language      uint8
	NodeTypeTable []string
	Nodes         []FlatNode
}

// ParseCache persists FlatTree parse results keyed by content hash.
// A nil *ParseCache is a valid disabled cache — every method is a
// safe no-op.
//
// Each language holds its own LRU size cap; when a langCache's on-disk
// total exceeds its CapBytes, Save evicts the least-recently-accessed
// entries down to LowWaterFrac (80%) of the cap. Caps are per-language
// so a huge Kotlin corpus doesn't starve the Java cache and vice versa.
type ParseCache struct {
	root   string
	kotlin *langCache
	java   *langCache
}

// langCache is one per-language on-disk cache. Each language has its
// own grammar-version sidecar so a tree-sitter-java upgrade does not
// invalidate cached Kotlin trees and vice versa.
type langCache struct {
	dir        string // {root}/{lang}
	grammarVer string
	language   Language
	lru        *cacheutil.SizeCapLRU
}

var (
	tsDepVersionOnce sync.Once
	tsDepVersion     string

	kotlinGrammarVerOnce sync.Once
	kotlinGrammarVer     string

	javaGrammarVerOnce sync.Once
	javaGrammarVer     string
)

// tsDepVersion returns the smacker/go-tree-sitter module version string,
// or "unknown" when build info is unavailable. Shared across all
// per-language grammar version keys so a dep bump nukes every entry.
func treeSitterDepVersion() string {
	tsDepVersionOnce.Do(func() {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, dep := range info.Deps {
				if dep.Path == "github.com/smacker/go-tree-sitter" {
					tsDepVersion = dep.Version
					return
				}
			}
		}
		tsDepVersion = "unknown"
	})
	return tsDepVersion
}

// GrammarVersion returns the Kotlin grammar identifier. Kept for
// back-compat with callers that pre-date per-language keys; new code
// should prefer KotlinGrammarVersion / JavaGrammarVersion so the
// intent is explicit.
func GrammarVersion() string {
	return KotlinGrammarVersion()
}

// KotlinGrammarVersion returns a stable identifier for the tree-sitter
// Kotlin grammar binding in use. The SymbolCount is appended so a
// regenerated-but-same-dep-version grammar (rare but possible) still
// invalidates cached entries.
func KotlinGrammarVersion() string {
	kotlinGrammarVerOnce.Do(func() {
		kotlinGrammarVer = fmt.Sprintf("smacker/go-tree-sitter@%s#kotlin:%d",
			treeSitterDepVersion(), kotlin.GetLanguage().SymbolCount())
	})
	return kotlinGrammarVer
}

// JavaGrammarVersion returns a stable identifier for the tree-sitter
// Java grammar binding in use. Keyed independently of Kotlin so a
// tree-sitter-java bump evicts only Java cache entries.
func JavaGrammarVersion() string {
	javaGrammarVerOnce.Do(func() {
		javaGrammarVer = fmt.Sprintf("smacker/go-tree-sitter@%s#java:%d",
			treeSitterDepVersion(), java.GetLanguage().SymbolCount())
	})
	return javaGrammarVer
}

// NewParseCache returns a ParseCache rooted at repoDir/.krit/parse-cache.
// A schema-version, hash-algo, or grammar-version mismatch in the
// existing metadata clears the affected language's entries subtree.
// Kotlin and Java are versioned independently. The default per-language
// size cap (cacheutil.DefaultParseCacheCapBytes) is applied.
func NewParseCache(repoDir string) (*ParseCache, error) {
	return NewParseCacheWithCap(repoDir, cacheutil.DefaultParseCacheCapBytes)
}

// NewParseCacheWithCap is NewParseCache with an explicit per-language
// byte cap. capBytes <= 0 disables the cap (no eviction). The cap
// applies to each language's subtree independently so a Kotlin-heavy
// repo doesn't starve Java cached entries.
func NewParseCacheWithCap(repoDir string, capBytes int64) (*ParseCache, error) {
	if repoDir == "" {
		return nil, errors.New("scanner: NewParseCache requires a non-empty repoDir")
	}
	root := filepath.Join(repoDir, ".krit", parseCacheDirName)

	// One-time migration: older krit versions wrote directly to
	// {root}/entries/; drop that layout so stale payloads don't linger
	// alongside the new per-language subdirs. Best-effort; not fatal.
	legacyEntries := filepath.Join(root, parseCacheEntries)
	if fi, err := os.Stat(legacyEntries); err == nil && fi.IsDir() {
		_ = os.RemoveAll(legacyEntries)
	}

	kc, err := newLangCache(root, parseCacheKotlinDir, KotlinGrammarVersion(), LangKotlin, capBytes)
	if err != nil {
		return nil, err
	}
	jc, err := newLangCache(root, parseCacheJavaDir, JavaGrammarVersion(), LangJava, capBytes)
	if err != nil {
		return nil, err
	}
	return &ParseCache{root: root, kotlin: kc, java: jc}, nil
}

func newLangCache(root, sub, grammarVer string, lang Language, capBytes int64) (*langCache, error) {
	dir := filepath.Join(root, sub)
	vd := cacheutil.VersionedDir{
		Root:       dir,
		EntriesDir: parseCacheEntries,
		Tokens: []cacheutil.SchemaToken{
			{Name: "version", Value: parseCacheVersionStr},
			{Name: "grammar-version", Value: grammarVer},
			{Name: "hash", Value: hashutil.HasherName()},
		},
	}
	entriesDir, err := vd.Open()
	if err != nil {
		return nil, fmt.Errorf("create parse cache dir (%s): %w", sub, err)
	}
	lru := &cacheutil.SizeCapLRU{
		EntriesRoot: entriesDir,
		IndexPath:   filepath.Join(dir, parseCacheLRUIndex),
		LockPath:    filepath.Join(dir, parseCacheLRULock),
		Ext:         parseCacheExt,
		CapBytes:    capBytes,
	}
	if err := lru.Open(); err != nil {
		return nil, fmt.Errorf("open parse cache lru (%s): %w", sub, err)
	}
	return &langCache{dir: dir, grammarVer: grammarVer, language: lang, lru: lru}, nil
}

// Dir returns the Kotlin subtree root for the on-disk cache. Kept
// pointing at the Kotlin dir (not the parse-cache parent) so existing
// tests that check for cached Kotlin entries under {Dir}/entries keep
// working. Use Root to get the parent directory containing both
// languages.
func (pc *ParseCache) Dir() string {
	if pc == nil || pc.kotlin == nil {
		return ""
	}
	return pc.kotlin.dir
}

// Root returns the parent directory that contains both language
// subtrees. Exposed for diagnostics; callers that want to target a
// specific language should use the language-specific Load/Save
// entrypoints.
func (pc *ParseCache) Root() string {
	if pc == nil {
		return ""
	}
	return pc.root
}

// JavaDir returns the Java subtree root for the on-disk cache. Empty
// when pc is nil.
func (pc *ParseCache) JavaDir() string {
	if pc == nil || pc.java == nil {
		return ""
	}
	return pc.java.dir
}

// entryPath returns the sharded on-disk path for a Kotlin content hash.
// Layout: kotlin/entries/{hash[:2]}/{hash[2:]}.gob — two-level sharding
// so no single directory grows past 256 shards even on huge repos.
func (pc *ParseCache) entryPath(hash string) string {
	return pc.kotlin.entryPath(hash)
}

// javaEntryPath returns the sharded on-disk path for a Java content
// hash. Mirrors entryPath but rooted at the Java subtree.
func (pc *ParseCache) javaEntryPath(hash string) string {
	return pc.java.entryPath(hash)
}

func (lc *langCache) entryPath(hash string) string {
	return cacheutil.ShardedEntryPath(filepath.Join(lc.dir, parseCacheEntries), hash, parseCacheExt)
}

// Load tries to load a cached Kotlin FlatTree for the given content.
// Returns (tree, true) on hit, (nil, false) on miss, small file, or any
// read/decode error. A nil ParseCache is always a miss. When path is
// non-empty, the content hash is also recorded in the shared
// hashutil.Memo so downstream subsystems (cross-file index, oracle,
// incremental cache) reuse it without re-reading or re-hashing.
func (pc *ParseCache) Load(path string, content []byte) (*FlatTree, bool) {
	if pc == nil {
		return nil, false
	}
	return pc.kotlin.load(path, content)
}

// LoadJava is the Java-language equivalent of Load.
func (pc *ParseCache) LoadJava(path string, content []byte) (*FlatTree, bool) {
	if pc == nil {
		return nil, false
	}
	return pc.java.load(path, content)
}

func (lc *langCache) load(path string, content []byte) (*FlatTree, bool) {
	if lc == nil {
		return nil, false
	}
	if len(content) < parseCacheMinFileSize {
		return nil, false
	}
	hash := hashutil.Default().HashContent(path, content)
	return lc.loadByHash(hash)
}

func (pc *ParseCache) loadByHash(hash string) (*FlatTree, bool) {
	return pc.kotlin.loadByHash(hash)
}

func (pc *ParseCache) loadJavaByHash(hash string) (*FlatTree, bool) {
	return pc.java.loadByHash(hash)
}

func (lc *langCache) loadByHash(hash string) (*FlatTree, bool) {
	path := lc.entryPath(hash)
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
		if lc.lru != nil {
			lc.lru.Forget(hash)
		}
		return nil, false
	}
	if entry.Version != parseCacheVersion ||
		entry.GrammarVer != lc.grammarVer ||
		entry.ContentHash != hash ||
		entry.Language != uint8(lc.language) {
		return nil, false
	}

	if lc.lru != nil {
		lc.lru.Touch(hash)
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

// Save persists the Kotlin parse result for content under its content
// hash. Small files are skipped. A returned error means the write
// failed and the next run will miss; callers typically discard it.
func (pc *ParseCache) Save(path string, content []byte, tree *FlatTree) error {
	if pc == nil || tree == nil {
		return nil
	}
	return pc.kotlin.save(path, content, tree)
}

// SaveJava is the Java-language equivalent of Save.
func (pc *ParseCache) SaveJava(path string, content []byte, tree *FlatTree) error {
	if pc == nil || tree == nil {
		return nil
	}
	return pc.java.save(path, content, tree)
}

func (lc *langCache) save(path string, content []byte, tree *FlatTree) error {
	if lc == nil {
		return nil
	}
	if len(content) < parseCacheMinFileSize {
		return nil
	}
	return lc.saveEntry(hashutil.Default().HashContent(path, content), tree)
}

func (pc *ParseCache) saveEntry(hash string, tree *FlatTree) error {
	return pc.kotlin.saveEntry(hash, tree)
}

func (pc *ParseCache) saveJavaEntry(hash string, tree *FlatTree) error {
	return pc.java.saveEntry(hash, tree)
}

func (lc *langCache) saveEntry(hash string, tree *FlatTree) error {
	local, cloned := buildLocalTableAndNodes(tree.Nodes)

	target := lc.entryPath(hash)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("create cache shard dir: %w", err)
	}
	entry := parseCacheEntry{
		Version:       parseCacheVersion,
		GrammarVer:    lc.grammarVer,
		ContentHash:   hash,
		Language:      uint8(lc.language),
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

	if lc.lru != nil {
		lc.lru.Record(hash, size)
		if _, err := lc.lru.MaybeEvict(); err != nil {
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

// Clear removes every cache entry across both languages. The version /
// grammar-version metadata files are left in place so a subsequent
// NewParseCache call does not see a schema mismatch.
func (pc *ParseCache) Clear() error {
	if pc == nil {
		return nil
	}
	if err := pc.kotlin.clear(); err != nil {
		return err
	}
	if err := pc.java.clear(); err != nil {
		return err
	}
	return nil
}

func (lc *langCache) clear() error {
	if lc == nil {
		return nil
	}
	entries := filepath.Join(lc.dir, parseCacheEntries)
	if err := os.RemoveAll(entries); err != nil {
		return fmt.Errorf("clear parse cache: %w", err)
	}
	if err := os.MkdirAll(entries, 0o755); err != nil {
		return err
	}
	// Drop the sidecar index so it doesn't retain phantom entries.
	_ = os.Remove(filepath.Join(lc.dir, parseCacheLRUIndex))
	if lc.lru != nil {
		cap := lc.lru.CapBytes
		lc.lru = &cacheutil.SizeCapLRU{
			EntriesRoot: entries,
			IndexPath:   filepath.Join(lc.dir, parseCacheLRUIndex),
			LockPath:    filepath.Join(lc.dir, parseCacheLRULock),
			Ext:         parseCacheExt,
			CapBytes:    cap,
		}
		_ = lc.lru.Open()
	}
	return nil
}

// Close flushes the per-language LRU sidecars. Safe to call multiple
// times; a nil ParseCache Close is a no-op so callers can always defer
// it.
func (pc *ParseCache) Close() error {
	if pc == nil {
		return nil
	}
	var errs []error
	if pc.kotlin != nil && pc.kotlin.lru != nil {
		if err := pc.kotlin.lru.Flush(); err != nil {
			errs = append(errs, err)
		}
	}
	if pc.java != nil && pc.java.lru != nil {
		if err := pc.java.lru.Flush(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Stats returns a combined LRU snapshot across both languages. Entries
// and Bytes are summed; Cap reflects the per-language cap (both
// languages share the same configured cap).
func (pc *ParseCache) Stats() cacheutil.LRUStats {
	if pc == nil {
		return cacheutil.LRUStats{}
	}
	var out cacheutil.LRUStats
	if pc.kotlin != nil && pc.kotlin.lru != nil {
		s := pc.kotlin.lru.Stats()
		out.Entries += s.Entries
		out.Bytes += s.Bytes
		out.Cap = s.Cap
	}
	if pc.java != nil && pc.java.lru != nil {
		s := pc.java.lru.Stats()
		out.Entries += s.Entries
		out.Bytes += s.Bytes
		if out.Cap == 0 {
			out.Cap = s.Cap
		}
	}
	return out
}

func init() {
	cacheutil.Register(parseCacheRegistered{})
}

type parseCacheRegistered struct{}

func (parseCacheRegistered) Name() string                           { return parseCacheDirName }
func (parseCacheRegistered) Clear(ctx cacheutil.ClearContext) error { return ClearParseCache(ctx.RepoDir) }

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
