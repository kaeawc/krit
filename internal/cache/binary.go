package cache

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"fmt"
	"hash/crc32"
	"sort"

	"github.com/klauspost/compress/zstd"

	"github.com/kaeawc/krit/internal/scanner"
)

// Krit File Incremental Cache: a magic-prefixed, zstd-compressed gob frame
// that replaces the legacy JSON encoding. The legacy JSON path is still used
// as a fallback in Load so existing on-disk caches remain readable until they
// are rewritten by the next Save.
const (
	binaryMagic   uint32 = 0x4b464943 // "KFIC"
	binaryVersion uint32 = 1
	// binaryHeaderLen = magic(4) + version(4) + crc32(4) + payloadLen(8)
	binaryHeaderLen = 20
)

var (
	binaryZstdEncoder *zstd.Encoder
	binaryZstdDecoder *zstd.Decoder
)

func init() {
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault))
	if err != nil {
		panic(fmt.Sprintf("init cache zstd encoder: %v", err))
	}
	binaryZstdEncoder = enc
	dec, err := zstd.NewReader(nil)
	if err != nil {
		panic(fmt.Sprintf("init cache zstd decoder: %v", err))
	}
	binaryZstdDecoder = dec
}

// binaryEntry is the on-wire representation of a FileEntry. We persist a
// sorted slice (not a map) so the on-disk bytes are deterministic across
// runs — gob's map iteration order is unspecified, which would otherwise
// break the portability guarantees in portability_test.go.
type binaryEntry struct {
	Path    string
	Hash    string
	ModTime int64
	Size    int64
	Columns scanner.FindingColumns
}

type binaryPayload struct {
	Version   string
	RuleHash  string
	ScanPaths []string
	Entries   []binaryEntry
}

// hasBinaryMagic reports whether data is a binary cache frame. It accepts
// the bytes verbatim (no copy) and returns false for anything shorter than
// the magic itself, including legacy JSON content (which starts with '{').
func hasBinaryMagic(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	return binary.BigEndian.Uint32(data[:4]) == binaryMagic
}

func encodeBinary(c *Cache) ([]byte, error) {
	entries := make([]binaryEntry, 0, len(c.Files))
	for path, entry := range c.Files {
		entries = append(entries, binaryEntry{
			Path:    path,
			Hash:    entry.Hash,
			ModTime: entry.ModTime,
			Size:    entry.Size,
			Columns: entry.Columns,
		})
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Path < entries[j].Path
	})

	scanPaths := append([]string(nil), c.ScanPaths...)

	var gobBuf bytes.Buffer
	if err := gob.NewEncoder(&gobBuf).Encode(binaryPayload{
		Version:   c.Version,
		RuleHash:  c.RuleHash,
		ScanPaths: scanPaths,
		Entries:   entries,
	}); err != nil {
		return nil, fmt.Errorf("gob encode: %w", err)
	}

	compressed := binaryZstdEncoder.EncodeAll(gobBuf.Bytes(), nil)

	out := make([]byte, binaryHeaderLen, binaryHeaderLen+len(compressed))
	binary.BigEndian.PutUint32(out[0:4], binaryMagic)
	binary.BigEndian.PutUint32(out[4:8], binaryVersion)
	binary.BigEndian.PutUint32(out[8:12], crc32.ChecksumIEEE(compressed))
	binary.BigEndian.PutUint64(out[12:20], uint64(len(compressed)))
	out = append(out, compressed...)
	return out, nil
}

func decodeBinary(data []byte) (*Cache, bool) {
	if len(data) < binaryHeaderLen {
		return nil, false
	}
	if binary.BigEndian.Uint32(data[0:4]) != binaryMagic {
		return nil, false
	}
	if binary.BigEndian.Uint32(data[4:8]) != binaryVersion {
		return nil, false
	}
	wantCRC := binary.BigEndian.Uint32(data[8:12])
	payloadLen := binary.BigEndian.Uint64(data[12:20])
	if uint64(len(data)-binaryHeaderLen) != payloadLen {
		return nil, false
	}
	compressed := data[binaryHeaderLen:]
	if crc32.ChecksumIEEE(compressed) != wantCRC {
		return nil, false
	}

	raw, err := binaryZstdDecoder.DecodeAll(compressed, nil)
	if err != nil {
		return nil, false
	}
	var payload binaryPayload
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&payload); err != nil {
		return nil, false
	}

	files := make(map[string]FileEntry, len(payload.Entries))
	for _, e := range payload.Entries {
		files[e.Path] = FileEntry{
			Hash:    e.Hash,
			ModTime: e.ModTime,
			Size:    e.Size,
			Columns: e.Columns,
		}
	}
	return &Cache{
		Version:   payload.Version,
		RuleHash:  payload.RuleHash,
		ScanPaths: payload.ScanPaths,
		Files:     files,
	}, true
}
