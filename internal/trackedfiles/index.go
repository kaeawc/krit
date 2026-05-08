package trackedfiles

import (
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

// Index provides repository-relative tracked file paths for a root.
// Implementations return ok=false when callers should fall back to a
// filesystem walk.
type Index interface {
	Files(root string) ([]string, bool)
}

// Runner is the pluggable backend used by CachedIndex. Tests can supply a
// fake runner to assert that multiple consumers share one discovery result.
type Runner interface {
	List(root string) ([]string, bool)
}

// MetadataProvider snapshots the git metadata that determines whether a
// tracked-file listing can be reused without invoking git ls-files.
type MetadataProvider interface {
	Snapshot(root string) (Snapshot, bool)
}

// Snapshot is intentionally based on git metadata rather than worktree file
// mtimes. A modified tracked source file does not change the tracked-file set,
// while index or HEAD changes do.
type Snapshot struct {
	Root         string `json:"root"`
	GitDir       string `json:"gitDir"`
	IndexPath    string `json:"indexPath"`
	IndexSize    int64  `json:"indexSize"`
	IndexModTime int64  `json:"indexModTime"`
	Head         string `json:"head"`
}

// CachedIndex memoizes tracked file listings per root for one scan.
type CachedIndex struct {
	runner Runner

	mu    sync.Mutex
	cache map[string]cachedEntry
}

type cachedEntry struct {
	files []string
	ok    bool
}

const (
	trackedFilesCacheDirName = "tracked-files-cache"
	trackedFilesCacheVersion = 1
)

var (
	trackedFilesHits      atomic.Int64
	trackedFilesMisses    atomic.Int64
	trackedFilesBytes     atomic.Int64
	trackedFilesLastWrite atomic.Int64
)

func init() {
	cacheutil.Register(trackedFilesCacheRegistered{})
}

type trackedFilesCacheRegistered struct{}

func (trackedFilesCacheRegistered) Name() string { return trackedFilesCacheDirName }
func (trackedFilesCacheRegistered) Clear(ctx cacheutil.ClearContext) error {
	return ClearCache(CacheDir(ctx.RepoDir))
}
func (trackedFilesCacheRegistered) Stats() cacheutil.CacheStats {
	return cacheutil.CacheStats{
		Bytes:         trackedFilesBytes.Load(),
		Hits:          trackedFilesHits.Load(),
		Misses:        trackedFilesMisses.Load(),
		LastWriteUnix: trackedFilesLastWrite.Load(),
	}
}

// NewGitIndex returns a per-run tracked-file index backed by git ls-files.
func NewGitIndex() *CachedIndex {
	return NewCachedIndex(NewPersistentRunner(GitRunner{}, FileMetadataProvider{}))
}

// NewCachedIndex returns a memoizing index around runner.
func NewCachedIndex(runner Runner) *CachedIndex {
	return &CachedIndex{
		runner: runner,
		cache:  make(map[string]cachedEntry),
	}
}

// Files returns a defensive copy of the tracked files for root.
func (i *CachedIndex) Files(root string) ([]string, bool) {
	if i == nil || i.runner == nil {
		return nil, false
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	i.mu.Lock()
	if entry, ok := i.cache[root]; ok {
		i.mu.Unlock()
		return append([]string(nil), entry.files...), entry.ok
	}
	i.mu.Unlock()

	files, ok := i.runner.List(root)
	entry := cachedEntry{files: append([]string(nil), files...), ok: ok}

	i.mu.Lock()
	i.cache[root] = entry
	i.mu.Unlock()
	return append([]string(nil), entry.files...), entry.ok
}

// GitRunner lists files tracked by the git worktree rooted at root.
type GitRunner struct{}

func (GitRunner) List(root string) ([]string, bool) {
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return nil, false
	}
	gitBin, err := exec.LookPath("git")
	if err != nil {
		return nil, false
	}
	out, err := exec.CommandContext(context.Background(), gitBin, "-C", root, "ls-files", "-z").Output()
	if err != nil {
		return nil, false
	}
	return splitNULFiles(out), true
}

// FileMetadataProvider reads git metadata directly from .git so warm checks do
// not pay the cost of listing the full repository.
type FileMetadataProvider struct{}

func (FileMetadataProvider) Snapshot(root string) (Snapshot, bool) {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	gitDir, ok := gitDirForRoot(root)
	if !ok {
		return Snapshot{}, false
	}
	indexPath := filepath.Join(gitDir, "index")
	info, err := os.Stat(indexPath)
	if err != nil {
		return Snapshot{}, false
	}
	head, ok := readGitHead(gitDir)
	if !ok {
		return Snapshot{}, false
	}
	return Snapshot{
		Root:         root,
		GitDir:       gitDir,
		IndexPath:    indexPath,
		IndexSize:    info.Size(),
		IndexModTime: info.ModTime().UnixNano(),
		Head:         head,
	}, true
}

func gitDirForRoot(root string) (string, bool) {
	gitPath := filepath.Join(root, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return "", false
	}
	if info.IsDir() {
		return gitPath, true
	}
	raw, err := os.ReadFile(gitPath)
	if err != nil {
		return "", false
	}
	const prefix = "gitdir:"
	line := string(bytes.TrimSpace(raw))
	if len(line) < len(prefix) || line[:len(prefix)] != prefix {
		return "", false
	}
	dir := string(bytes.TrimSpace([]byte(line[len(prefix):])))
	if dir == "" {
		return "", false
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	return filepath.Clean(dir), true
}

func readGitHead(gitDir string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(gitDir, "HEAD"))
	if err != nil {
		return "", false
	}
	return string(bytes.TrimSpace(raw)), true
}

// Store persists tracked-file listings by root. Implementations treat all
// decode, version, or metadata mismatches as cache misses.
type Store interface {
	Load(root string, snapshot Snapshot) ([]string, bool)
	Save(root string, snapshot Snapshot, files []string) error
}

// PersistentRunner wraps a Runner with a disk cache. It implements Runner so
// callers can keep using NewCachedIndex for per-run memoization.
type PersistentRunner struct {
	runner Runner
	meta   MetadataProvider
	store  Store
}

func NewPersistentRunner(runner Runner, meta MetadataProvider) *PersistentRunner {
	return NewPersistentRunnerWithStore(runner, meta, nil)
}

func NewPersistentRunnerWithStore(runner Runner, meta MetadataProvider, store Store) *PersistentRunner {
	if store == nil {
		store = DiskStore{}
	}
	return &PersistentRunner{runner: runner, meta: meta, store: store}
}

func (r *PersistentRunner) List(root string) ([]string, bool) {
	if r == nil || r.runner == nil || r.meta == nil || r.store == nil {
		return nil, false
	}
	snapshot, ok := r.meta.Snapshot(root)
	if !ok {
		trackedFilesMisses.Add(1)
		return r.runner.List(root)
	}
	if files, ok := r.store.Load(root, snapshot); ok {
		trackedFilesHits.Add(1)
		return files, true
	}
	trackedFilesMisses.Add(1)
	files, ok := r.runner.List(root)
	if !ok {
		return nil, false
	}
	_ = r.store.Save(root, snapshot, files)
	return files, true
}

// DiskStore stores one listing per repository root.
type DiskStore struct{}

type diskEntry struct {
	Version  int      `json:"version"`
	Snapshot Snapshot `json:"snapshot"`
	Files    []string `json:"files"`
}

func CacheDir(repoDir string) string {
	if repoDir == "" {
		return ""
	}
	return filepath.Join(repoDir, ".krit", trackedFilesCacheDirName)
}

func ClearCache(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	trackedFilesBytes.Store(0)
	return nil
}

func (DiskStore) Load(root string, snapshot Snapshot) ([]string, bool) {
	path := diskEntryPath(root)
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	var entry diskEntry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return nil, false
	}
	if entry.Version != trackedFilesCacheVersion || !sameSnapshot(entry.Snapshot, snapshot) || entry.Files == nil {
		return nil, false
	}
	trackedFilesBytes.Add(int64(len(raw)))
	return append([]string(nil), entry.Files...), true
}

func (DiskStore) Save(root string, snapshot Snapshot, files []string) error {
	path := diskEntryPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.Marshal(diskEntry{
		Version:  trackedFilesCacheVersion,
		Snapshot: snapshot,
		Files:    append([]string(nil), files...),
	})
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(path, raw, 0o644); err != nil {
		return err
	}
	trackedFilesBytes.Add(int64(len(raw)))
	trackedFilesLastWrite.Store(time.Now().Unix())
	return nil
}

func diskEntryPath(root string) string {
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	h := hashutil.Hasher().New()
	_, _ = h.Write([]byte(root))
	key := hex.EncodeToString(h.Sum(nil))
	dir := CacheDir(root)
	if len(key) >= 2 {
		return filepath.Join(dir, "entries", key[:2], key[2:]+".json")
	}
	return filepath.Join(dir, "entries", key+".json")
}

func sameSnapshot(a, b Snapshot) bool {
	return a.Root == b.Root &&
		a.GitDir == b.GitDir &&
		a.IndexPath == b.IndexPath &&
		a.IndexSize == b.IndexSize &&
		a.IndexModTime == b.IndexModTime &&
		a.Head == b.Head
}

func splitNULFiles(out []byte) []string {
	sc := bufio.NewScanner(bytes.NewReader(out))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	sc.Split(splitNUL)
	var files []string
	for sc.Scan() {
		if rel := sc.Text(); rel != "" {
			files = append(files, rel)
		}
	}
	return files
}

func splitNUL(data []byte, atEOF bool) (advance int, token []byte, err error) {
	if i := bytes.IndexByte(data, 0); i >= 0 {
		return i + 1, data[:i], nil
	}
	if atEOF && len(data) > 0 {
		return len(data), data, nil
	}
	return 0, nil, nil
}
