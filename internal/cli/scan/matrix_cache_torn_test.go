package scan

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/store"
)

// TestMatrixBaseline_TornEntryIsMiss covers a baseline cache entry damaged by
// a crash (cache writes skip fsync): it must be a miss, not a stale report.
func TestMatrixBaseline_TornEntryIsMiss(t *testing.T) {
	storeDir := t.TempDir()
	s := store.New(storeDir)
	key := strings.Repeat("ab", 32)
	saveBaseline(key, matrixCaseReport{Name: "baseline", MeanFindings: 3}, s)

	var entry string
	_ = filepath.WalkDir(storeDir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			entry = p
		}
		return nil
	})
	if entry == "" {
		t.Fatal("no baseline entry written")
	}
	valid, err := os.ReadFile(entry)
	if err != nil {
		t.Fatal(err)
	}
	if r, ok := tryLoadBaseline(key, s); !ok || r.MeanFindings != 3 {
		t.Fatalf("valid entry: ok=%v report=%+v", ok, r)
	}
	for name, data := range map[string][]byte{
		"empty":       {},
		"truncated":   valid[:len(valid)/2],
		"chopped":     valid[:len(valid)-1],
		"zero-filled": make([]byte, len(valid)),
	} {
		if err := os.WriteFile(entry, data, 0o644); err != nil {
			t.Fatal(err)
		}
		if _, ok := tryLoadBaseline(key, s); ok {
			t.Errorf("%s entry loaded as a hit", name)
		}
	}
}
