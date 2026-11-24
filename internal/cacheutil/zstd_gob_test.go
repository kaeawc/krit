package cacheutil

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"reflect"
	"testing"

	"github.com/klauspost/compress/zstd"
)

// No maps: gob map order is unspecified, and the level test compares bytes.
type zstdTestPayload struct {
	Paths []string
	Lines [][]int
}

func newZstdTestPayload() zstdTestPayload {
	var p zstdTestPayload
	for i := 0; i < 2000; i++ {
		path := fmt.Sprintf("features/module%d/src/main/kotlin/io/example/feature%d/Screen%d.kt", i%37, i%11, i)
		p.Paths = append(p.Paths, path)
		p.Lines = append(p.Lines, []int{i, i * 3, i * 7 % 101})
	}
	return p
}

func gobBytes(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		t.Fatalf("gob encode: %v", err)
	}
	return buf.Bytes()
}

// The cache writers must use the shared ZstdEncoderLevel (SpeedFastest).
// Encoding is deterministic per level, so comparing against reference
// encoders pins the level actually used by EncodeZstdGob.
func TestEncodeZstdGobUsesFastestLevel(t *testing.T) {
	if ZstdEncoderLevel != zstd.SpeedFastest {
		t.Fatalf("ZstdEncoderLevel = %v, want SpeedFastest", ZstdEncoderLevel)
	}
	payload := newZstdTestPayload()
	raw := gobBytes(t, payload)

	got, err := EncodeZstdGob(payload)
	if err != nil {
		t.Fatalf("EncodeZstdGob: %v", err)
	}
	fastest, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedFastest), zstd.WithLowerEncoderMem(true))
	if err != nil {
		t.Fatal(err)
	}
	defaultLvl, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(zstd.SpeedDefault), zstd.WithLowerEncoderMem(true))
	if err != nil {
		t.Fatal(err)
	}
	wantFastest := fastest.EncodeAll(raw, nil)
	wantDefault := defaultLvl.EncodeAll(raw, nil)
	if bytes.Equal(wantFastest, wantDefault) {
		t.Fatal("test payload does not distinguish SpeedFastest from SpeedDefault")
	}
	if !bytes.Equal(got, wantFastest) {
		t.Fatalf("EncodeZstdGob output does not match SpeedFastest encoding (got %d bytes, fastest %d, default %d)",
			len(got), len(wantFastest), len(wantDefault))
	}
}

// Caches written before the level change (SpeedDefault) and at any other
// level must remain readable by the shared decoder.
func TestDecodeZstdGobReadsOtherLevels(t *testing.T) {
	payload := newZstdTestPayload()
	raw := gobBytes(t, payload)
	levels := []zstd.EncoderLevel{zstd.SpeedFastest, zstd.SpeedDefault, zstd.SpeedBetterCompression, zstd.SpeedBestCompression}
	for _, lvl := range levels {
		for _, lowMem := range []bool{true, false} {
			t.Run(fmt.Sprintf("%v/lowmem=%v", lvl, lowMem), func(t *testing.T) {
				enc, err := zstd.NewWriter(nil, zstd.WithEncoderLevel(lvl), zstd.WithLowerEncoderMem(lowMem))
				if err != nil {
					t.Fatal(err)
				}
				var got zstdTestPayload
				if err := DecodeZstdGobBytes(enc.EncodeAll(raw, nil), &got); err != nil {
					t.Fatalf("DecodeZstdGobBytes: %v", err)
				}
				if !reflect.DeepEqual(got, payload) {
					t.Fatal("decoded payload differs from original")
				}
			})
		}
	}
}
