package scanner

import (
	"bytes"
	"os"
	"testing"
)

func TestParsePackGetReadsFromDiskAndRejectsRewrittenPack(t *testing.T) {
	dir := t.TempDir()
	ps := newParsePackStore(dir)
	hash := "ab" + "cdef0123"
	payload := []byte("first payload bytes")
	if _, err := ps.SaveEncodedEntries([]parseEncodedEntryWrite{{hash: hash, data: payload}}); err != nil {
		t.Fatal(err)
	}

	// A fresh store has no in-memory copy of the pack and reads the entry from disk.
	fresh := newParsePackStore(dir)
	h, err := fresh.packForHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	got, ok := h.get(hash)
	if !ok || !bytes.Equal(got, payload) {
		t.Fatalf("get = %q, %v; want %q", got, ok, payload)
	}

	// Another writer replaces the pack with different content. The stale index must
	// read as a miss rather than return the wrong bytes.
	other := newParsePackStore(dir)
	if _, err := other.SaveEncodedEntries([]parseEncodedEntryWrite{
		{hash: hash, data: []byte("a considerably longer replacement payload")},
		{hash: "ab11112222", data: []byte("sibling")},
	}); err != nil {
		t.Fatal(err)
	}
	if got, ok := h.get(hash); ok {
		t.Fatalf("stale get returned %q, want miss", got)
	}
}

// A torn write can damage an entry's payload while the pack index still records the original
// length and CRC. Rebuilding that entry must rewrite it instead of trusting the index.
func TestParsePackPutRepairsCorruptPayloadWithMatchingIndex(t *testing.T) {
	dir := t.TempDir()
	hash := "ab" + "cdef0123"
	payload := []byte("payload that a torn write damages")
	if _, err := newParsePackStore(dir).SaveEncodedEntries([]parseEncodedEntryWrite{{hash: hash, data: payload}}); err != nil {
		t.Fatal(err)
	}
	h, err := newParsePackStore(dir).packForHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(h.path)
	if err != nil {
		t.Fatal(err)
	}
	at := bytes.Index(raw, payload)
	if at < 0 {
		t.Fatal("payload not found in pack")
	}
	raw[at] ^= 0xff
	if err := os.WriteFile(h.path, raw, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := newParsePackStore(dir).SaveEncodedEntries([]parseEncodedEntryWrite{{hash: hash, data: payload}}); err != nil {
		t.Fatal(err)
	}

	h, err = newParsePackStore(dir).packForHash(hash)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.ensureLoaded(); err != nil {
		t.Fatal(err)
	}
	if got, ok := h.get(hash); !ok || !bytes.Equal(got, payload) {
		t.Fatalf("after rebuild get = %q, %v; want %q", got, ok, payload)
	}
}
