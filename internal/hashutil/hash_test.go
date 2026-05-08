package hashutil

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
)

func TestHashHexDeterministic(t *testing.T) {
	// The default (xxh3-256) is a dual-seed construction, not a
	// standardized algorithm, so there's no external test vector to
	// pin against. Assert self-consistency and non-trivial mixing
	// instead.
	a := HashHex([]byte("abc"))
	b := HashHex([]byte("abc"))
	if a != b {
		t.Fatalf("HashHex non-deterministic: %q vs %q", a, b)
	}
	if len(a) != 64 {
		t.Fatalf("HashHex(\"abc\") length = %d, want 64", len(a))
	}
	if HashHex([]byte("abc")) == HashHex([]byte("abd")) {
		t.Fatal("HashHex collapses single-byte diff")
	}
}

func TestHashBytesConsistency(t *testing.T) {
	data := []byte("hello, world! this is arbitrary test data 12345")
	raw := HashBytes(data)
	fromBytes := hex.EncodeToString(raw[:])
	fromHex := HashHex(data)
	if fromBytes != fromHex {
		t.Errorf("HashBytes and HashHex are inconsistent: %q vs %q", fromBytes, fromHex)
	}
}

func TestHashReaderParity(t *testing.T) {
	// 1 MiB buffer
	buf := bytes.Repeat([]byte{0xAB, 0xCD, 0xEF}, 1<<20/3+1)
	buf = buf[:1<<20]

	readerResult, err := HashReader(bytes.NewReader(buf))
	if err != nil {
		t.Fatalf("HashReader error: %v", err)
	}
	hexResult := HashHex(buf)
	if readerResult != hexResult {
		t.Errorf("HashReader = %q, HashHex = %q; want equal", readerResult, hexResult)
	}
}

func TestHashFileMissing(t *testing.T) {
	_, err := HashFile("/nonexistent/path/that/does/not/exist/hashutil_test_sentinel")
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
	if !os.IsNotExist(err) {
		t.Errorf("expected os.IsNotExist(err) to be true, got err = %v", err)
	}
}

func TestHasherNameDefault(t *testing.T) {
	if got := HasherName(); got != "xxh3-256" {
		t.Errorf("HasherName() = %q, want %q", got, "xxh3-256")
	}
}
