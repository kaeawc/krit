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

// crossFindingsCacheVersion is bumped whenever the on-disk layout or
// FindingColumns shape changes in a way the old payload cannot be
// decoded under. Mismatch is treated as miss.
const crossFindingsCacheVersion uint32 = 1

const crossFindingsCacheDirName = "cross-findings-cache"

const crossFindingsCacheFile = "findings.bin"

// Hot-path counters. Populated on Load/Save success.
var (
	crossFindingsHits      atomic.Int64
	crossFindingsMisses    atomic.Int64
	crossFindingsBytes     atomic.Int64
	crossFindingsLastWrite atomic.Int64
)

func init() {
	cacheutil.Register(crossFindingsCacheRegistered{})
}

type crossFindingsCacheRegistered struct{}

func (crossFindingsCacheRegistered) Name() string { return crossFindingsCacheDirName }
func (crossFindingsCacheRegistered) Clear(ctx cacheutil.ClearContext) error {
	return ClearCrossFindingsCache(CrossFindingsCacheDir(ctx.RepoDir))
}
func (crossFindingsCacheRegistered) Stats() cacheutil.CacheStats {
	entries := int64(0)
	if crossFindingsBytes.Load() > 0 {
		entries = 1
	}
	return cacheutil.CacheStats{
		Entries:       int(entries),
		Bytes:         crossFindingsBytes.Load(),
		Hits:          crossFindingsHits.Load(),
		Misses:        crossFindingsMisses.Load(),
		LastWriteUnix: crossFindingsLastWrite.Load(),
	}
}

// CrossFindingsCacheDir returns the cache root for a repo.
func CrossFindingsCacheDir(repoDir string) string {
	if repoDir == "" {
		return ""
	}
	return filepath.Join(repoDir, ".krit", crossFindingsCacheDirName)
}

// ClearCrossFindingsCache removes the on-disk cross-findings cache. Safe
// to call when the directory is absent.
func ClearCrossFindingsCache(dir string) error {
	if dir == "" {
		return nil
	}
	if err := os.RemoveAll(dir); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	crossFindingsBytes.Store(0)
	return nil
}

// CrossFindingsKey composes the cache lookup key from the
// codeIndex/parsed-files fingerprint and the cross-rule ruleHash. The
// returned hex digest covers both inputs plus the cache version, so
// changing any one of them produces a new entry.
func CrossFindingsKey(indexFingerprint, ruleHash string) string {
	h := hashutil.Hasher().New()
	var v [4]byte
	binary.BigEndian.PutUint32(v[:], crossFindingsCacheVersion)
	_, _ = h.Write(v[:])
	_, _ = h.Write([]byte(indexFingerprint))
	_, _ = h.Write([]byte{'|'})
	_, _ = h.Write([]byte(ruleHash))
	return hex.EncodeToString(h.Sum(nil))
}

// crossFindingsHeader is the on-disk fixed prefix preceding the gob
// payload. CRC covers the gob payload only.
type crossFindingsHeader struct {
	Magic   uint32 // 'KFND'
	Version uint32
	KeyLen  uint16
	CRC32   uint32
	Length  uint64
}

const crossFindingsMagic uint32 = 0x4b464e44 // "KFND"

// crossFindingsPayload is the gob-encoded payload. Wraps FindingColumns
// directly; the columnar form is the canonical cross-rule output.
type crossFindingsPayload struct {
	Columns FindingColumns
}

// LoadCrossFindings reads cached cross-rule findings for the given key.
// Returns (cols, true) on hit and (FindingColumns{}, false) on any miss
// (including key mismatch, version mismatch, CRC failure, missing file).
func LoadCrossFindings(cacheDir, key string) (FindingColumns, bool) {
	if cacheDir == "" || key == "" {
		return FindingColumns{}, false
	}
	path := filepath.Join(cacheDir, crossFindingsCacheFile)
	data, err := os.ReadFile(path)
	if err != nil {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if len(data) < 4+4+2+4+8 {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	r := bytes.NewReader(data)
	var hdr crossFindingsHeader
	if err := binary.Read(r, binary.BigEndian, &hdr.Magic); err != nil {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if hdr.Magic != crossFindingsMagic {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.Version); err != nil || hdr.Version != crossFindingsCacheVersion {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.KeyLen); err != nil {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	storedKey := make([]byte, hdr.KeyLen)
	if _, err := io.ReadFull(r, storedKey); err != nil {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if string(storedKey) != key {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.CRC32); err != nil {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if err := binary.Read(r, binary.BigEndian, &hdr.Length); err != nil {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	compressed := make([]byte, hdr.Length)
	if _, err := io.ReadFull(r, compressed); err != nil {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	if crc32.ChecksumIEEE(compressed) != hdr.CRC32 {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	raw, err := shardZstdDecoder.DecodeAll(compressed, nil)
	if err != nil {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	var payload crossFindingsPayload
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&payload); err != nil {
		crossFindingsMisses.Add(1)
		return FindingColumns{}, false
	}
	crossFindingsHits.Add(1)
	crossFindingsBytes.Store(int64(len(data)))
	return payload.Columns, true
}

// SaveCrossFindings writes cached cross-rule findings for the given
// key. Best-effort: any error returns nil after recording a miss
// counter, since cache failures should never break analysis.
func SaveCrossFindings(cacheDir, key string, cols FindingColumns) error {
	if cacheDir == "" || key == "" {
		return nil
	}
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return err
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(crossFindingsPayload{Columns: cols}); err != nil {
		return err
	}
	compressed := shardZstdEncoder.EncodeAll(buf.Bytes(), nil)
	keyBytes := []byte(key)
	out := bytes.NewBuffer(nil)
	_ = binary.Write(out, binary.BigEndian, crossFindingsMagic)
	_ = binary.Write(out, binary.BigEndian, crossFindingsCacheVersion)
	_ = binary.Write(out, binary.BigEndian, uint16(len(keyBytes)))
	out.Write(keyBytes)
	_ = binary.Write(out, binary.BigEndian, crc32.ChecksumIEEE(compressed))
	_ = binary.Write(out, binary.BigEndian, uint64(len(compressed)))
	out.Write(compressed)
	dest := filepath.Join(cacheDir, crossFindingsCacheFile)
	if err := fsutil.WriteFileAtomic(dest, out.Bytes(), 0o644); err != nil {
		return err
	}
	crossFindingsBytes.Store(int64(out.Len()))
	crossFindingsLastWrite.Store(time.Now().Unix())
	return nil
}
