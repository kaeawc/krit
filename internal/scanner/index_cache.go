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
//
// v2: switched payload to a columnar form with a shared string table.
// Every Reference previously serialized its File path in full (~100
// bytes × millions of rows on Signal-scale repos); interning collapses
// that to uint32 indexes and a de-duplicated string list.
const CrossFileCacheVersion = 2

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
}

func crossFileCacheFiles(dir string) crossFileCachePaths {
	return crossFileCachePaths{
		Dir:     dir,
		Meta:    filepath.Join(dir, "meta.json"),
		Symbols: filepath.Join(dir, "payload.gob"),
	}
}

// cachePayload is the interned on-disk form. A shared string table backs
// every File/Name/Kind/Visibility reference so the gob encoding doesn't
// repeat the same ~100-byte file path millions of times.
type cachePayload struct {
	Strings []string
	Syms    packedSymbols
	Refs    packedRefs
}

type packedSymbols struct {
	Name       []uint32 // into Strings
	Kind       []uint32
	Visibility []uint32
	File       []uint32
	Line       []int32
	StartByte  []int32
	EndByte    []int32
	// Flags bits: 1=IsOverride 2=IsTest 4=IsMain
	Flags []uint8
}

type packedRefs struct {
	Name      []uint32
	File      []uint32
	Line      []int32
	InComment []bool
}

type stringInterner struct {
	idx   map[string]uint32
	table []string
}

func newStringInterner(hint int) *stringInterner {
	return &stringInterner{idx: make(map[string]uint32, hint)}
}

func (s *stringInterner) intern(v string) uint32 {
	if i, ok := s.idx[v]; ok {
		return i
	}
	i := uint32(len(s.table))
	s.idx[v] = i
	s.table = append(s.table, v)
	return i
}

func packPayload(symbols []Symbol, refs []Reference) cachePayload {
	// Hint capacity: ~2× file count + small enum set (4 visibilities, ~5 kinds)
	// plus every unique name. Undersized maps grow fine.
	intr := newStringInterner(len(symbols) + 256)

	ps := packedSymbols{
		Name:       make([]uint32, len(symbols)),
		Kind:       make([]uint32, len(symbols)),
		Visibility: make([]uint32, len(symbols)),
		File:       make([]uint32, len(symbols)),
		Line:       make([]int32, len(symbols)),
		StartByte:  make([]int32, len(symbols)),
		EndByte:    make([]int32, len(symbols)),
		Flags:      make([]uint8, len(symbols)),
	}
	for i, s := range symbols {
		ps.Name[i] = intr.intern(s.Name)
		ps.Kind[i] = intr.intern(s.Kind)
		ps.Visibility[i] = intr.intern(s.Visibility)
		ps.File[i] = intr.intern(s.File)
		ps.Line[i] = int32(s.Line)
		ps.StartByte[i] = int32(s.StartByte)
		ps.EndByte[i] = int32(s.EndByte)
		var f uint8
		if s.IsOverride {
			f |= 1
		}
		if s.IsTest {
			f |= 2
		}
		if s.IsMain {
			f |= 4
		}
		ps.Flags[i] = f
	}

	pr := packedRefs{
		Name:      make([]uint32, len(refs)),
		File:      make([]uint32, len(refs)),
		Line:      make([]int32, len(refs)),
		InComment: make([]bool, len(refs)),
	}
	for i, r := range refs {
		pr.Name[i] = intr.intern(r.Name)
		pr.File[i] = intr.intern(r.File)
		pr.Line[i] = int32(r.Line)
		pr.InComment[i] = r.InComment
	}

	return cachePayload{Strings: intr.table, Syms: ps, Refs: pr}
}

func (p cachePayload) unpack() ([]Symbol, []Reference, bool) {
	getStr := func(i uint32) (string, bool) {
		if i >= uint32(len(p.Strings)) {
			return "", false
		}
		return p.Strings[i], true
	}

	n := len(p.Syms.Name)
	if len(p.Syms.Kind) != n || len(p.Syms.Visibility) != n || len(p.Syms.File) != n ||
		len(p.Syms.Line) != n || len(p.Syms.StartByte) != n || len(p.Syms.EndByte) != n ||
		len(p.Syms.Flags) != n {
		return nil, nil, false
	}
	symbols := make([]Symbol, n)
	for i := 0; i < n; i++ {
		name, ok1 := getStr(p.Syms.Name[i])
		kind, ok2 := getStr(p.Syms.Kind[i])
		vis, ok3 := getStr(p.Syms.Visibility[i])
		file, ok4 := getStr(p.Syms.File[i])
		if !(ok1 && ok2 && ok3 && ok4) {
			return nil, nil, false
		}
		f := p.Syms.Flags[i]
		symbols[i] = Symbol{
			Name:       name,
			Kind:       kind,
			Visibility: vis,
			File:       file,
			Line:       int(p.Syms.Line[i]),
			StartByte:  int(p.Syms.StartByte[i]),
			EndByte:    int(p.Syms.EndByte[i]),
			IsOverride: f&1 != 0,
			IsTest:     f&2 != 0,
			IsMain:     f&4 != 0,
		}
	}

	m := len(p.Refs.Name)
	if len(p.Refs.File) != m || len(p.Refs.Line) != m || len(p.Refs.InComment) != m {
		return nil, nil, false
	}
	refs := make([]Reference, m)
	for i := 0; i < m; i++ {
		name, ok1 := getStr(p.Refs.Name[i])
		file, ok2 := getStr(p.Refs.File[i])
		if !(ok1 && ok2) {
			return nil, nil, false
		}
		refs[i] = Reference{
			Name:      name,
			File:      file,
			Line:      int(p.Refs.Line[i]),
			InComment: p.Refs.InComment[i],
		}
	}
	return symbols, refs, true
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
	var packed cachePayload
	if err := decodeGob(paths.Symbols, &packed); err != nil {
		return nil, nil, false
	}
	return packed.unpack()
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

	packed := packPayload(symbols, refs)
	if err := encodeGob(paths.Symbols, packed); err != nil {
		return fmt.Errorf("write payload: %w", err)
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
