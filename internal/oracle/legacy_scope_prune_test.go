package oracle

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A scope directory touched within the grace window may belong to a concurrent
// run, so pruning keeps it even when it is beyond the limit; stale ones go.
func TestPruneLegacyScopesSparesRecentlyUsedDirs(t *testing.T) {
	root := t.TempDir()
	old := time.Now().Add(-time.Hour)
	for _, n := range []string{"keep", "old1", "old2", "old3", "busy"} {
		d := filepath.Join(root, n)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		mt := old
		switch n {
		case "keep", "busy":
			mt = time.Now().Add(-time.Minute)
		}
		if err := os.Chtimes(d, mt, mt); err != nil {
			t.Fatal(err)
		}
	}
	pruneLegacyScopes(root, "keep", 1)
	for _, n := range []string{"keep", "busy"} {
		if _, err := os.Stat(filepath.Join(root, n)); err != nil {
			t.Fatalf("%s was pruned: %v", n, err)
		}
	}
	for _, n := range []string{"old1", "old2", "old3"} {
		if _, err := os.Stat(filepath.Join(root, n)); err == nil {
			t.Fatalf("stale scope %s survived", n)
		}
	}
}
