package store

import (
	"bytes"
	"testing"
)

func makeKey(kind StoreKind, fileByte, ruleByte byte) Key {
	var k Key
	k.Kind = kind
	k.FileHash[0] = fileByte
	k.RuleSetHash[0] = ruleByte
	return k
}

func TestGetMissOnEmpty(t *testing.T) {
	s := New(t.TempDir())
	_, ok := s.Get(makeKey(KindIncremental, 0xAB, 0x01))
	if ok {
		t.Fatal("expected miss on empty store")
	}
}

func TestPutGet(t *testing.T) {
	s := New(t.TempDir())
	key := makeKey(KindIncremental, 0xAB, 0x01)
	payload := []byte(`{"findings":42}`)

	if err := s.Put(key, payload); err != nil {
		t.Fatalf("Put: %v", err)
	}
	got, ok := s.Get(key)
	if !ok {
		t.Fatal("expected hit after Put")
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("payload mismatch: got %q want %q", got, payload)
	}
}

func TestPutOverwrites(t *testing.T) {
	s := New(t.TempDir())
	key := makeKey(KindOracle, 0x11, 0x22)

	if err := s.Put(key, []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if err := s.Put(key, []byte("v2")); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(key)
	if string(got) != "v2" {
		t.Fatalf("expected v2, got %q", got)
	}
}

func TestDifferentKindsIsolated(t *testing.T) {
	s := New(t.TempDir())
	// Same file+rule hashes but different kinds must be independent.
	k1 := makeKey(KindIncremental, 0xFF, 0x01)
	k2 := makeKey(KindOracle, 0xFF, 0x01)

	s.Put(k1, []byte("incremental"))
	s.Put(k2, []byte("oracle"))

	v1, _ := s.Get(k1)
	v2, _ := s.Get(k2)
	if string(v1) != "incremental" || string(v2) != "oracle" {
		t.Fatalf("kind isolation broken: %q %q", v1, v2)
	}
}

func TestInvalidateClearsAll(t *testing.T) {
	s := New(t.TempDir())
	keys := []Key{
		makeKey(KindIncremental, 0x01, 0x01),
		makeKey(KindOracle, 0x02, 0x02),
		makeKey(KindMatrix, 0x03, 0x03),
	}
	for _, k := range keys {
		s.Put(k, []byte("data"))
	}

	if err := s.Invalidate(); err != nil {
		t.Fatalf("Invalidate: %v", err)
	}
	for _, k := range keys {
		if _, ok := s.Get(k); ok {
			t.Fatalf("entry still present after Invalidate")
		}
	}
}

func TestInvalidateOnEmptyStore(t *testing.T) {
	s := New(t.TempDir())
	if err := s.Invalidate(); err != nil {
		t.Fatalf("Invalidate on empty store: %v", err)
	}
}

func TestStats(t *testing.T) {
	s := New(t.TempDir())
	for i := byte(0); i < 5; i++ {
		s.Put(makeKey(KindIncremental, i, 0x01), []byte("payload"))
	}

	st, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.EntryCount != 5 {
		t.Fatalf("expected 5 entries, got %d", st.EntryCount)
	}
	if st.TotalBytes == 0 {
		t.Fatal("expected non-zero total bytes")
	}
}

func TestStatsEmptyStore(t *testing.T) {
	s := New(t.TempDir())
	st, err := s.Stats()
	if err != nil {
		t.Fatalf("Stats on empty: %v", err)
	}
	if st.EntryCount != 0 || st.TotalBytes != 0 {
		t.Fatalf("expected empty stats, got %+v", st)
	}
}
