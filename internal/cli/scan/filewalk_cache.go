package scan

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fileignore"
	"github.com/kaeawc/krit/internal/trackedfiles"
)

const (
	filewalkCacheMagic    uint32 = 0x4b465743 // "KFWC"
	filewalkCacheVersion  uint32 = 1
	filewalkCacheDirName         = "filewalk-cache"
	filewalkCacheFileName        = "index.bin"
	filewalkHeaderSize           = 4 + 4 + 4 + 8 // magic + version + crc32 + length (bytes)
)

// FilewalkFilters controls which files are collected and which paths are skipped.
type FilewalkFilters struct {
	Extensions []string // file suffixes to include, e.g. [".kt", ".kts", ".java"]
	Excludes   []string // user-specified glob/substring exclude patterns
}

// Hash returns a stable hex string over the filter configuration. A change
// in any field causes a whole-cache miss on the next warm run.
func (f FilewalkFilters) Hash() string {
	b, _ := json.Marshal(f)
	return fmt.Sprintf("%08x", crc32.ChecksumIEEE(b))
}

// Match reports whether filename matches one of the configured extensions.
func (f FilewalkFilters) Match(name string) bool {
	for _, ext := range f.Extensions {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// isExcluded reports whether path should be skipped. Mirrors the logic in
// internal/scanner that prunes test-data directories and user-specified
// exclusion patterns.
func (f FilewalkFilters) isExcluded(path string) bool {
	if strings.Contains(path, "/test/data/") ||
		strings.Contains(path, "/testData/") ||
		strings.Contains(path, "/testdata/") ||
		strings.Contains(path, "/test-data/") ||
		strings.Contains(path, "/compiler-tests/") ||
		strings.Contains(path, "/compilerTests/") {
		return true
	}
	for _, pattern := range f.Excludes {
		if matched, _ := filepath.Match(pattern, filepath.Base(path)); matched {
			return true
		}
		if strings.Contains(path, strings.Trim(pattern, "*")) {
			return true
		}
	}
	return false
}

// filewalkPayload is the gob-encoded body of the on-disk cache. It is
// zstd-compressed before writing.
type filewalkPayload struct {
	Version uint32
	Roots   []string
	Filters string // FilewalkFilters.Hash() at write time
	Dirs    map[string]filewalkDirEntry
}

// filewalkDirEntry records the directory snapshot used to detect mutations.
// A change in either ModNanos or Size triggers a ReadDir on the next run.
type filewalkDirEntry struct {
	ModNanos int64           // os.Stat mtime in nanoseconds
	Size     int64           // dir size (cheap mutation signal on macOS/Linux)
	Children []filewalkChild // sorted by Name
}

// filewalkChild is one entry inside a cached directory.
type filewalkChild struct {
	Name  string
	IsDir bool
}

func filewalkIndexPath(cacheDir string) string {
	return filepath.Join(cacheDir, filewalkCacheFileName)
}

// loadFilewalkCache reads and validates the on-disk payload. Returns (nil,
// false) on any error, version mismatch, or CRC failure — callers treat all
// misses identically.
func loadFilewalkCache(cacheDir string) (*filewalkPayload, bool) {
	data, err := os.ReadFile(filewalkIndexPath(cacheDir))
	if err != nil {
		return nil, false
	}
	if len(data) < filewalkHeaderSize {
		return nil, false
	}
	magic := binary.BigEndian.Uint32(data[0:4])
	version := binary.BigEndian.Uint32(data[4:8])
	checksum := binary.BigEndian.Uint32(data[8:12])
	length := binary.BigEndian.Uint64(data[12:20])
	if magic != filewalkCacheMagic || version != filewalkCacheVersion {
		return nil, false
	}
	payload := data[filewalkHeaderSize:]
	if uint64(len(payload)) != length {
		return nil, false
	}
	if crc32.ChecksumIEEE(payload) != checksum {
		return nil, false
	}
	var p filewalkPayload
	if err := cacheutil.DecodeZstdGob(bytes.NewReader(payload), &p); err != nil {
		return nil, false
	}
	return &p, true
}

// saveFilewalkCache writes the payload atomically. Write errors are
// non-fatal; callers should discard the return value.
func saveFilewalkCache(cacheDir string, p *filewalkPayload) error {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	encoded, err := cacheutil.EncodeZstdGob(p)
	if err != nil {
		return err
	}
	var hdr [filewalkHeaderSize]byte
	binary.BigEndian.PutUint32(hdr[0:4], filewalkCacheMagic)
	binary.BigEndian.PutUint32(hdr[4:8], filewalkCacheVersion)
	binary.BigEndian.PutUint32(hdr[8:12], crc32.ChecksumIEEE(encoded))
	binary.BigEndian.PutUint64(hdr[12:20], uint64(len(encoded)))

	var buf bytes.Buffer
	buf.Write(hdr[:])
	buf.Write(encoded)
	return os.WriteFile(filewalkIndexPath(cacheDir), buf.Bytes(), 0o644)
}

// CollectFilesCached returns every path under roots whose basename matches
// filters.Match.
//
// Order of preference, fastest first:
//
//  1. git ls-files for each root that is a git work tree top — one fork
//     per root, dominant cost is the fork itself.
//  2. Per-directory mtime cache: when a directory's (mtime, size) are
//     unchanged we replay its cached children list rather than reading.
//
// A nil or empty cacheDir disables the directory-mtime cache (the git fast
// path still runs). Cache write failures are silently ignored so a
// read-only filesystem never breaks a scan.
func CollectFilesCached(roots []string, filters FilewalkFilters, cacheDir string) ([]string, error) {
	return CollectFilesCachedWithIndex(roots, filters, cacheDir, trackedfiles.NewGitIndex())
}

// CollectFilesCachedWithIndex is CollectFilesCached with injectable tracked
// file discovery so multiple scan phases can share one git ls-files result.
func CollectFilesCachedWithIndex(roots []string, filters FilewalkFilters, cacheDir string, index trackedfiles.Index) ([]string, error) {
	out := make([]string, 0, 16384)
	needsWalk := make([]string, 0, len(roots))
	ignoreMatchers := make(map[string]*fileignore.Matcher)

	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			if filters.Match(filepath.Base(root)) && !filters.isExcluded(root) {
				out = append(out, root)
			}
			continue
		}
		// git ls-files already respects .gitignore; krit's matcher
		// only adds .gitignore handling on top, so re-running it on
		// ls-files output is pure waste. User --exclude patterns
		// still apply via filters.isExcluded.
		if index != nil {
			if files, ok := index.Files(root); ok {
				for _, rel := range files {
					if !filters.Match(filepath.Base(rel)) {
						continue
					}
					full := filepath.Join(root, rel)
					if filters.isExcluded(full) {
						continue
					}
					out = append(out, full)
				}
				continue
			}
		}
		needsWalk = append(needsWalk, root)
	}

	if len(needsWalk) > 0 {
		walked, err := walkRootsWithCache(needsWalk, filters, cacheDir, ignoreMatchers)
		if err != nil {
			return nil, err
		}
		out = append(out, walked...)
	}
	return out, nil
}

// walkRootsWithCache is the directory-mtime cached walk path used when
// git ls-files isn't available (non-git roots, missing git binary).
// ignoreMatchers may be shared with the caller so a non-git root nested
// under a git root reuses its parent .gitignore chain.
func walkRootsWithCache(roots []string, filters FilewalkFilters, cacheDir string, ignoreMatchers map[string]*fileignore.Matcher) ([]string, error) {
	var cached *filewalkPayload
	if cacheDir != "" {
		if c, ok := loadFilewalkCache(cacheDir); ok {
			if c.Version == filewalkCacheVersion &&
				c.Filters == filters.Hash() &&
				equalStringSlice(c.Roots, roots) {
				cached = c
			}
		}
	}

	fresh := &filewalkPayload{
		Version: filewalkCacheVersion,
		Roots:   append([]string{}, roots...),
		Filters: filters.Hash(),
		Dirs:    make(map[string]filewalkDirEntry),
	}

	var out []string
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			return nil, err
		}
		matcher := fileignore.MatcherForPath(root, info, ignoreMatchers)
		filewalkDir(root, filters, cached, fresh, matcher, &out)
	}

	if cacheDir != "" {
		_ = saveFilewalkCache(cacheDir, fresh)
	}
	return out, nil
}

// filewalkDir is the recursive per-directory step. On a cache hit it replays
// the cached children list; on a miss it calls ReadDir and records a fresh
// entry for the next run.
func filewalkDir(dir string, filters FilewalkFilters, cached, fresh *filewalkPayload, matcher *fileignore.Matcher, out *[]string) {
	info, err := os.Stat(dir)
	if err != nil {
		return
	}
	if cached != nil {
		if entry, ok := cached.Dirs[dir]; ok && filewalkEntryValid(info, entry) {
			filewalkReplayEntry(dir, entry, filters, cached, fresh, matcher, out)
			return
		}
	}
	filewalkReadDir(dir, info, filters, cached, fresh, matcher, out)
}

// filewalkEntryValid reports whether the cached entry matches the current stat.
func filewalkEntryValid(info os.FileInfo, entry filewalkDirEntry) bool {
	return info.ModTime().UnixNano() == entry.ModNanos && info.Size() == entry.Size
}

// filewalkReplayEntry replays a cache-hit entry, recurring into subdirs.
func filewalkReplayEntry(dir string, entry filewalkDirEntry, filters FilewalkFilters, cached, fresh *filewalkPayload, matcher *fileignore.Matcher, out *[]string) {
	fresh.Dirs[dir] = entry
	for _, c := range entry.Children {
		full := filepath.Join(dir, c.Name)
		if c.IsDir {
			if !fileignore.DefaultPrunedDir(c.Name) && !matcher.Ignored(full, true) {
				filewalkDir(full, filters, cached, fresh, matcher, out)
			}
		} else if filters.Match(c.Name) && !matcher.Ignored(full, false) && !filters.isExcluded(full) {
			*out = append(*out, full)
		}
	}
}

// filewalkReadDir handles a cache miss: reads the directory, records a fresh
// entry, and recurses into subdirs.
func filewalkReadDir(dir string, info os.FileInfo, filters FilewalkFilters, cached, fresh *filewalkPayload, matcher *fileignore.Matcher, out *[]string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	fe := filewalkDirEntry{
		ModNanos: info.ModTime().UnixNano(),
		Size:     info.Size(),
		Children: make([]filewalkChild, 0, len(entries)),
	}
	for _, e := range entries {
		fe.Children = append(fe.Children, filewalkChild{Name: e.Name(), IsDir: e.IsDir()})
	}
	sort.Slice(fe.Children, func(i, j int) bool { return fe.Children[i].Name < fe.Children[j].Name })
	fresh.Dirs[dir] = fe

	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		if e.IsDir() {
			if !fileignore.DefaultPrunedDir(e.Name()) && !matcher.Ignored(full, true) {
				filewalkDir(full, filters, cached, fresh, matcher, out)
			}
		} else if filters.Match(e.Name()) && !matcher.Ignored(full, false) && !filters.isExcluded(full) {
			*out = append(*out, full)
		}
	}
}

func equalStringSlice(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
