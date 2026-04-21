package hashutil

import (
	"encoding/binary"
	"encoding/hex"
	"hash"
	"io"
	"os"

	"github.com/zeebo/xxh3"
)

// ContentHasher abstracts the hash function used for content fingerprints
// (parse cache, cross-file cache, oracle cache, incremental cache).
// Implementations must return a 32-byte digest so existing store.Key
// FileHash [32]byte layouts keep working.
//
// The default is xxh3-128 widened to 256 bits via two distinct seeds —
// non-crypto but SIMD-accelerated on both amd64 (AVX / AVX-512) and
// arm64 (NEON), so it stays ahead of hardware SHA-256 on Apple Silicon
// (~5×) and beats software SHA-256 on typical Linux CI hardware by
// ~10×. The interface leaves room for swapping in blake3, wyhash, or
// a successor via SetContentHasher without disturbing any callers.
type ContentHasher interface {
	// Name is a stable identifier embedded in cache version tokens so
	// an algorithm swap automatically invalidates prior entries.
	Name() string
	// Sum returns the 32-byte digest of b.
	Sum(b []byte) [32]byte
	// New returns a streaming hash.Hash whose Sum(nil) matches
	// Sum(b)[:] when fed the same bytes. Size() is 32.
	New() hash.Hash
}

// xxh3 needs 128 bits × 2 to fill the [32]byte store.Key width. Two
// distinct seeds give an effectively independent pair of 128-bit
// hashes; we concatenate them as hi:lo in memory order. Birthday
// bound is 2^128 — far beyond any plausible krit corpus.
const (
	xxh3SeedLo uint64 = 0
	xxh3SeedHi uint64 = 0xC3A5_C85C_97CB_3127
)

type xxh3Hasher struct{}

func (xxh3Hasher) Name() string { return "xxh3-256" }

func (xxh3Hasher) Sum(b []byte) [32]byte {
	return packXxh3Pair(xxh3.Hash128Seed(b, xxh3SeedLo), xxh3.Hash128Seed(b, xxh3SeedHi))
}

// packXxh3Pair serializes two xxh3 128-bit digests as hi:lo big-endian
// uint64 quads into a single [32]byte. Shared between the one-shot
// xxh3Hasher.Sum and the streaming xxh3DualHasher.Sum so both paths
// produce byte-identical output for the same input.
func packXxh3Pair(lo, hi xxh3.Uint128) [32]byte {
	var out [32]byte
	binary.BigEndian.PutUint64(out[0:8], lo.Hi)
	binary.BigEndian.PutUint64(out[8:16], lo.Lo)
	binary.BigEndian.PutUint64(out[16:24], hi.Hi)
	binary.BigEndian.PutUint64(out[24:32], hi.Lo)
	return out
}

func (xxh3Hasher) New() hash.Hash {
	return &xxh3DualHasher{
		lo: xxh3.NewSeed128(xxh3SeedLo),
		hi: xxh3.NewSeed128(xxh3SeedHi),
	}
}

// xxh3DualHasher runs two seeded xxh3 128-bit hashers in lock-step so
// Sum returns the concatenation (32 bytes total). Both hashers receive
// the same Write input, so the concatenated digest matches the Sum path
// exactly.
type xxh3DualHasher struct {
	lo *xxh3.Hasher128
	hi *xxh3.Hasher128
}

func (h *xxh3DualHasher) Write(p []byte) (int, error) {
	_, _ = h.lo.Write(p)
	_, _ = h.hi.Write(p)
	return len(p), nil
}

func (h *xxh3DualHasher) Sum(b []byte) []byte {
	out := packXxh3Pair(h.lo.Sum128(), h.hi.Sum128())
	return append(b, out[:]...)
}

func (h *xxh3DualHasher) Reset() {
	h.lo.ResetSeed(xxh3SeedLo)
	h.hi.ResetSeed(xxh3SeedHi)
}

func (h *xxh3DualHasher) Size() int      { return 32 }
func (h *xxh3DualHasher) BlockSize() int { return h.lo.BlockSize() }

var activeHasher ContentHasher = xxh3Hasher{}

// Hasher returns the currently installed ContentHasher. Subsystems that
// need streaming semantics (e.g. oracle closureFingerprint) call
// Hasher().New() instead of importing a specific algorithm.
func Hasher() ContentHasher { return activeHasher }

// SetContentHasher swaps the active hasher. Intended for benchmarks and
// future algorithm experiments; production code should leave the default
// (xxh3-256) in place. The swap is NOT safe for concurrent use — call it
// during process init or from a test's SetUp before any hashing runs.
func SetContentHasher(h ContentHasher) { activeHasher = h }

// HasherName returns the Name() of the active ContentHasher. Embedded
// in cache version tokens so an algorithm swap auto-invalidates every
// cache that keys on content hash.
func HasherName() string { return activeHasher.Name() }

// HashBytes returns the raw 32-byte digest of b.
func HashBytes(b []byte) [32]byte {
	return activeHasher.Sum(b)
}

// HashHex returns the digest of b as a lowercase hex string. Use this
// for on-disk cache keys, fingerprints, and any user-visible identifier.
func HashHex(b []byte) string {
	h := activeHasher.Sum(b)
	return hex.EncodeToString(h[:])
}

// HashReader streams r into the active hasher and returns the lowercase
// hex digest. Used by the oracle content-hash path to avoid reading
// whole files into memory.
func HashReader(r io.Reader) (string, error) {
	h := activeHasher.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// HashFile is a convenience wrapper that opens path, streams it through
// HashReader, and returns the hex digest. Returns the underlying os error
// on open failure (do not wrap — oracle.ContentHash's current callers
// check os.IsNotExist on the return).
func HashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	return HashReader(f)
}
