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
	"sync/atomic"
	"time"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/fsutil"
	"github.com/kaeawc/krit/internal/hashutil"
)

// androidFindingsCacheVersion is bumped whenever the on-disk layout or
// FindingColumns shape changes in a way the old payload cannot be decoded
// under. Mismatch is treated as a miss.
const androidFindingsCacheVersion uint32 = 1

const androidFindingsCacheDirName = "android-findings-cache"

// AndroidFindingsKind tags the input family inside an AndroidFindings cache
// key so two different families with coincidentally equal content
// fingerprints can never share a cache entry.
type AndroidFindingsKind string

const (
	AndroidFindingsKindManifest             AndroidFindingsKind = "manifest"
	AndroidFindingsKindGradle               AndroidFindingsKind = "gradle"
	AndroidFindingsKindResources            AndroidFindingsKind = "resources"
	AndroidFindingsKindIcons                AndroidFindingsKind = "icons"
	AndroidFindingsKindManifestBundle       AndroidFindingsKind = "manifest-bundle"
	AndroidFindingsKindResourceBundle       AndroidFindingsKind = "resource-bundle"
	AndroidFindingsKindGradleBundle         AndroidFindingsKind = "gradle-bundle"
	AndroidFindingsKindIconBundle           AndroidFindingsKind = "icon-bundle"
	AndroidFindingsKindResourceSource       AndroidFindingsKind = "resource-source"
	AndroidFindingsKindResourceSourceBundle AndroidFindingsKind = "resource-source-bundle"
	AndroidFindingsKindProject              AndroidFindingsKind = "project"
)

var (
	androidFindingsHits      atomic.Int64
	androidFindingsMisses    atomic.Int64
	androidFindingsBytes     atomic.Int64
	androidFindingsLastWrite atomic.Int64
)

func init() {
	cacheutil.Register(androidFindingsCacheRegistered{})
}

type androidFindingsCacheRegistered struct{}

func (androidFindingsCacheRegistered) Name() string { return androidFindingsCacheDirName }
func (androidFindingsCacheRegistered) Clear(ctx cacheutil.ClearContext) error {
	return ClearAndroidFindingsCache(AndroidFindingsCacheDir(ctx.RepoDir))
}
func (androidFindingsCacheRegistered) Stats() cacheutil.CacheStats {
	return cacheutil.CacheStats{
		Bytes:         androidFindingsBytes.Load(),
		Hits:          androidFindingsHits.Load(),
		Misses:        androidFindingsMisses.Load(),
		LastWriteUnix: androidFindingsLastWrite.Load(),
	}
}

// AndroidFindingsCacheDir returns the cache root for a repo.
func AndroidFindingsCacheDir(repoDir string) string {
	if repoDir == "" {
		return ""
	}
	return filepath.Join(repoDir, ".krit", androidFindingsCacheDirName)
}

// ClearAndroidFindingsCache removes the on-disk cache. Safe to call when
// absent.
func ClearAndroidFindingsCache(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	androidFindingsBytes.Store(0)
	return nil
}

// AndroidFindingsKeyInputs are the components mixed into a cache lookup
// key. Every field that influences findings for the cached unit MUST be
// represented here — a missing input is a false-hit waiting to happen.
type AndroidFindingsKeyInputs struct {
	// Kind is the input family; manifests, gradle files, resource dirs,
	// icons, and per-source resource lookups never share entries.
	Kind AndroidFindingsKind
	// RuleHash covers active rule IDs and their config (from
	// cache.ComputeConfigHash).
	RuleHash string
	// LibraryFactsFP is librarymodel.Facts.Fingerprint().
	LibraryFactsFP string
	// JavaSemanticFactsFP is javafacts.Facts.Fingerprint().
	JavaSemanticFactsFP string
	// InputFP is the kind-specific input fingerprint: a manifest's
	// content hash, a Gradle file's content hash, a merged resource-index
	// hash, etc. The discipline is content-based, never mtime.
	InputFP string
	// Extra is a kind-specific supplementary fingerprint mixed in after
	// InputFP. Use it for context that isn't captured by the named
	// fingerprints above (e.g. concatenated manifest-merge ancestors,
	// merged ResourceIndex hash for resource-source rules). Empty when
	// the kind's findings depend on no extra context.
	Extra string
}

// AndroidFindingsKey composes the inputs into a stable hex digest. Order
// is fixed; field separators are NUL bytes that cannot appear inside any
// of the hex/string fields.
func AndroidFindingsKey(in AndroidFindingsKeyInputs) string {
	h := hashutil.Hasher().New()
	var v [4]byte
	binary.BigEndian.PutUint32(v[:], androidFindingsCacheVersion)
	_, _ = h.Write(v[:])
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(in.Kind))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(in.RuleHash))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(in.LibraryFactsFP))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(in.JavaSemanticFactsFP))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(in.InputFP))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(in.Extra))
	return hex.EncodeToString(h.Sum(nil))
}

// androidFindingsHeader is the on-disk fixed prefix preceding the gob
// payload. CRC covers the (zstd-compressed) gob payload only.
type androidFindingsHeader struct {
	Magic   uint32 // 'KAFD'
	Version uint32
	KeyLen  uint16
	CRC32   uint32
	Length  uint64
}

const androidFindingsMagic uint32 = 0x4b414644 // "KAFD"

type androidFindingsPayload struct {
	Columns FindingColumns
}

func androidEntryPath(cacheDir, key string) string {
	if len(key) < 2 {
		return filepath.Join(cacheDir, "entries", key+".bin")
	}
	return filepath.Join(cacheDir, "entries", key[:2], key[2:]+".bin")
}

// LoadAndroidFindings reads cached findings for the given key. Returns
// (cols, true) on hit and (FindingColumns{}, false) on any miss (missing
// file, version/CRC mismatch, key collision, decode error).
func LoadAndroidFindings(cacheDir, key string) (FindingColumns, bool) {
	if cacheDir == "" || key == "" {
		return FindingColumns{}, false
	}
	data, err := os.ReadFile(androidEntryPath(cacheDir, key))
	if err != nil {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if len(data) < 4+4+2+4+8 {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	r := bytes.NewReader(data)
	var hdr androidFindingsHeader
	if err := binary.Read(r, binary.BigEndian, &hdr.Magic); err != nil || hdr.Magic != androidFindingsMagic {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.Version); err != nil || hdr.Version != androidFindingsCacheVersion {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.KeyLen); err != nil {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	storedKey := make([]byte, hdr.KeyLen)
	if _, err := io.ReadFull(r, storedKey); err != nil || string(storedKey) != key {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.CRC32); err != nil {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.Length); err != nil {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	compressed := make([]byte, hdr.Length)
	if _, err := io.ReadFull(r, compressed); err != nil {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if crc32.ChecksumIEEE(compressed) != hdr.CRC32 {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	raw, err := shardZstdDecoder.DecodeAll(compressed, nil)
	if err != nil {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	var payload androidFindingsPayload
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&payload); err != nil {
		androidFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	androidFindingsHits.Add(1)
	androidFindingsBytes.Add(int64(len(data)))
	return payload.Columns, true
}

// SaveAndroidFindings writes cached findings for the given key. Best
// effort: any error is returned but the analysis path treats failure as
// non-fatal.
func SaveAndroidFindings(cacheDir, key string, cols FindingColumns) error {
	if cacheDir == "" || key == "" {
		return nil
	}
	entryPath := androidEntryPath(cacheDir, key)
	if err := os.MkdirAll(filepath.Dir(entryPath), 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(androidFindingsPayload{Columns: cols}); err != nil {
		return err
	}
	compressed := shardZstdEncoder.EncodeAll(buf.Bytes(), nil)
	keyBytes := []byte(key)
	out := bytes.NewBuffer(nil)
	_ = binary.Write(out, binary.BigEndian, androidFindingsMagic)
	_ = binary.Write(out, binary.BigEndian, androidFindingsCacheVersion)
	_ = binary.Write(out, binary.BigEndian, uint16(len(keyBytes)))
	out.Write(keyBytes)
	_ = binary.Write(out, binary.BigEndian, crc32.ChecksumIEEE(compressed))
	_ = binary.Write(out, binary.BigEndian, uint64(len(compressed)))
	out.Write(compressed)
	if err := fsutil.WriteFileAtomic(entryPath, out.Bytes(), 0o644); err != nil {
		return err
	}
	androidFindingsBytes.Add(int64(out.Len()))
	androidFindingsLastWrite.Store(time.Now().Unix())
	return nil
}
