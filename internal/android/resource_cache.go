package android

// On-disk cache of parsed values-XML ResourceIndex results. Keyed by a
// content fingerprint over every values/*.xml file in a single res/values
// directory (absolute resDir path + sorted per-file (name, content-hash)
// pairs + requested ValuesScanKind mask). A warm hit skips the
// encoding/xml parse entirely and deserializes a ResourceIndex from
// zstd-wrapped gob.
//
// The cache lives at {repo}/.krit/resource-cache/{hash[:2]}/{hash[2:]}.gob
// with the same sharded layout and cacheutil.SizeCapLRU bookkeeping used
// by the parse cache. Registered with cacheutil so --clear-cache wipes
// it alongside every other on-disk cache.

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

const (
	resourceCacheVersion    uint32 = 2
	resourceCacheVersionStr        = "2"

	resourceCacheDirName = "resource-cache"
	resourceCacheEntries = "entries"
	resourceCacheExt     = ".gob"
	resourceCacheLRUIdx  = "lru-index.gob"
	resourceCacheLRULock = "lru.lock"

	resourceCacheAsyncWorkers = 1
	resourceCacheAsyncQueue   = 256
)

// DefaultResourceCacheCapBytes caps the on-disk footprint of the
// ResourceIndex cache. Sized generously — a Signal-scale values scan
// produces ~30 MB of gob entries across ~200 module dirs; large
// multi-module repos fit in 128 MB.
const DefaultResourceCacheCapBytes int64 = 128 * 1024 * 1024

// resourceCacheEntry is the compressed on-disk gob payload. Version and
// hash algorithm live in sidecars; fingerprint, kind mask, and res dir are
// already represented by the cache key.
type resourceCacheEntry struct {
	Index *ResourceIndex
}

// ResourceIndexCache persists parsed ResourceIndex results keyed by
// per-directory content fingerprint. A nil *ResourceIndexCache is a
// valid disabled cache — every method is a safe no-op.
type ResourceIndexCache struct {
	root       string
	entriesDir string
	lru        *cacheutil.SizeCapLRU
	writer     *cacheutil.AsyncWriter
	dirs       sync.Map

	hits         atomic.Int64
	misses       atomic.Int64
	evictions    atomic.Int64
	lastWriteSec atomic.Int64
}

// NewResourceIndexCache opens (creating if needed) the resource-cache
// directory under repoDir/.krit. A schema-version or hash-algo change
// in the sidecar metadata wipes the entries subtree before returning.
func NewResourceIndexCache(repoDir string) (*ResourceIndexCache, error) {
	return NewResourceIndexCacheWithCap(repoDir, DefaultResourceCacheCapBytes)
}

// NewResourceIndexCacheWithCap is NewResourceIndexCache with an explicit
// byte cap. capBytes <= 0 disables the cap (no eviction).
func NewResourceIndexCacheWithCap(repoDir string, capBytes int64) (*ResourceIndexCache, error) {
	if repoDir == "" {
		return nil, errors.New("android: NewResourceIndexCache requires a non-empty repoDir")
	}
	root := filepath.Join(repoDir, ".krit", resourceCacheDirName)
	vd := cacheutil.VersionedDir{
		Root:       root,
		EntriesDir: resourceCacheEntries,
		Tokens: []cacheutil.SchemaToken{
			{Name: "version", Value: resourceCacheVersionStr},
			{Name: "hash", Value: hashutil.HasherName()},
		},
	}
	entriesDir, err := vd.Open()
	if err != nil {
		return nil, fmt.Errorf("create resource cache dir: %w", err)
	}
	lru := &cacheutil.SizeCapLRU{
		EntriesRoot: entriesDir,
		IndexPath:   filepath.Join(root, resourceCacheLRUIdx),
		LockPath:    filepath.Join(root, resourceCacheLRULock),
		Ext:         resourceCacheExt,
		CapBytes:    capBytes,
	}
	if err := lru.Open(); err != nil {
		return nil, fmt.Errorf("open resource cache lru: %w", err)
	}
	c := &ResourceIndexCache{
		root:       root,
		entriesDir: entriesDir,
		lru:        lru,
		writer:     cacheutil.NewAsyncWriter(resourceCacheAsyncWorkers, resourceCacheAsyncQueue),
	}
	activeResourceCache.Store(c)
	return c, nil
}

// Root returns the on-disk root directory for the resource cache.
// Exposed for diagnostics and tests.
func (c *ResourceIndexCache) Root() string {
	if c == nil {
		return ""
	}
	return c.root
}

func (c *ResourceIndexCache) entryPath(fingerprint string) string {
	return cacheutil.ShardedEntryPath(c.entriesDir, fingerprint, resourceCacheExt)
}

// Load tries to load a cached ResourceIndex for the given fingerprint.
// Returns (index, true) on hit; (nil, false) on miss, corrupt entry,
// or any read/decode error. A nil cache is always a miss.
func (c *ResourceIndexCache) Load(fingerprint string, kinds ValuesScanKind, resDir string) (*ResourceIndex, bool) {
	if c == nil || fingerprint == "" {
		return nil, false
	}
	path := c.entryPath(fingerprint)
	f, err := os.Open(path)
	if err != nil {
		c.misses.Add(1)
		return nil, false
	}
	defer f.Close()

	var entry resourceCacheEntry
	if err := cacheutil.DecodeZstdGob(f, &entry); err != nil {
		// Corrupt entry: drop it so we don't keep re-reading a doomed payload.
		_ = os.Remove(path)
		if c.lru != nil {
			c.lru.Forget(fingerprint)
		}
		c.misses.Add(1)
		c.evictions.Add(1)
		return nil, false
	}
	if entry.Index == nil {
		c.misses.Add(1)
		return nil, false
	}
	if c.lru != nil {
		c.lru.Touch(fingerprint)
	}
	c.hits.Add(1)
	return entry.Index, true
}

// Save persists idx under fingerprint. A write failure is returned but
// callers typically discard it — the next run will just miss.
func (c *ResourceIndexCache) Save(fingerprint string, kinds ValuesScanKind, resDir string, idx *ResourceIndex) error {
	if c == nil || fingerprint == "" || idx == nil {
		return nil
	}
	target := c.entryPath(fingerprint)
	if err := c.ensureEntryDir(filepath.Dir(target)); err != nil {
		return fmt.Errorf("create resource cache shard dir: %w", err)
	}
	entry := resourceCacheEntry{Index: idx}

	blob, err := cacheutil.EncodeZstdGob(entry)
	if err != nil {
		return fmt.Errorf("encode resource cache entry: %w", err)
	}
	size := int64(len(blob))

	if err := fsutil.WriteFileAtomicStream(target, 0o644, func(w io.Writer) error {
		_, werr := w.Write(blob)
		return werr
	}); err != nil {
		return err
	}
	c.lastWriteSec.Store(time.Now().Unix())
	if c.lru != nil {
		c.lru.Record(fingerprint, size)
		if removed, err := c.lru.MaybeEvict(); err != nil {
			// Non-fatal: entry is on disk; cap overshoots this run.
			return nil
		} else if removed > 0 {
			c.evictions.Add(int64(removed))
		}
	}
	return nil
}

// SaveAsync persists idx outside the caller's critical path when the
// background writer has capacity. The ResourceIndex is cloned before queueing
// so later merges or rule execution cannot race with gob encoding.
func (c *ResourceIndexCache) SaveAsync(fingerprint string, kinds ValuesScanKind, resDir string, idx *ResourceIndex) error {
	if c == nil || fingerprint == "" || idx == nil {
		return nil
	}
	idxCopy := cloneResourceIndex(idx)
	if c.writer != nil && c.writer.Submit(func() (int64, error) {
		return 0, c.Save(fingerprint, kinds, resDir, idxCopy)
	}) {
		return nil
	}
	return c.Save(fingerprint, kinds, resDir, idx)
}

func (c *ResourceIndexCache) ensureEntryDir(dir string) error {
	if c == nil {
		return nil
	}
	if _, ok := c.dirs.Load(dir); ok {
		return nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	c.dirs.Store(dir, struct{}{})
	return nil
}

func cloneResourceIndex(idx *ResourceIndex) *ResourceIndex {
	if idx == nil {
		return nil
	}
	out := newResourceIndex()
	for k, v := range idx.Layouts {
		out.Layouts[k] = cloneLayout(v)
	}
	for name, configs := range idx.LayoutConfigs {
		out.LayoutConfigs[name] = make(map[string]*Layout, len(configs))
		for qualifier, layout := range configs {
			out.LayoutConfigs[name][qualifier] = cloneLayout(layout)
		}
	}
	for k, v := range idx.Strings {
		out.Strings[k] = v
	}
	for k, v := range idx.StringsNonTranslate {
		out.StringsNonTranslate[k] = v
	}
	for k, v := range idx.StringsNonFormatted {
		out.StringsNonFormatted[k] = v
	}
	for k, v := range idx.StringsLocation {
		out.StringsLocation[k] = v
	}
	for k, v := range idx.Colors {
		out.Colors[k] = v
	}
	for k, v := range idx.Dimensions {
		out.Dimensions[k] = v
	}
	for k, v := range idx.Styles {
		out.Styles[k] = cloneStyle(v)
	}
	out.Drawables = append(out.Drawables, idx.Drawables...)
	for k, v := range idx.DrawableSelectors {
		out.DrawableSelectors[k] = cloneSelectorItems(v)
	}
	for k, v := range idx.StringArrays {
		out.StringArrays[k] = append([]string(nil), v...)
	}
	for k, v := range idx.Plurals {
		out.Plurals[k] = cloneStringMap(v)
	}
	for k, v := range idx.Integers {
		out.Integers[k] = v
	}
	for k, v := range idx.Booleans {
		out.Booleans[k] = v
	}
	for k, v := range idx.IDs {
		out.IDs[k] = v
	}
	out.ExtraTexts = append(out.ExtraTexts, idx.ExtraTexts...)
	return out
}

func cloneSelectorItems(in []SelectorItem) []SelectorItem {
	out := make([]SelectorItem, len(in))
	for i, item := range in {
		out[i] = item
		if item.StateAttrs != nil {
			out[i].StateAttrs = cloneStringMap(item.StateAttrs)
		}
	}
	return out
}

func cloneLayout(layout *Layout) *Layout {
	if layout == nil {
		return nil
	}
	out := *layout
	out.RootView = cloneView(layout.RootView)
	return &out
}

func cloneView(view *View) *View {
	if view == nil {
		return nil
	}
	out := *view
	if len(view.Attributes) > 0 {
		out.Attributes = make(map[string]string, len(view.Attributes))
		for k, v := range view.Attributes {
			out.Attributes[k] = v
		}
	}
	if len(view.Children) > 0 {
		out.Children = make([]*View, len(view.Children))
		for i, child := range view.Children {
			out.Children[i] = cloneView(child)
		}
	}
	return &out
}

func cloneStyle(style *Style) *Style {
	if style == nil {
		return nil
	}
	out := *style
	out.Items = cloneStringMap(style.Items)
	return &out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

// Clear removes every cache entry. The sidecar schema tokens are left
// in place so a subsequent Open does not see a mismatch.
func (c *ResourceIndexCache) Clear() error {
	if c == nil {
		return nil
	}
	if c.writer != nil {
		_ = c.writer.Close()
	}
	if err := os.RemoveAll(c.entriesDir); err != nil {
		return fmt.Errorf("clear resource cache: %w", err)
	}
	if err := os.MkdirAll(c.entriesDir, 0o755); err != nil {
		return err
	}
	_ = os.Remove(filepath.Join(c.root, resourceCacheLRUIdx))
	if c.lru != nil {
		cap := c.lru.CapBytes
		c.lru = &cacheutil.SizeCapLRU{
			EntriesRoot: c.entriesDir,
			IndexPath:   filepath.Join(c.root, resourceCacheLRUIdx),
			LockPath:    filepath.Join(c.root, resourceCacheLRULock),
			Ext:         resourceCacheExt,
			CapBytes:    cap,
		}
		_ = c.lru.Open()
	}
	c.dirs = sync.Map{}
	return nil
}

// Close flushes async writes and the LRU sidecar. Safe on a nil receiver.
func (c *ResourceIndexCache) Close() error {
	if c == nil {
		return nil
	}
	var errs []error
	if c.writer != nil {
		if err := c.writer.Close(); err != nil {
			errs = append(errs, err)
		}
	}
	if c.lru != nil {
		if err := c.lru.Flush(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Stats returns a unified stats snapshot.
func (c *ResourceIndexCache) Stats() cacheutil.CacheStats {
	if c == nil {
		return cacheutil.CacheStats{}
	}
	var out cacheutil.CacheStats
	if c.lru != nil {
		s := c.lru.Stats()
		out.Entries = s.Entries
		out.Bytes = s.Bytes
	}
	out.Hits = c.hits.Load()
	out.Misses = c.misses.Load()
	out.Evictions = c.evictions.Load()
	out.LastWriteUnix = c.lastWriteSec.Load()
	return out
}

// LRUStats returns the raw LRU snapshot for --perf / tests.
func (c *ResourceIndexCache) LRUStats() cacheutil.LRUStats {
	if c == nil || c.lru == nil {
		return cacheutil.LRUStats{}
	}
	return c.lru.Stats()
}

// valuesDirFile pairs a values/*.xml path with its content bytes.
// Shared between the cache fingerprint path and the parser so a cache
// miss does not re-read the file from disk.
type valuesDirFile struct {
	path    string
	content []byte
}

// computeValuesDirFingerprint builds the per-dir cache key. It expects
// inputs in stable order (caller sorts by path) and mixes in resDir +
// kinds so two directories with byte-identical contents but different
// paths do not collide (paths appear in StringsLocation).
func computeValuesDirFingerprint(resDir string, kinds ValuesScanKind, files []valuesDirFile) string {
	h := hashutil.Hasher().New()
	_, _ = h.Write([]byte("resource-cache-v"))
	_, _ = h.Write([]byte(resourceCacheVersionStr))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(resDir))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strconv.FormatUint(uint64(kinds), 10)))
	_, _ = h.Write([]byte{0})
	for _, f := range files {
		_, _ = h.Write([]byte(filepath.Base(f.path)))
		_, _ = h.Write([]byte{0})
		sum := hashutil.HashBytes(f.content)
		_, _ = h.Write(sum[:])
		_, _ = h.Write([]byte{0})
	}
	raw := h.Sum(nil)
	return hexEncode(raw)
}

// hexEncode is a tiny stdlib-free hex formatter kept local to avoid
// pulling encoding/hex into the hot fingerprint path. All callers
// render lowercase hex so ShardedEntryPath partitions evenly.
func hexEncode(b []byte) string {
	const hextable = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hextable[v>>4]
		out[i*2+1] = hextable[v&0x0f]
	}
	return string(out)
}

// readValuesDirFiles lists and reads every values/*.xml file in dir,
// returning the slice sorted by path. Files are read in parallel up to
// maxWorkers. The returned slice is suitable for both fingerprinting
// and parsing without a second read.
func readValuesDirFiles(dir string, maxWorkers int) ([]valuesDirFile, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("cannot read values dir %s: %w", dir, err)
	}
	paths := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !isValuesXMLFile(e.Name()) {
			continue
		}
		paths = append(paths, filepath.Join(dir, e.Name()))
	}
	if len(paths) == 0 {
		return nil, nil
	}
	sort.Strings(paths)

	out := make([]valuesDirFile, len(paths))
	workers := clampWorkers(maxWorkers, len(paths))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	errs := make([]error, len(paths))
	for i, p := range paths {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, p string) {
			defer wg.Done()
			defer func() { <-sem }()
			data, rerr := os.ReadFile(p)
			if rerr != nil {
				errs[i] = rerr
				return
			}
			out[i] = valuesDirFile{path: p, content: data}
		}(i, p)
	}
	wg.Wait()
	for _, e := range errs {
		if e != nil {
			return nil, e
		}
	}
	return out, nil
}

func isValuesXMLFile(name string) bool {
	return len(name) >= 4 && name[len(name)-4:] == ".xml"
}

// activeResourceCache is the most-recently-constructed cache so the
// cacheutil.Registered stub can surface Stats() without plumbing the
// handle through every caller.
var activeResourceCache atomic.Pointer[ResourceIndexCache]

// ActiveResourceIndexCache returns the most-recently-opened cache, or
// nil when none has been constructed.
func ActiveResourceIndexCache() *ResourceIndexCache {
	return activeResourceCache.Load()
}

// SetActiveResourceIndexCache installs c as the process-global cache
// consulted by scanValuesDirIndexKinds. Passing nil disables cache use
// for the remainder of the process.
func SetActiveResourceIndexCache(c *ResourceIndexCache) {
	activeResourceCache.Store(c)
}

func init() {
	cacheutil.Register(resourceCacheRegistered{})
}

type resourceCacheRegistered struct{}

func (resourceCacheRegistered) Name() string { return resourceCacheDirName }
func (resourceCacheRegistered) Clear(ctx cacheutil.ClearContext) error {
	return ClearResourceIndexCache(ctx.RepoDir)
}
func (resourceCacheRegistered) Stats() cacheutil.CacheStats {
	return activeResourceCache.Load().Stats()
}

// ClearResourceIndexCache removes the resource-cache directory under
// repoDir. Used by --clear-cache; a no-op when the cache dir does not
// exist.
func ClearResourceIndexCache(repoDir string) error {
	if repoDir == "" {
		return nil
	}
	dir := filepath.Join(repoDir, ".krit", resourceCacheDirName)
	if c := activeResourceCache.Load(); c != nil && c.root == dir {
		_ = c.Close()
	}
	err := os.RemoveAll(dir)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clear resource cache: %w", err)
	}
	return nil
}
