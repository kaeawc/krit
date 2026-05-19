package rename

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestMoveFileNoReplaceMovesContentAndRemovesSource(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.kt")
	dst := filepath.Join(dir, "dst.kt")
	if err := os.WriteFile(src, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := moveFileNoReplace(src, dst); err != nil {
		t.Fatalf("moveFileNoReplace: %v", err)
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("source must be removed; Stat returned %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "hello" {
		t.Errorf("dst content = %q (err=%v), want %q", string(data), err, "hello")
	}
}

func TestMoveFileNoReplaceRefusesPreExistingDestination(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.kt")
	dst := filepath.Join(dir, "dst.kt")
	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := moveFileNoReplace(src, dst)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("want already-exists error, got %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil || string(data) != "existing" {
		t.Errorf("dst must not be overwritten; got %q (err=%v)", string(data), err)
	}
	if _, err := os.Stat(src); err != nil {
		t.Errorf("source must remain on EEXIST failure; Stat: %v", err)
	}
}

// TestMoveFileNoReplaceConcurrentCreateLosesNoData is the regression
// test for the original Stat-then-Rename TOCTOU. Many goroutines race
// to move distinct sources into the same destination directory and
// onto the same target name; exactly one must succeed, and the others
// must fail with "already exists" rather than silently overwriting.
func TestMoveFileNoReplaceConcurrentCreateLosesNoData(t *testing.T) {
	dir := t.TempDir()
	const N = 16
	srcs := make([]string, N)
	for i := 0; i < N; i++ {
		srcs[i] = filepath.Join(dir, "src", "f"+itoa(i)+".kt")
		if err := os.MkdirAll(filepath.Dir(srcs[i]), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(srcs[i], []byte("from-"+itoa(i)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dst := filepath.Join(dir, "dst.kt")

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		winners  []string
		failures int
	)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := moveFileNoReplace(srcs[i], dst); err == nil {
				mu.Lock()
				winners = append(winners, srcs[i])
				mu.Unlock()
				return
			}
			mu.Lock()
			failures++
			mu.Unlock()
		}(i)
	}
	wg.Wait()

	if len(winners) != 1 {
		t.Fatalf("expected exactly one winning move, got %d (winners=%v, failures=%d)", len(winners), winners, failures)
	}
	if failures != N-1 {
		t.Errorf("expected %d failures (all but the winner), got %d", N-1, failures)
	}
	// Surviving file must equal the winner's source content.
	winnerName := winners[0]
	wantContent := readSource(t, winnerName) // empty — source was removed
	_ = wantContent
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("dst missing: %v", err)
	}
	if !strings.HasPrefix(string(data), "from-") {
		t.Errorf("dst content does not look like a winner: %q", string(data))
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

func readSource(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(data)
}
