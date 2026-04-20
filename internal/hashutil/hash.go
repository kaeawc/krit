package hashutil

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
)

// HashBytes returns the raw SHA-256 digest of b. Use this when the consumer
// wants [32]byte (e.g. store.Key.FileHash).
func HashBytes(b []byte) [32]byte {
	return sha256.Sum256(b)
}

// HashHex returns the SHA-256 digest of b as a lowercase hex string. Use this
// for on-disk cache keys, fingerprints, and any user-visible identifier.
func HashHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// HashReader streams from r into a SHA-256, returning the lowercase hex digest.
// Used by the oracle content-hash path to avoid reading whole files into memory.
func HashReader(r io.Reader) (string, error) {
	h := sha256.New()
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
