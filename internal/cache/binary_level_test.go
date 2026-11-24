package cache

import (
	"encoding/binary"
	"hash/crc32"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// A binary cache frame written by an older krit at zstd SpeedDefault must
// still load after the shared cache level moved to SpeedFastest.
func TestDecodeBinaryReadsSpeedDefaultFrame(t *testing.T) {
	c := &Cache{
		Version:   "v-test",
		RuleHash:  "rules",
		ScanPaths: []string{"/repo"},
		Files: map[string]FileEntry{
			"/repo/A.kt": {Hash: "a", ModTime: 1, Size: 10},
			"/repo/B.kt": {Hash: "b", ModTime: 2, Size: 20},
		},
	}
	current, err := encodeBinary(c)
	if err != nil {
		t.Fatalf("encodeBinary: %v", err)
	}
	raw, err := binaryZstdDecoder.DecodeAll(current[binaryHeaderLen:], nil)
	if err != nil {
		t.Fatalf("decode current frame: %v", err)
	}
	enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault), zstd.WithLowerEncoderMem(true))
	if err != nil {
		t.Fatal(err)
	}
	compressed := enc.EncodeAll(raw, nil)
	legacy := make([]byte, binaryHeaderLen, binaryHeaderLen+len(compressed))
	copy(legacy, current[:8])
	binary.BigEndian.PutUint32(legacy[8:12], crc32.ChecksumIEEE(compressed))
	binary.BigEndian.PutUint64(legacy[12:20], uint64(len(compressed)))
	legacy = append(legacy, compressed...)

	got, ok := decodeBinary(legacy)
	if !ok {
		t.Fatal("decodeBinary rejected a SpeedDefault frame")
	}
	if got.Version != c.Version || got.RuleHash != c.RuleHash || len(got.Files) != len(c.Files) {
		t.Fatalf("decoded cache mismatch: %+v", got)
	}
	for path, want := range c.Files {
		if g := got.Files[path]; g.Hash != want.Hash || g.ModTime != want.ModTime || g.Size != want.Size {
			t.Fatalf("entry %s = %+v, want %+v", path, g, want)
		}
	}
}
