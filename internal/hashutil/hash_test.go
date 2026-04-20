package hashutil

import (
	"bytes"
	"encoding/hex"
	"os"
	"testing"
)

func TestHashHexKnownVector(t *testing.T) {
	// Canonical SHA-256 of "abc"
	const want = "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	got := HashHex([]byte("abc"))
	if got != want {
		t.Errorf("HashHex(\"abc\") = %q, want %q", got, want)
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
