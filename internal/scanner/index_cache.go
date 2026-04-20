package scanner

import (
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// CrossFileCacheVersion is bumped whenever the on-disk layout or the
// Symbol / Reference shapes change. A version mismatch is treated as a
// miss.
const CrossFileCacheVersion = 1

const crossFileCacheDirName = "cross-file-cache"

// CrossFileCacheMeta is persisted alongside the serialized symbols and
// references. JSON-encoded for human inspection.
type CrossFileCacheMeta struct {
	Version        int       `json:"version"`
	Fingerprint    string    `json:"fingerprint"`
	KotlinFiles    int       `json:"kotlin_files"`
	JavaFiles      int       `json:"java_files"`
	XMLFiles       int       `json:"xml_files"`
	SymbolCount    int       `json:"symbol_count"`
	ReferenceCount int       `json:"reference_count"`
	WrittenAt      time.Time `json:"written_at"`
	KritVersion    string    `json:"krit_version,omitempty"`
}

// CrossFileCacheDir returns the cache root for a repo. The directory is
// created lazily on write.
func CrossFileCacheDir(repoDir string) string {
	if repoDir == "" {
		return ""
	}
	return filepath.Join(repoDir, ".krit", crossFileCacheDirName)
}

func contentHashBytes(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

type fingerprintEntry struct {
	Path string
	Hash string
}

// computeCrossFileFingerprint hashes the sorted concatenation of
// "path:content_hash\n" over every contributing file. Sorting makes it
// order-independent.
func computeCrossFileFingerprint(kotlinFiles, javaFiles []*File, xmlFiles []*xmlCacheFile) (string, []fingerprintEntry) {
	entries := make([]fingerprintEntry, 0, len(kotlinFiles)+len(javaFiles)+len(xmlFiles))
	for _, f := range kotlinFiles {
		if f == nil {
			continue
		}
		entries = append(entries, fingerprintEntry{Path: f.Path, Hash: contentHashBytes(f.Content)})
	}
	for _, f := range javaFiles {
		if f == nil {
			continue
		}
		entries = append(entries, fingerprintEntry{Path: f.Path, Hash: contentHashBytes(f.Content)})
	}
	for _, f := range xmlFiles {
		if f == nil {
			continue
		}
		entries = append(entries, fingerprintEntry{Path: f.Path, Hash: f.Hash})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	h := sha256.New()
	for _, e := range entries {
		_, _ = h.Write([]byte(e.Path))
		_, _ = h.Write([]byte{':'})
		_, _ = h.Write([]byte(e.Hash))
		_, _ = h.Write([]byte{'\n'})
	}
	return hex.EncodeToString(h.Sum(nil)), entries
}

type crossFileCachePaths struct {
	Dir     string
	Meta    string
	Symbols string
	Refs    string
}

func crossFileCacheFiles(dir string) crossFileCachePaths {
	return crossFileCachePaths{
		Dir:     dir,
		Meta:    filepath.Join(dir, "meta.json"),
		Symbols: filepath.Join(dir, "symbols.gob"),
		Refs:    filepath.Join(dir, "refs.gob"),
	}
}

// LoadCrossFileCache returns (symbols, refs, true) when the on-disk
// fingerprint matches wantFingerprint. Any other outcome — missing
// files, version mismatch, decode error, fingerprint drift — is a miss.
// A miss is never an error; callers fall back to BuildIndex.
func LoadCrossFileCache(dir, wantFingerprint string) ([]Symbol, []Reference, bool) {
	if dir == "" || wantFingerprint == "" {
		return nil, nil, false
	}
	paths := crossFileCacheFiles(dir)
	metaBytes, err := os.ReadFile(paths.Meta)
	if err != nil {
		return nil, nil, false
	}
	var meta CrossFileCacheMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, nil, false
	}
	if meta.Version != CrossFileCacheVersion || meta.Fingerprint != wantFingerprint {
		return nil, nil, false
	}
	var syms []Symbol
	if err := decodeGob(paths.Symbols, &syms); err != nil {
		return nil, nil, false
	}
	var refs []Reference
	if err := decodeGob(paths.Refs, &refs); err != nil {
		return nil, nil, false
	}
	return syms, refs, true
}

// SaveCrossFileCache writes the symbols and references slices under the
// given fingerprint. Payloads stream directly into tempfiles and are
// atomically renamed so concurrent readers never observe a truncated
// cache.
func SaveCrossFileCache(dir, fingerprint string, meta CrossFileCacheMeta, symbols []Symbol, refs []Reference) error {
	if dir == "" {
		return fmt.Errorf("empty cache dir")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir cache dir: %w", err)
	}
	paths := crossFileCacheFiles(dir)

	meta.Version = CrossFileCacheVersion
	meta.Fingerprint = fingerprint
	if meta.WrittenAt.IsZero() {
		meta.WrittenAt = time.Now().UTC()
	}
	meta.SymbolCount = len(symbols)
	meta.ReferenceCount = len(refs)

	if err := encodeGob(paths.Symbols, symbols); err != nil {
		return fmt.Errorf("write symbols: %w", err)
	}
	if err := encodeGob(paths.Refs, refs); err != nil {
		return fmt.Errorf("write refs: %w", err)
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	if err := writeFileAtomic(paths.Meta, metaBytes, 0o644); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}
	return nil
}

// ClearCrossFileCache removes every file under the cache dir.
func ClearCrossFileCache(dir string) error {
	if dir == "" {
		return nil
	}
	return os.RemoveAll(dir)
}

// encodeGob streams v through gob into a tempfile, then atomically
// renames it into place. Streaming avoids holding the serialized bytes
// in memory alongside the source slice.
func encodeGob(path string, v any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := gob.NewEncoder(tmp).Encode(v); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func decodeGob(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewDecoder(f).Decode(v)
}

// writeFileAtomic writes data to a tempfile in the same directory and
// renames it into place, so concurrent readers never see a truncated
// file.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, perm); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
