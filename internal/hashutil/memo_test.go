package hashutil

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/testutil"
)

// writeTemp writes content to a fresh file dated an hour ago, like an
// unchanged source on a warm run: only a stat older than RacyWindow is
// memoized.
func writeTemp(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	old := time.Now().Add(-time.Hour)
	if err := os.Chtimes(p, old, old); err != nil {
		t.Fatalf("chtimes: %v", err)
	}
	return p
}

func TestMemo_SameTickRewriteOfFreshFileIsRehashed(t *testing.T) {
	cases := map[string]func(m *Memo, p string) string{
		"HashFile": func(m *Memo, p string) string {
			hx, err := m.HashFile(p, nil)
			if err != nil {
				t.Fatal(err)
			}
			return hx
		},
		"HashFileRaw": func(m *Memo, p string) string {
			raw, err := m.HashFileRaw(p, nil)
			if err != nil {
				t.Fatal(err)
			}
			return string(raw[:])
		},
		"HashContent": func(m *Memo, p string) string {
			b, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			return m.HashContent(p, b)
		},
	}
	for name, hash := range cases {
		t.Run(name, func(t *testing.T) {
			p := filepath.Join(t.TempDir(), "A.kt")
			if err := os.WriteFile(p, []byte("fun b() = 2"), 0o644); err != nil {
				t.Fatal(err)
			}
			m := NewMemo()
			before := hash(m, p)
			testutil.RewriteKeepingStat(t, p, "fun b() = 3")
			if hash(m, p) == before {
				t.Fatal("a same-size rewrite within one mtime tick of a fresh file served the stale hash")
			}
			if m.Len() != 0 {
				t.Errorf("a file modified within RacyWindow was memoized: %d entries", m.Len())
			}
		})
	}
}

func TestMemo_OldFileStillMemoized(t *testing.T) {
	p := writeTemp(t, "fun b() = 2")
	m := NewMemo()
	before, _ := m.HashFile(p, nil)
	// An old stat proves the content: a real rewrite would carry a new mtime,
	// so the memo may (and, for warm-scan speed, must) serve the entry.
	testutil.RewriteKeepingStat(t, p, "fun b() = 3")
	if got, _ := m.HashFile(p, nil); got != before {
		t.Fatalf("an old, unchanged stat must hit the memo")
	}
	if hits, misses := m.Stats(); hits != 1 || misses != 1 {
		t.Errorf("stats = (%d,%d); want (1,1)", hits, misses)
	}
}

func TestStatTrusted(t *testing.T) {
	at := time.Unix(1_000_000, 0)
	for _, tc := range []struct {
		mtime time.Time
		want  bool
	}{
		{at.Add(-time.Hour), true},
		{at.Add(-RacyWindow - time.Nanosecond), true},
		{at.Add(-RacyWindow), false},
		{at.Add(-time.Millisecond), false},
		{at, false},
		{at.Add(time.Hour), false}, // future-dated: clock skew or a restored archive
	} {
		if got := StatTrusted(tc.mtime, at); got != tc.want {
			t.Errorf("StatTrusted(at%+v) = %v, want %v", tc.mtime.Sub(at), got, tc.want)
		}
	}
}

func TestMemo_HashFile_HitAfterFirstCall(t *testing.T) {
	p := writeTemp(t, "hello world")
	m := NewMemo()

	h1, err := m.HashFile(p, nil)
	if err != nil {
		t.Fatalf("first HashFile: %v", err)
	}
	h2, err := m.HashFile(p, nil)
	if err != nil {
		t.Fatalf("second HashFile: %v", err)
	}
	if h1 != h2 {
		t.Errorf("hashes differ: %q vs %q", h1, h2)
	}
	hits, misses := m.Stats()
	if hits != 1 || misses != 1 {
		t.Errorf("stats = (%d,%d); want (1,1)", hits, misses)
	}
}

func TestMemo_NilReceiverFallsThrough(t *testing.T) {
	p := writeTemp(t, "nil-path")
	var m *Memo
	h, err := m.HashFile(p, nil)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	want, _ := HashFile(p)
	if h != want {
		t.Errorf("got %q, want %q", h, want)
	}
}

func TestMemo_DetectsMtimeChange(t *testing.T) {
	p := writeTemp(t, "v1")
	m := NewMemo()
	h1, _ := m.HashFile(p, nil)

	// Rewrite with distinct content and advance mtime explicitly so the
	// test doesn't rely on same-millisecond filesystem resolution.
	if err := os.WriteFile(p, []byte("v2-different"), 0o644); err != nil {
		t.Fatalf("rewrite: %v", err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatalf("chtimes: %v", err)
	}

	h2, _ := m.HashFile(p, nil)
	if h1 == h2 {
		t.Errorf("expected different hashes after rewrite; got %q twice", h1)
	}
	_, misses := m.Stats()
	if misses != 2 {
		t.Errorf("want two misses, got %d", misses)
	}
}

func TestMemo_HashContentMemoizesForHashFile(t *testing.T) {
	p := writeTemp(t, "pre-read-content")
	m := NewMemo()

	content, _ := os.ReadFile(p)
	first := m.HashContent(p, content)
	second, _ := m.HashFile(p, func() ([]byte, error) {
		t.Fatalf("provider should not be called on hit")
		return nil, nil
	})
	if first != second {
		t.Errorf("HashContent then HashFile disagreed: %q vs %q", first, second)
	}
	hits, _ := m.Stats()
	if hits == 0 {
		t.Errorf("expected at least one hit, got 0")
	}
}

func TestMemo_ProviderUsedOnMiss(t *testing.T) {
	p := writeTemp(t, "disk-bytes")
	m := NewMemo()

	called := false
	got, err := m.HashFile(p, func() ([]byte, error) {
		called = true
		return []byte("provided-bytes"), nil
	})
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	if !called {
		t.Fatalf("provider not invoked on miss")
	}
	want := HashHex([]byte("provided-bytes"))
	if got != want {
		t.Errorf("got %q, want %q (provider bytes should be hashed, not disk bytes)", got, want)
	}
}

func TestMemo_HashFileRaw_MatchesHex(t *testing.T) {
	p := writeTemp(t, "raw-bytes")
	m := NewMemo()

	hx, err := m.HashFile(p, nil)
	if err != nil {
		t.Fatalf("HashFile: %v", err)
	}
	raw, err := m.HashFileRaw(p, nil)
	if err != nil {
		t.Fatalf("HashFileRaw: %v", err)
	}
	hexExpected := HashBytes([]byte("raw-bytes"))
	if raw != hexExpected {
		t.Errorf("raw hash mismatch")
	}
	// Sanity: hex should encode the same digest.
	if want := HashHex([]byte("raw-bytes")); hx != want {
		t.Errorf("hex mismatch: %q vs %q", hx, want)
	}
}

func TestMemo_Clear(t *testing.T) {
	p := writeTemp(t, "clear-me")
	m := NewMemo()
	_, _ = m.HashFile(p, nil)
	if m.Len() != 1 {
		t.Fatalf("expected 1 entry, got %d", m.Len())
	}
	m.Clear()
	if m.Len() != 0 {
		t.Errorf("expected 0 entries after Clear, got %d", m.Len())
	}
	h, m2 := m.Stats()
	if h != 0 || m2 != 0 {
		t.Errorf("stats not reset after Clear: (%d,%d)", h, m2)
	}
}

func TestMemo_ConcurrentHashFile(t *testing.T) {
	paths := make([]string, 8)
	for i := range paths {
		paths[i] = writeTemp(t, "concurrent-"+string(rune('a'+i)))
	}
	m := NewMemo()

	var wg sync.WaitGroup
	for g := 0; g < 16; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				for _, p := range paths {
					if _, err := m.HashFile(p, nil); err != nil {
						t.Errorf("HashFile: %v", err)
						return
					}
				}
			}
		}()
	}
	wg.Wait()

	if m.Len() != len(paths) {
		t.Errorf("expected %d entries, got %d", len(paths), m.Len())
	}
}

func TestDefault_ResetDefault(t *testing.T) {
	p := writeTemp(t, "default-memo")
	ResetDefault()
	if _, err := Default().HashFile(p, nil); err != nil {
		t.Fatalf("HashFile via Default: %v", err)
	}
	if Default().Len() == 0 {
		t.Errorf("expected an entry in default memo")
	}
	ResetDefault()
	if Default().Len() != 0 {
		t.Errorf("ResetDefault did not clear entries")
	}
}
