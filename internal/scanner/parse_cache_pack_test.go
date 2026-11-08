package scanner

import (
	"bytes"
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
