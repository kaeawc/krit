package scanner

import (
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"

	"github.com/bits-and-blooms/bloom/v3"
	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

// Hot-path counters for the cross-file index cache. Populated on
// Load/Save success; a fresh process starts at zero (drift across
// restarts is deliberate).
var (
	crossFileHits      atomic.Int64
	crossFileMisses    atomic.Int64
	crossFileEntries   atomic.Int64
	crossFileBytes     atomic.Int64
	crossFileLastWrite atomic.Int64
)

func init() {
	cacheutil.Register(crossFileCacheRegistered{})
}

type crossFileCacheRegistered struct{}

func (crossFileCacheRegistered) Name() string { return crossFileCacheDirName }
func (crossFileCacheRegistered) Clear(ctx cacheutil.ClearContext) error {
	return ClearCrossFileCache(CrossFileCacheDir(ctx.RepoDir))
}
func (crossFileCacheRegistered) Stats() cacheutil.CacheStats {
	return cacheutil.CacheStats{
		Entries:       int(crossFileEntries.Load()),
		Bytes:         crossFileBytes.Load(),
		Hits:          crossFileHits.Load(),
		Misses:        crossFileMisses.Load(),
		LastWriteUnix: crossFileLastWrite.Load(),
	}
}

// recordCrossFileDisk updates the on-disk Entries/Bytes counters from
// the meta + payload paths. Safe to call on any disk state; stat errors
// zero-out the fields.
func recordCrossFileDisk(paths crossFileCachePaths, entries int) {
	var bytes int64
	if fi, err := os.Stat(paths.Meta); err == nil {
		bytes += fi.Size()
	}
	if fi, err := os.Stat(paths.Symbols); err == nil {
		bytes += fi.Size()
	}
	crossFileBytes.Store(bytes)
	crossFileEntries.Store(int64(entries))
}

// CrossFileCacheVersion is bumped whenever the on-disk layout or the
// Symbol / Reference shapes change. A version mismatch is treated as a
// miss.
//
// v2: switched payload to a columnar form with a shared string table.
// Every Reference previously serialized its File path in full (~100
// bytes × millions of rows on Signal-scale repos); interning collapses
// that to uint32 indexes and a de-duplicated string list.
// v4: embedded the assembled lookup maps (symbolsByName, refCountByName,
// refFilesByName, nonCommentRefFilesByName,
// nonCommentRefCountByNameFile) plus the serialized bloom filter so a
// warm hit can skip the lookup-map rebuild phase entirely.
// v5: force rebuild after the FlatFindChild sentinel-collision fix (see
// crossFileShardVersion v3). The monolithic payload mirrors the shard
// data and is similarly corrupted pre-fix.
const CrossFileCacheVersion = 5

// bloomLibraryVersion is the pinned bits-and-blooms/bloom/v3 version.
// It is mixed into the cross-file fingerprint so a library upgrade
// that changes the bloom filter's binary layout nukes the cache.
// Keep this in sync with go.mod.
const bloomLibraryVersion = "bits-and-blooms/bloom/v3@v3.7.1"

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
	return hashutil.HashHex(b)
}

// contentHashForFile returns the content hash of f using the shared
// hashutil.Memo keyed on f.Path, so the cross-file index reuses the
// parse-cache / oracle hash instead of re-computing SHA-256.
func contentHashForFile(path string, content []byte) string {
	return hashutil.Default().HashContent(path, content)
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
		entries = append(entries, fingerprintEntry{Path: f.Path, Hash: contentHashForFile(f.Path, f.Content)})
	}
	for _, f := range javaFiles {
		if f == nil {
			continue
		}
		entries = append(entries, fingerprintEntry{Path: f.Path, Hash: contentHashForFile(f.Path, f.Content)})
	}
	for _, f := range xmlFiles {
		if f == nil {
			continue
		}
		entries = append(entries, fingerprintEntry{Path: f.Path, Hash: f.Hash})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	h := hashutil.Hasher().New()
	_, _ = h.Write([]byte(bloomLibraryVersion))
	_, _ = h.Write([]byte{'\n'})
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
//
// Lookup is optional. When non-empty, the warm-load path feeds it
// directly into the CodeIndex instead of re-building the maps and bloom
// filter from Refs.
type cachePayload struct {
	Strings []string
	Syms    packedSymbols
	Refs    packedRefs
	Lookup  packedLookup
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

// packedLookup stores the assembled lookup maps + bloom filter in a
// form that references the enclosing payload's string table. All keys
// and values that hold strings are uint32 indices into Strings.
//
// An empty packedLookup (zero Has flag) means "not persisted" and the
// loader reconstructs the maps from Refs.
type packedLookup struct {
	// Has is true when the remaining fields are populated. Distinguishes
	// "legitimately empty index" from "legacy payload without maps".
	Has bool

	// map[name] -> []symbolIndex (into Syms arrays)
	SymByNameKeys   []uint32
	SymByNameValues [][]uint32

	// map[name] -> refCount
	RefCountKeys   []uint32
	RefCountValues []uint32

	// map[name] -> set of fileIndex
	RefFilesKeys   []uint32
	RefFilesValues [][]uint32

	// map[name] -> set of fileIndex (non-comment only)
	NCRefFilesKeys   []uint32
	NCRefFilesValues [][]uint32

	// map[name] -> map[file] -> count (non-comment only)
	NCRefCountKeys   []uint32
	NCRefCountFiles  [][]uint32
	NCRefCountValues [][]uint32

	// Bloom is the bloom filter's MarshalBinary output.
	Bloom []byte
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

// packPayloadWithIndex packs the symbols, references, and lookup maps
// from a fully-built CodeIndex. If idx is nil, the lookup section is
// left empty and the warm-load path will fall back to rebuilding.
func packPayloadWithIndex(idx *CodeIndex) cachePayload {
	if idx == nil {
		return packPayload(nil, nil)
	}
	p := packPayload(idx.Symbols, idx.References)
	p.Lookup = packLookup(idx, &p)
	return p
}

func packLookup(idx *CodeIndex, p *cachePayload) packedLookup {
	if idx == nil {
		return packedLookup{}
	}

	intr := &stringInterner{idx: make(map[string]uint32, len(p.Strings)), table: p.Strings}
	for i, s := range intr.table {
		intr.idx[s] = uint32(i)
	}

	// Build a symbol-index lookup keyed by "Name|File|Line|StartByte" so
	// symbolsByName values can reference rows in p.Syms by index.
	type symKey struct {
		Name      uint32
		File      uint32
		Line      int32
		StartByte int32
	}
	symIndexByKey := make(map[symKey]uint32, len(idx.Symbols))
	for i := range idx.Symbols {
		k := symKey{
			Name:      p.Syms.Name[i],
			File:      p.Syms.File[i],
			Line:      p.Syms.Line[i],
			StartByte: p.Syms.StartByte[i],
		}
		if _, ok := symIndexByKey[k]; !ok {
			symIndexByKey[k] = uint32(i)
		}
	}

	out := packedLookup{Has: true}

	// symbolsByName
	out.SymByNameKeys = make([]uint32, 0, len(idx.symbolsByName))
	out.SymByNameValues = make([][]uint32, 0, len(idx.symbolsByName))
	for name, syms := range idx.symbolsByName {
		nameIdx := intr.intern(name)
		rows := make([]uint32, 0, len(syms))
		for _, s := range syms {
			k := symKey{
				Name:      intr.intern(s.Name),
				File:      intr.intern(s.File),
				Line:      int32(s.Line),
				StartByte: int32(s.StartByte),
			}
			if ri, ok := symIndexByKey[k]; ok {
				rows = append(rows, ri)
			}
		}
		out.SymByNameKeys = append(out.SymByNameKeys, nameIdx)
		out.SymByNameValues = append(out.SymByNameValues, rows)
	}

	// refCountByName
	out.RefCountKeys = make([]uint32, 0, len(idx.refCountByName))
	out.RefCountValues = make([]uint32, 0, len(idx.refCountByName))
	for name, count := range idx.refCountByName {
		out.RefCountKeys = append(out.RefCountKeys, intr.intern(name))
		out.RefCountValues = append(out.RefCountValues, uint32(count))
	}

	// refFilesByName
	out.RefFilesKeys = make([]uint32, 0, len(idx.refFilesByName))
	out.RefFilesValues = make([][]uint32, 0, len(idx.refFilesByName))
	for name, files := range idx.refFilesByName {
		vals := make([]uint32, 0, len(files))
		for f := range files {
			vals = append(vals, intr.intern(f))
		}
		out.RefFilesKeys = append(out.RefFilesKeys, intr.intern(name))
		out.RefFilesValues = append(out.RefFilesValues, vals)
	}

	// nonCommentRefFilesByName
	out.NCRefFilesKeys = make([]uint32, 0, len(idx.nonCommentRefFilesByName))
	out.NCRefFilesValues = make([][]uint32, 0, len(idx.nonCommentRefFilesByName))
	for name, files := range idx.nonCommentRefFilesByName {
		vals := make([]uint32, 0, len(files))
		for f := range files {
			vals = append(vals, intr.intern(f))
		}
		out.NCRefFilesKeys = append(out.NCRefFilesKeys, intr.intern(name))
		out.NCRefFilesValues = append(out.NCRefFilesValues, vals)
	}

	// nonCommentRefCountByNameFile
	out.NCRefCountKeys = make([]uint32, 0, len(idx.nonCommentRefCountByNameFile))
	out.NCRefCountFiles = make([][]uint32, 0, len(idx.nonCommentRefCountByNameFile))
	out.NCRefCountValues = make([][]uint32, 0, len(idx.nonCommentRefCountByNameFile))
	for name, byFile := range idx.nonCommentRefCountByNameFile {
		files := make([]uint32, 0, len(byFile))
		counts := make([]uint32, 0, len(byFile))
		for f, c := range byFile {
			files = append(files, intr.intern(f))
			counts = append(counts, uint32(c))
		}
		out.NCRefCountKeys = append(out.NCRefCountKeys, intr.intern(name))
		out.NCRefCountFiles = append(out.NCRefCountFiles, files)
		out.NCRefCountValues = append(out.NCRefCountValues, counts)
	}

	if idx.refBloom != nil {
		if data, err := idx.refBloom.MarshalBinary(); err == nil {
			out.Bloom = data
		}
	}

	p.Strings = intr.table
	return out
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

// unpackFull rebuilds a full CodeIndex (including lookup maps + bloom
// filter) from the on-disk payload. Returns ok=false if any index is
// out of range or the bloom filter fails to decode; the caller must
// then fall back to a rebuild-from-scratch.
func (p cachePayload) unpackFull() (*CodeIndex, bool) {
	symbols, refs, ok := p.unpack()
	if !ok {
		return nil, false
	}
	if !p.Lookup.Has {
		return BuildIndexFromData(symbols, refs), true
	}

	getStr := func(i uint32) (string, bool) {
		if i >= uint32(len(p.Strings)) {
			return "", false
		}
		return p.Strings[i], true
	}

	idx := &CodeIndex{
		Symbols:                      symbols,
		References:                   refs,
		symbolsByName:                make(map[string][]Symbol, len(p.Lookup.SymByNameKeys)),
		refCountByName:               make(map[string]int, len(p.Lookup.RefCountKeys)),
		refFilesByName:               make(map[string]map[string]bool, len(p.Lookup.RefFilesKeys)),
		nonCommentRefFilesByName:     make(map[string]map[string]bool, len(p.Lookup.NCRefFilesKeys)),
		nonCommentRefCountByNameFile: make(map[string]map[string]int, len(p.Lookup.NCRefCountKeys)),
	}

	// symbolsByName
	if len(p.Lookup.SymByNameKeys) != len(p.Lookup.SymByNameValues) {
		return nil, false
	}
	for i, nameIdx := range p.Lookup.SymByNameKeys {
		name, ok := getStr(nameIdx)
		if !ok {
			return nil, false
		}
		rows := p.Lookup.SymByNameValues[i]
		syms := make([]Symbol, 0, len(rows))
		for _, r := range rows {
			if r >= uint32(len(symbols)) {
				return nil, false
			}
			syms = append(syms, symbols[r])
		}
		idx.symbolsByName[name] = syms
	}

	// refCountByName
	if len(p.Lookup.RefCountKeys) != len(p.Lookup.RefCountValues) {
		return nil, false
	}
	for i, nameIdx := range p.Lookup.RefCountKeys {
		name, ok := getStr(nameIdx)
		if !ok {
			return nil, false
		}
		idx.refCountByName[name] = int(p.Lookup.RefCountValues[i])
	}

	// refFilesByName
	if len(p.Lookup.RefFilesKeys) != len(p.Lookup.RefFilesValues) {
		return nil, false
	}
	for i, nameIdx := range p.Lookup.RefFilesKeys {
		name, ok := getStr(nameIdx)
		if !ok {
			return nil, false
		}
		set := make(map[string]bool, len(p.Lookup.RefFilesValues[i]))
		for _, fIdx := range p.Lookup.RefFilesValues[i] {
			f, ok := getStr(fIdx)
			if !ok {
				return nil, false
			}
			set[f] = true
		}
		idx.refFilesByName[name] = set
	}

	// nonCommentRefFilesByName
	if len(p.Lookup.NCRefFilesKeys) != len(p.Lookup.NCRefFilesValues) {
		return nil, false
	}
	for i, nameIdx := range p.Lookup.NCRefFilesKeys {
		name, ok := getStr(nameIdx)
		if !ok {
			return nil, false
		}
		set := make(map[string]bool, len(p.Lookup.NCRefFilesValues[i]))
		for _, fIdx := range p.Lookup.NCRefFilesValues[i] {
			f, ok := getStr(fIdx)
			if !ok {
				return nil, false
			}
			set[f] = true
		}
		idx.nonCommentRefFilesByName[name] = set
	}

	// nonCommentRefCountByNameFile
	if len(p.Lookup.NCRefCountKeys) != len(p.Lookup.NCRefCountFiles) ||
		len(p.Lookup.NCRefCountKeys) != len(p.Lookup.NCRefCountValues) {
		return nil, false
	}
	for i, nameIdx := range p.Lookup.NCRefCountKeys {
		name, ok := getStr(nameIdx)
		if !ok {
			return nil, false
		}
		files := p.Lookup.NCRefCountFiles[i]
		counts := p.Lookup.NCRefCountValues[i]
		if len(files) != len(counts) {
			return nil, false
		}
		inner := make(map[string]int, len(files))
		for j, fIdx := range files {
			f, ok := getStr(fIdx)
			if !ok {
				return nil, false
			}
			inner[f] = int(counts[j])
		}
		idx.nonCommentRefCountByNameFile[name] = inner
	}

	// Bloom filter
	if len(p.Lookup.Bloom) == 0 {
		return nil, false
	}
	bf := &bloom.BloomFilter{}
	if err := bf.UnmarshalBinary(p.Lookup.Bloom); err != nil {
		return nil, false
	}
	idx.refBloom = bf

	return idx, true
}

// LoadCrossFileCacheIndex returns a fully-assembled CodeIndex (symbols,
// references, lookup maps, and bloom filter) when the on-disk
// fingerprint matches. A miss — missing files, version mismatch,
// decode error, fingerprint drift, or missing lookup section — is
// never an error; callers fall back to BuildIndex.
func LoadCrossFileCacheIndex(dir, wantFingerprint string) (*CodeIndex, bool) {
	if dir == "" || wantFingerprint == "" {
		crossFileMisses.Add(1)
		return nil, false
	}
	paths := crossFileCacheFiles(dir)
	metaBytes, err := os.ReadFile(paths.Meta)
	if err != nil {
		crossFileMisses.Add(1)
		return nil, false
	}
	var meta CrossFileCacheMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		crossFileMisses.Add(1)
		return nil, false
	}
	if meta.Version != CrossFileCacheVersion || meta.Fingerprint != wantFingerprint {
		crossFileMisses.Add(1)
		return nil, false
	}
	var packed cachePayload
	if err := decodeGob(paths.Symbols, &packed); err != nil {
		crossFileMisses.Add(1)
		return nil, false
	}
	idx, ok := packed.unpackFull()
	if !ok {
		crossFileMisses.Add(1)
		return nil, false
	}
	crossFileHits.Add(1)
	recordCrossFileDisk(paths, meta.SymbolCount+meta.ReferenceCount)
	if !meta.WrittenAt.IsZero() {
		crossFileLastWrite.Store(meta.WrittenAt.Unix())
	}
	return idx, true
}

// SaveCrossFileCacheIndex persists a fully-built CodeIndex so warm
// loads can skip the lookup-map rebuild.
func SaveCrossFileCacheIndex(dir, fingerprint string, meta CrossFileCacheMeta, idx *CodeIndex) error {
	if dir == "" {
		return fmt.Errorf("empty cache dir")
	}
	if idx == nil {
		return fmt.Errorf("nil index")
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
	meta.SymbolCount = len(idx.Symbols)
	meta.ReferenceCount = len(idx.References)

	packed := packPayloadWithIndex(idx)
	if err := fsutil.WriteFileAtomicStream(paths.Symbols, 0o644, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(packed)
	}); err != nil {
		return fmt.Errorf("write cross-file cache payload: %w", err)
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	if err := fsutil.WriteFileAtomic(paths.Meta, metaBytes, 0644); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}
	recordCrossFileDisk(paths, meta.SymbolCount+meta.ReferenceCount)
	crossFileLastWrite.Store(meta.WrittenAt.Unix())
	return nil
}

// LoadCrossFileCache returns (symbols, refs, true) when the on-disk
// fingerprint matches wantFingerprint. Any other outcome — missing
// files, version mismatch, decode error, fingerprint drift — is a miss.
// A miss is never an error; callers fall back to BuildIndex.
func LoadCrossFileCache(dir, wantFingerprint string) ([]Symbol, []Reference, bool) {
	if dir == "" || wantFingerprint == "" {
		crossFileMisses.Add(1)
		return nil, nil, false
	}
	paths := crossFileCacheFiles(dir)
	metaBytes, err := os.ReadFile(paths.Meta)
	if err != nil {
		crossFileMisses.Add(1)
		return nil, nil, false
	}
	var meta CrossFileCacheMeta
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		crossFileMisses.Add(1)
		return nil, nil, false
	}
	if meta.Version != CrossFileCacheVersion || meta.Fingerprint != wantFingerprint {
		crossFileMisses.Add(1)
		return nil, nil, false
	}
	var packed cachePayload
	if err := decodeGob(paths.Symbols, &packed); err != nil {
		crossFileMisses.Add(1)
		return nil, nil, false
	}
	syms, refs, ok := packed.unpack()
	if !ok {
		crossFileMisses.Add(1)
		return nil, nil, false
	}
	crossFileHits.Add(1)
	recordCrossFileDisk(paths, meta.SymbolCount+meta.ReferenceCount)
	if !meta.WrittenAt.IsZero() {
		crossFileLastWrite.Store(meta.WrittenAt.Unix())
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

	packed := packPayload(symbols, refs)
	if err := fsutil.WriteFileAtomicStream(paths.Symbols, 0o644, func(w io.Writer) error {
		return gob.NewEncoder(w).Encode(packed)
	}); err != nil {
		return fmt.Errorf("write cross-file cache symbols: %w", err)
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	if err := fsutil.WriteFileAtomic(paths.Meta, metaBytes, 0644); err != nil {
		return fmt.Errorf("write meta: %w", err)
	}
	recordCrossFileDisk(paths, meta.SymbolCount+meta.ReferenceCount)
	crossFileLastWrite.Store(meta.WrittenAt.Unix())
	return nil
}

// ClearCrossFileCache removes every file under the cache dir.
func ClearCrossFileCache(dir string) error {
	if dir == "" {
		return nil
	}
	crossFileEntries.Store(0)
	crossFileBytes.Store(0)
	return os.RemoveAll(dir)
}

func decodeGob(path string, v any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return gob.NewDecoder(f).Decode(v)
}

