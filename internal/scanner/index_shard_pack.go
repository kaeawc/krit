package scanner

// Pack-file storage for cross-file shards. Replaces the
// "one .gob per shard" layout with 256 pack files keyed by the first
// byte of the shard hash. Each pack holds a header table of
// (key, offset, length, crc32) followed by concatenated gob-encoded
// fileShard blobs.
//
// Why 256 packs: uniform bucket distribution (keys are hex-encoded
// xxh3 hashes) means each pack holds ~N/256 shards, so a write
// touches a bounded fraction of the dataset. On the Signal-Android
// 5,500-shard corpus, that's ~21 shards / ~3 MB per pack.
//
// Rewrite semantics: replacing one shard rewrites its entire pack
// atomically (write .tmp, rename). Benchmarks measured ~850 µs on
// the Signal dataset - acceptable for the incremental-scan path.
//
// Corruption handling: each blob carries a CRC-32. A mismatch is
// treated as a per-key miss rather than a whole-pack failure, so a
// single bit-flip invalidates one shard not 21.
//
// Concurrency: per-pack RWMutex. Reads take RLock; writes and the
// first-access load take Lock. The scanner creates one packStore per
// scan, so the store is garbage-collected after the scan finishes
// (no long-lived process-wide heap retention).
//
// Versioning: the pack magic/version header is independent of
// CrossFileCacheVersion. A version mismatch during ensureLoaded
// returns ErrPackVersion, which the caller treats as "every shard
// in this pack missed" - the same behavior as an empty pack.

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/klauspost/compress/zstd"
)

// Per-process zstd codec for shard blobs. Both encoder and decoder are
// safe for concurrent EncodeAll / DecodeAll. Level=SpeedDefault (~3)
// hits the documented ~5× ratio at ~1 GB/s decompress on M-class
// silicon, well below disk-read amortisation.
var (
	shardZstdEncoder *zstd.Encoder
	shardZstdDecoder *zstd.Decoder
)

func init() {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		panic(fmt.Sprintf("init shard zstd encoder: %v", err))
	}
	shardZstdEncoder = enc
	dec, err := zstd.NewReader(nil)
	if err != nil {
		panic(fmt.Sprintf("init shard zstd decoder: %v", err))
	}
	shardZstdDecoder = dec
}

const (
	packMagic    uint32 = 0x4b50414b // "KPAK"
	packVersion  uint16 = 1
	packSubdir          = "packs-v1"
	packBucketBits      = 256
	packHeaderFixed     = 4 + 2 + 4 // magic + version + entryCount
	packEntryFixed      = 2 + 8 + 8 + 4 // keyLen + offset + length + crc32
	packExt             = ".pack"
)

// errPackCorrupt is returned when a pack header fails to parse. The
// caller treats this as "pack missing" and lets the scanner rebuild.
var errPackCorrupt = errors.New("pack corrupt")

// packEntry locates one blob within a pack file. offset/length are in
// bytes from the start of the file; crc is crc32.IEEE of the blob.
type packEntry struct {
	offset uint64
	length uint64
	crc    uint32
}

// packHandle wraps one pack file. Lazy-loaded on first access so an
// LSP scan that only touches a few files doesn't eagerly read all
// 256 packs.
type packHandle struct {
	path   string
	mu     sync.RWMutex
	loaded bool
	missing bool // file did not exist on last load attempt
	index  map[string]packEntry
	data   []byte // full pack contents, indexed by entry.offset
}

// legacyShardsSubdir is the directory name used by the pre-v3 layout
// (one .gob per shard). newPackStore removes it lazily so upgraders
// don't carry ~750 MB of dead data forever.
const legacyShardsSubdir = "shards"

// packStore owns the 256 packs under {cacheDir}/{packSubdir}/. One
// instance per scan.
type packStore struct {
	dir         string
	cacheDir    string
	legacySweep sync.Once
	packs       [packBucketBits]*packHandle
}

// newPackStore returns a store rooted at {cacheDir}/{packSubdir}/.
// The directory is created on first write, not eagerly - a cacheDir
// that's never written to leaves no trace.
func newPackStore(cacheDir string) *packStore {
	if cacheDir == "" {
		return nil
	}
	ps := &packStore{
		dir:      filepath.Join(cacheDir, packSubdir),
		cacheDir: cacheDir,
	}
	for i := 0; i < packBucketBits; i++ {
		ps.packs[i] = &packHandle{
			path: filepath.Join(ps.dir, fmt.Sprintf("%02x%s", i, packExt)),
		}
	}
	ps.sweepLegacyShardsDir()
	return ps
}

// packFor returns the handle owning key. Panics on a key shorter than
// two hex chars - shardKey() always produces a 32-char hex digest so
// this is a programmer error, not a runtime condition.
func (ps *packStore) packFor(key string) *packHandle {
	if len(key) < 2 {
		panic("shard key too short: " + key)
	}
	var hi byte
	if _, err := fmt.Sscanf(key[:2], "%02x", &hi); err != nil {
		panic(fmt.Sprintf("shard key not hex: %s", key))
	}
	return ps.packs[hi]
}

// ensureLoaded reads the pack from disk and parses the header. A
// missing file is not an error - it's just an empty pack.
// Checksum failures on individual blobs are deferred to the caller
// (the read path verifies on Get); header corruption returns errPackCorrupt.
func (h *packHandle) ensureLoaded() error {
	h.mu.RLock()
	if h.loaded {
		h.mu.RUnlock()
		return nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.loaded {
		return nil
	}

	data, err := os.ReadFile(h.path)
	if err != nil {
		if os.IsNotExist(err) {
			h.loaded = true
			h.missing = true
			h.index = nil
			return nil
		}
		return err
	}
	idx, err := parsePackHeader(data)
	if err != nil {
		return err
	}
	h.data = data
	h.index = idx
	h.loaded = true
	h.missing = false
	return nil
}

// parsePackHeader validates the magic/version and returns the entry
// index. The blob region is verified per-blob at Get time, so corrupt
// blobs invalidate only their own key.
func parsePackHeader(data []byte) (map[string]packEntry, error) {
	if len(data) < packHeaderFixed {
		return nil, errPackCorrupt
	}
	if binary.LittleEndian.Uint32(data[0:4]) != packMagic {
		return nil, errPackCorrupt
	}
	if binary.LittleEndian.Uint16(data[4:6]) != packVersion {
		return nil, errPackCorrupt
	}
	n := binary.LittleEndian.Uint32(data[6:10])
	idx := make(map[string]packEntry, n)
	pos := int64(packHeaderFixed)
	for i := uint32(0); i < n; i++ {
		if int64(len(data)) < pos+2 {
			return nil, errPackCorrupt
		}
		keyLen := binary.LittleEndian.Uint16(data[pos:])
		pos += 2
		if int64(len(data)) < pos+int64(keyLen)+int64(packEntryFixed-2) {
			return nil, errPackCorrupt
		}
		key := string(data[pos : pos+int64(keyLen)])
		pos += int64(keyLen)
		off := binary.LittleEndian.Uint64(data[pos:])
		pos += 8
		ln := binary.LittleEndian.Uint64(data[pos:])
		pos += 8
		crc := binary.LittleEndian.Uint32(data[pos:])
		pos += 4
		if off+ln > uint64(len(data)) {
			return nil, errPackCorrupt
		}
		idx[key] = packEntry{offset: off, length: ln, crc: crc}
	}
	return idx, nil
}

// get returns the raw blob bytes for key, verifying the CRC. Returns
// (nil, false) on miss, corrupt blob, or unloaded handle. The byte
// slice aliases the pack's mmap-like in-memory buffer; the caller
// must copy before the next write to this pack.
func (h *packHandle) get(key string) ([]byte, bool) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if !h.loaded || h.missing || h.index == nil {
		return nil, false
	}
	e, ok := h.index[key]
	if !ok {
		return nil, false
	}
	blob := h.data[e.offset : e.offset+e.length]
	if crc32.ChecksumIEEE(blob) != e.crc {
		return nil, false
	}
	return blob, true
}

// put atomically rewrites the pack with (key, blob) inserted or
// replaced. All other entries are preserved. Holds the pack write
// lock for the duration.
func (h *packHandle) put(key string, blob []byte) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	// ensure loaded without re-locking
	if !h.loaded {
		data, err := os.ReadFile(h.path)
		switch {
		case err == nil:
			idx, perr := parsePackHeader(data)
			if perr != nil {
				// Corrupt pack - start fresh. Losing ≤21 shards is
				// cheaper than losing the run.
				h.data = nil
				h.index = map[string]packEntry{}
			} else {
				h.data = data
				h.index = idx
			}
		case os.IsNotExist(err):
			h.index = map[string]packEntry{}
		default:
			return err
		}
		h.loaded = true
		h.missing = false
	}
	if h.index == nil {
		h.index = map[string]packEntry{}
	}

	// Reconstruct blob list: existing entries (excluding key) + new.
	type kv struct {
		key  string
		data []byte
		crc  uint32
	}
	items := make([]kv, 0, len(h.index)+1)
	for k, e := range h.index {
		if k == key {
			continue
		}
		// Copy existing blob - the final buffer replaces h.data.
		cp := make([]byte, e.length)
		copy(cp, h.data[e.offset:e.offset+e.length])
		items = append(items, kv{key: k, data: cp, crc: e.crc})
	}
	items = append(items, kv{key: key, data: blob, crc: crc32.ChecksumIEEE(blob)})

	// Compute header size and write new pack.
	headerSize := int64(packHeaderFixed)
	for _, it := range items {
		headerSize += int64(packEntryFixed) + int64(len(it.key))
	}
	totalSize := headerSize
	for _, it := range items {
		totalSize += int64(len(it.data))
	}
	buf := make([]byte, totalSize)
	binary.LittleEndian.PutUint32(buf[0:4], packMagic)
	binary.LittleEndian.PutUint16(buf[4:6], packVersion)
	binary.LittleEndian.PutUint32(buf[6:10], uint32(len(items)))
	pos := int64(packHeaderFixed)
	dataOff := headerSize
	newIndex := make(map[string]packEntry, len(items))
	for _, it := range items {
		binary.LittleEndian.PutUint16(buf[pos:], uint16(len(it.key)))
		pos += 2
		copy(buf[pos:], it.key)
		pos += int64(len(it.key))
		binary.LittleEndian.PutUint64(buf[pos:], uint64(dataOff))
		pos += 8
		binary.LittleEndian.PutUint64(buf[pos:], uint64(len(it.data)))
		pos += 8
		binary.LittleEndian.PutUint32(buf[pos:], it.crc)
		pos += 4
		newIndex[it.key] = packEntry{
			offset: uint64(dataOff),
			length: uint64(len(it.data)),
			crc:    it.crc,
		}
		dataOff += int64(len(it.data))
	}
	// Append blobs.
	blobOff := headerSize
	for _, it := range items {
		copy(buf[blobOff:], it.data)
		blobOff += int64(len(it.data))
	}

	if err := os.MkdirAll(filepath.Dir(h.path), 0o755); err != nil {
		return err
	}
	tmp := h.path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(buf); err != nil {
		_ = f.Close()
		_ = os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, h.path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	h.data = buf
	h.index = newIndex
	return nil
}

// LoadShard returns a decoded shard matching (path, contentHash), or
// (nil, false) on any miss. Matches the semantics of loadFileShard so
// callers can swap implementations.
func (ps *packStore) LoadShard(path, contentHash string) (*fileShard, bool) {
	if ps == nil {
		shardsMisses.Add(1)
		return nil, false
	}
	key := shardKey(path, contentHash)
	h := ps.packFor(key)
	if err := h.ensureLoaded(); err != nil {
		shardsMisses.Add(1)
		return nil, false
	}
	blob, ok := h.get(key)
	if !ok {
		shardsMisses.Add(1)
		return nil, false
	}
	s, err := decodeShardBlob(blob, path)
	if err != nil {
		shardsMisses.Add(1)
		return nil, false
	}
	s.ContentHash = contentHash
	observeShard(key, int64(len(blob)))
	shardsHits.Add(1)
	return s, true
}

// SaveShard persists s. A nil store or empty cacheDir is a silent
// no-op to match the fs-backend error shape.
func (ps *packStore) SaveShard(s *fileShard) error {
	if ps == nil {
		return nil
	}
	if s == nil {
		return fmt.Errorf("nil shard")
	}
	s.Version = crossFileShardVersion
	key := shardKey(s.Path, s.ContentHash)
	blob, err := encodeShardBlob(s)
	if err != nil {
		return err
	}
	h := ps.packFor(key)
	if err := h.put(key, blob); err != nil {
		return err
	}
	shardsLastWrite.Store(time.Now().Unix())
	observeShard(key, int64(len(blob)))
	return nil
}

// sweepLegacyShardsDir removes the pre-v3 shards/ directory once per
// packStore lifetime. Best-effort: a failed removal is a silent no-op
// (disk-full, permission, etc.) so the scan continues either way.
// Running once per process is enough; subsequent scans will see the
// dir gone.
func (ps *packStore) sweepLegacyShardsDir() {
	if ps == nil || ps.cacheDir == "" {
		return
	}
	ps.legacySweep.Do(func() {
		legacy := filepath.Join(ps.cacheDir, legacyShardsSubdir)
		_ = os.RemoveAll(legacy)
	})
}

// encodeShardBlob / decodeShardBlob centralize the columnar+zstd
// envelope so the pack layer is orthogonal to the shard payload shape.
//
// Wire format (v6): zstd(shardPayload). shardPayload is the framed
// columnar "KSHC" blob defined in index_shard_codec.go. Symbol.File /
// Reference.File / fileShard.Path / fileShard.ContentHash are not
// serialised — the pack key + LoadShard args already identify the
// shard's (path, contentHash), and all rows in a shard share that path.
func encodeShardBlob(s *fileShard) ([]byte, error) {
	return shardZstdEncoder.EncodeAll(encodeShardPayload(s), nil), nil
}

func decodeShardBlob(blob []byte, path string) (*fileShard, error) {
	raw, err := shardZstdDecoder.DecodeAll(blob, nil)
	if err != nil {
		return nil, err
	}
	return decodeShardPayload(raw, path)
}
