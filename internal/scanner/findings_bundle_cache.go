package scanner

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"errors"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

const (
	findingsBundleCacheDirName        = "findings-bundle-cache"
	findingsBundleVersion             = 1
	findingsBundleMagic        uint32 = 0x4b465542 // "KFUB"
)

var (
	findingsBundleHits      atomic.Int64
	findingsBundleMisses    atomic.Int64
	findingsBundleBytes     atomic.Int64
	findingsBundleLastWrite atomic.Int64
)

func init() {
	cacheutil.Register(findingsBundleCacheRegistered{})
}

type findingsBundleCacheRegistered struct{}

func (findingsBundleCacheRegistered) Name() string { return findingsBundleCacheDirName }
func (findingsBundleCacheRegistered) Clear(ctx cacheutil.ClearContext) error {
	return ClearFindingsBundleCache(FindingsBundleCacheDir(ctx.RepoDir))
}
func (findingsBundleCacheRegistered) Stats() cacheutil.CacheStats {
	return cacheutil.CacheStats{
		Bytes:         findingsBundleBytes.Load(),
		Hits:          findingsBundleHits.Load(),
		Misses:        findingsBundleMisses.Load(),
		LastWriteUnix: findingsBundleLastWrite.Load(),
	}
}

type RunFingerprint struct {
	Version      string
	Rules        string
	Config       string
	SourceSet    string
	CrossFile    string
	Android      string
	LibraryFacts string
}

type FindingsBundleStore interface {
	Load(root string, fp RunFingerprint) (*FindingColumns, bool)
	Save(root string, fp RunFingerprint, cols *FindingColumns) error
}

type DeltaPlan struct {
	ReusePrevious bool
	ChangedPaths  []string
	AffectedPaths []string
}

type DeltaPlanner interface {
	Plan(previous, current RunFingerprint, changed []string) DeltaPlan
}

type ConservativeDeltaPlanner struct{}

func (ConservativeDeltaPlanner) Plan(previous, current RunFingerprint, changed []string) DeltaPlan {
	changed = cleanSortedPaths(changed)
	if len(changed) == 0 || len(changed) > 1 {
		return DeltaPlan{ChangedPaths: changed, AffectedPaths: changed}
	}
	stable := previous.Version == current.Version &&
		previous.Rules == current.Rules &&
		previous.Config == current.Config &&
		previous.CrossFile == current.CrossFile &&
		previous.Android == current.Android &&
		previous.LibraryFacts == current.LibraryFacts
	if !stable {
		return DeltaPlan{ChangedPaths: changed, AffectedPaths: changed}
	}
	return DeltaPlan{ReusePrevious: true, ChangedPaths: changed, AffectedPaths: changed}
}

func ApplyDelta(previous *FindingColumns, replacement *FindingColumns, affected []string) FindingColumns {
	if previous == nil {
		if replacement == nil {
			return FindingColumns{}
		}
		return replacement.Clone()
	}
	affectedSet := make(map[string]bool, len(affected))
	for _, path := range affected {
		if path != "" {
			affectedSet[path] = true
		}
	}
	base := previous.FilterRows(func(row int) bool {
		return !affectedSet[previous.FileAt(row)]
	})
	collector := NewFindingCollector(base.Len())
	collector.AppendColumns(&base)
	if replacement != nil {
		collector.AppendColumns(replacement)
	}
	out := *collector.Columns()
	out.SortByFileLine()
	return out
}

type DiskFindingsBundleStore struct{}

type findingsBundleHeader struct {
	Magic   uint32
	Version uint32
	KeyLen  uint16
	CRC32   uint32
	Length  uint64
}

type findingsBundlePayload struct {
	Columns FindingColumns
}

func FindingsBundleCacheDir(repoDir string) string {
	if repoDir == "" {
		return ""
	}
	return filepath.Join(repoDir, ".krit", findingsBundleCacheDirName)
}

func ClearFindingsBundleCache(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	findingsBundleBytes.Store(0)
	return nil
}

// FileStructuralFingerprint hashes a single file's contribution to the
// cross-file CodeIndex: its declared Symbols (sorted by FQN + Name +
// Kind + Visibility + Signature) and its References (sorted by Name +
// InComment flag). Per-line positions are deliberately excluded —
// line-number drift from intra-file body edits should not invalidate
// cross-file findings for unchanged files.
//
// Mismatches are precise: any added/removed/renamed declaration moves
// the fingerprint, any added/removed reference moves it, but a body
// edit that touches no Symbols and no References (whitespace, comment,
// constant value, local variable rename) keeps it stable. The
// ConservativeDeltaPlanner's "CrossFile stable" gate uses the
// aggregate of per-file structural fps to decide whether the delta
// path is safe — see pipeline.crossFileStructuralFingerprint.
func FileStructuralFingerprint(file *File) string {
	if file == nil {
		return ""
	}
	symbols, references := indexFileForFingerprint(file)
	return hashFileStructural(symbols, references)
}

func indexFileForFingerprint(file *File) ([]Symbol, []Reference) {
	if file == nil || file.FlatTree == nil {
		return nil, nil
	}
	var symbols []Symbol
	var references []Reference
	if file.Language == LangJava {
		collectJavaDeclarationsFlat(file, &symbols)
		collectJavaReferencesFlat(file, &references)
	} else {
		collectDeclarationsFlat(file, &symbols)
		collectReferencesFlat(file, &references)
	}
	return symbols, references
}

func hashFileStructural(symbols []Symbol, references []Reference) string {
	type symKey struct {
		fqn, name, kind, vis, sig string
	}
	symKeys := make([]symKey, 0, len(symbols))
	for _, s := range symbols {
		symKeys = append(symKeys, symKey{fqn: s.FQN, name: s.Name, kind: s.Kind, vis: s.Visibility, sig: s.Signature})
	}
	sort.Slice(symKeys, func(i, j int) bool {
		a, b := symKeys[i], symKeys[j]
		if a.fqn != b.fqn {
			return a.fqn < b.fqn
		}
		if a.name != b.name {
			return a.name < b.name
		}
		if a.kind != b.kind {
			return a.kind < b.kind
		}
		if a.vis != b.vis {
			return a.vis < b.vis
		}
		return a.sig < b.sig
	})

	type refKey struct {
		name      string
		inComment bool
	}
	refKeys := make([]refKey, 0, len(references))
	for _, r := range references {
		refKeys = append(refKeys, refKey{name: r.Name, inComment: r.InComment})
	}
	sort.Slice(refKeys, func(i, j int) bool {
		a, b := refKeys[i], refKeys[j]
		if a.name != b.name {
			return a.name < b.name
		}
		return !a.inComment && b.inComment
	})

	h := hashutil.Hasher().New()
	for _, k := range symKeys {
		_, _ = h.Write([]byte(k.fqn))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(k.name))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(k.kind))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(k.vis))
		_, _ = h.Write([]byte{0})
		_, _ = h.Write([]byte(k.sig))
		_, _ = h.Write([]byte{1})
	}
	_, _ = h.Write([]byte{2})
	for _, k := range refKeys {
		_, _ = h.Write([]byte(k.name))
		if k.inComment {
			_, _ = h.Write([]byte{'c'})
		}
		_, _ = h.Write([]byte{3})
	}
	return hex.EncodeToString(h.Sum(nil))
}

func FindingsBundleKey(fp RunFingerprint) string {
	h := hashutil.Hasher().New()
	var v [4]byte
	binary.BigEndian.PutUint32(v[:], findingsBundleVersion)
	_, _ = h.Write(v[:])
	writeFingerprintField(h, fp.Version)
	writeFingerprintField(h, fp.Rules)
	writeFingerprintField(h, fp.Config)
	writeFingerprintField(h, fp.SourceSet)
	writeFingerprintField(h, fp.CrossFile)
	writeFingerprintField(h, fp.Android)
	writeFingerprintField(h, fp.LibraryFacts)
	return hex.EncodeToString(h.Sum(nil))
}

func (DiskFindingsBundleStore) Load(root string, fp RunFingerprint) (*FindingColumns, bool) {
	key := FindingsBundleKey(fp)
	if root == "" || key == "" {
		return nil, false
	}
	data, err := os.ReadFile(findingsBundleEntryPath(root, key))
	if err != nil {
		findingsBundleMisses.Add(1)
		return nil, false
	}
	r := bytes.NewReader(data)
	var hdr findingsBundleHeader
	if err := binary.Read(r, binary.BigEndian, &hdr.Magic); err != nil || hdr.Magic != findingsBundleMagic {
		findingsBundleMisses.Add(1)
		return nil, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.Version); err != nil || hdr.Version != findingsBundleVersion {
		findingsBundleMisses.Add(1)
		return nil, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.KeyLen); err != nil {
		findingsBundleMisses.Add(1)
		return nil, false
	}
	storedKey := make([]byte, hdr.KeyLen)
	if _, err := io.ReadFull(r, storedKey); err != nil || string(storedKey) != key {
		findingsBundleMisses.Add(1)
		return nil, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.CRC32); err != nil {
		findingsBundleMisses.Add(1)
		return nil, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.Length); err != nil {
		findingsBundleMisses.Add(1)
		return nil, false
	}
	compressed := make([]byte, hdr.Length)
	if _, err := io.ReadFull(r, compressed); err != nil || crc32.ChecksumIEEE(compressed) != hdr.CRC32 {
		findingsBundleMisses.Add(1)
		return nil, false
	}
	raw, err := shardZstdDecoder.DecodeAll(compressed, nil)
	if err != nil {
		findingsBundleMisses.Add(1)
		return nil, false
	}
	var payload findingsBundlePayload
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&payload); err != nil {
		findingsBundleMisses.Add(1)
		return nil, false
	}
	findingsBundleHits.Add(1)
	findingsBundleBytes.Add(int64(len(data)))
	return &payload.Columns, true
}

func (DiskFindingsBundleStore) Save(root string, fp RunFingerprint, cols *FindingColumns) error {
	if root == "" || cols == nil {
		return nil
	}
	key := FindingsBundleKey(fp)
	path := findingsBundleEntryPath(root, key)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(findingsBundlePayload{Columns: *cols}); err != nil {
		return err
	}
	compressed := shardZstdEncoder.EncodeAll(buf.Bytes(), nil)
	keyBytes := []byte(key)
	out := bytes.NewBuffer(nil)
	_ = binary.Write(out, binary.BigEndian, findingsBundleMagic)
	_ = binary.Write(out, binary.BigEndian, uint32(findingsBundleVersion))
	_ = binary.Write(out, binary.BigEndian, uint16(len(keyBytes)))
	out.Write(keyBytes)
	_ = binary.Write(out, binary.BigEndian, crc32.ChecksumIEEE(compressed))
	_ = binary.Write(out, binary.BigEndian, uint64(len(compressed)))
	out.Write(compressed)
	if err := fsutil.WriteFileAtomic(path, out.Bytes(), 0o644); err != nil {
		return err
	}
	findingsBundleBytes.Add(int64(out.Len()))
	findingsBundleLastWrite.Store(time.Now().Unix())
	return nil
}

func findingsBundleEntryPath(root, key string) string {
	if len(key) >= 2 {
		return filepath.Join(FindingsBundleCacheDir(root), "entries", key[:2], key[2:]+".bin")
	}
	return filepath.Join(FindingsBundleCacheDir(root), "entries", key+".bin")
}

func writeFingerprintField(h interface{ Write([]byte) (int, error) }, value string) {
	_, _ = h.Write([]byte(value))
	_, _ = h.Write([]byte{0})
}

func cleanSortedPaths(paths []string) []string {
	seen := make(map[string]bool, len(paths))
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, path)
	}
	sort.Strings(out)
	return out
}
