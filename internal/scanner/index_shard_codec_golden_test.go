package scanner

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"reflect"
	"testing"
)

// goldenShard returns a small, fully-specified shard used to lock the
// on-disk codec format. Hand-built (not RNG-based) so the encoded bytes
// are stable across stdlib changes.
func goldenShard() *fileShard {
	path := "/proj/Foo.kt"
	return &fileShard{
		Version:     0, // decode rewrites this; keep zero for the input
		Path:        path,
		ContentHash: "abc123",
		Symbols: []Symbol{
			{
				Name:       "Foo",
				Kind:       "class",
				Visibility: "public",
				File:       path,
				Line:       3,
				StartByte:  10,
				EndByte:    50,
				Language:   LangKotlin,
				Package:    "com.example",
				FQN:        "com.example.Foo",
				Owner:      "",
				Signature:  "",
				Arity:      0,
				IsFinal:    true,
			},
			{
				Name:       "bar",
				Kind:       "function",
				Visibility: "private",
				File:       path,
				Line:       7,
				StartByte:  60,
				EndByte:    90,
				Language:   LangKotlin,
				Package:    "com.example",
				FQN:        "com.example.Foo.bar",
				Owner:      "Foo",
				Signature:  "(Int) -> String",
				Arity:      1,
				IsOverride: true,
			},
		},
		References: []Reference{
			{Name: "println", File: path, Line: 8, InComment: false},
			{Name: "Foo", File: path, Line: 11, InComment: false},
			{Name: "TODO", File: path, Line: 12, InComment: true},
		},
		// Bloom is opaque bytes from the codec's perspective; pin a small
		// fixed payload so encoded output is stable.
		Bloom: []byte{0x00, 0xAA, 0xFF, 0x42},
	}
}

// TestShardCodec_EncodeIsDeterministic asserts encodeShardPayload produces
// byte-identical output across repeated calls on the same input. This is
// the most important property for cache portability: any nondeterminism in
// encoding (map iteration, time, random) breaks cross-machine reuse.
func TestShardCodec_EncodeIsDeterministic(t *testing.T) {
	s := goldenShard()
	first := encodeShardPayload(s)
	for i := 0; i < 16; i++ {
		got := encodeShardPayload(s)
		if !bytes.Equal(first, got) {
			t.Fatalf("encode iter %d differs (%d vs %d bytes)", i, len(first), len(got))
		}
	}
}

// TestShardCodec_RoundTrip verifies encode -> decode -> encode produces the
// same bytes. Catches asymmetric codec drift where decode silently drops or
// re-orders fields.
func TestShardCodec_RoundTrip(t *testing.T) {
	s := goldenShard()
	encoded := encodeShardPayload(s)

	decoded, err := decodeShardPayload(encoded, s.Path)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Path is set by the decoder; for the structural compare, ignore the
	// Version field (decoder injects current version).
	want := *s
	got := *decoded
	got.Version = want.Version
	if !reflect.DeepEqual(want.Symbols, got.Symbols) {
		t.Errorf("symbols differ:\nwant=%#v\ngot=%#v", want.Symbols, got.Symbols)
	}
	if !reflect.DeepEqual(want.References, got.References) {
		t.Errorf("references differ:\nwant=%#v\ngot=%#v", want.References, got.References)
	}
	if !bytes.Equal(want.Bloom, got.Bloom) {
		t.Errorf("bloom differs: want=%x got=%x", want.Bloom, got.Bloom)
	}

	reEncoded := encodeShardPayload(decoded)
	if !bytes.Equal(encoded, reEncoded) {
		t.Fatalf("encode->decode->encode is not byte-stable\nfirst=%x\nsecond=%x", encoded, reEncoded)
	}
}

// TestShardCodec_GoldenSHA256 pins the SHA-256 of the golden shard's
// encoded bytes. A change here means the on-disk format moved; bump
// shardPayloadVersion AND update this hash deliberately. This is the
// safety net against accidental wire-format drift.
func TestShardCodec_GoldenSHA256(t *testing.T) {
	encoded := encodeShardPayload(goldenShard())
	sum := sha256.Sum256(encoded)
	got := hex.EncodeToString(sum[:])

	const want = "f7f3ff2908f1d9d244dd422bd48d96a20624e592ed080f7d468055f6ce07d00b"

	if got != want {
		t.Fatalf("golden shard digest changed:\nwant=%s\ngot =%s\nencoded=%x\nthis means the shard codec wire format moved; bump shardPayloadVersion and update this digest deliberately", want, got, encoded)
	}
}

// TestShardCodec_RejectsBadMagic verifies decode fails cleanly when the
// magic header is wrong. A cache restored from an older krit (or from an
// unrelated tool) must be rejected, not silently misinterpreted.
func TestShardCodec_RejectsBadMagic(t *testing.T) {
	encoded := encodeShardPayload(goldenShard())
	corrupt := append([]byte(nil), encoded...)
	// First 4 bytes are the magic; flip them.
	corrupt[0] ^= 0xFF
	corrupt[1] ^= 0xFF
	corrupt[2] ^= 0xFF
	corrupt[3] ^= 0xFF

	if _, err := decodeShardPayload(corrupt, "/proj/Foo.kt"); err == nil {
		t.Fatal("decode accepted payload with bad magic; expected rejection")
	}
}

// TestShardCodec_RejectsBadVersion verifies decode fails when the version
// field doesn't match the build's shardPayloadVersion. This is the
// migration safety: stale caches from an older format must invalidate, not
// produce garbage symbols.
func TestShardCodec_RejectsBadVersion(t *testing.T) {
	encoded := encodeShardPayload(goldenShard())
	corrupt := append([]byte(nil), encoded...)
	// Version is little-endian uint16 at bytes 4-5. Bump it.
	corrupt[4] = 0xFF
	corrupt[5] = 0xFF

	if _, err := decodeShardPayload(corrupt, "/proj/Foo.kt"); err == nil {
		t.Fatal("decode accepted payload with bad version; expected rejection")
	}
}

// TestShardCodec_RejectsTruncated verifies decode fails on truncated input
// rather than panicking or returning a half-built shard.
func TestShardCodec_RejectsTruncated(t *testing.T) {
	encoded := encodeShardPayload(goldenShard())
	for _, n := range []int{0, 1, 4, 6, 10, len(encoded) - 1} {
		if n < 0 || n >= len(encoded) {
			continue
		}
		if _, err := decodeShardPayload(encoded[:n], "/proj/Foo.kt"); err == nil {
			t.Errorf("decode accepted truncated payload of length %d; expected rejection", n)
		}
	}
}
