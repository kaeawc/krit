package serve

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/pipeline"
)

func drainedBases(ws *pipeline.WorkspaceState) []string {
	var out []string
	for _, p := range ws.DrainDirty() {
		out = append(out, filepath.Base(p))
	}
	sort.Strings(out)
	return out
}

// A file created inside the debounce window must already be dirty: an analyze
// that arrives in that window used to see no dirty path and trust source-path
// lists that predate the file.
func TestFileWatcher_CreateIsDirtyBeforeTheDebounceWindowElapses(t *testing.T) {
	root := t.TempDir()
	ws := pipeline.NewWorkspaceState(root)
	w, err := startFileWatcherWithState(context.Background(), root, ws, nil, withDebounceWindow(time.Hour))
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()
	<-w.Ready()

	created := filepath.Join(root, "New.kt")
	if err := os.WriteFile(created, []byte("class New\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.handle(backendEvent{Path: created, Op: opCreate})
	if got := drainedBases(ws); len(got) != 1 || got[0] != "New.kt" {
		t.Fatalf("dirty after a create = %v, want [New.kt] (an hour-long debounce must not delay it)", got)
	}

	// A plain write is still debounced.
	if err := os.WriteFile(created, []byte("class New2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	w.handle(backendEvent{Path: created, Op: opWrite})
	if got := drainedBases(ws); len(got) != 0 {
		t.Fatalf("dirty after a write = %v, want none until the debounce window elapses", got)
	}
}

// A directory that arrives already populated (mv, cp -r, git stash pop) emits
// no events for its contents; its sources must still become dirty.
func TestFileWatcher_MovedInDirectoryReportsItsSources(t *testing.T) {
	root := t.TempDir()
	ws := pipeline.NewWorkspaceState(root)
	w, err := startFileWatcherWithState(context.Background(), root, ws, nil, withDebounceWindow(time.Hour))
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()
	<-w.Ready()

	staging := t.TempDir()
	moved := filepath.Join(staging, "pkg")
	if err := os.MkdirAll(filepath.Join(moved, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	for rel, body := range map[string]string{
		"A.kt":              "class A\n",
		"nested/B.java":     "class B {}\n",
		"nested/script.kts": "println(1)\n",
		"nested/readme.md":  "not source\n",
	} {
		if err := os.WriteFile(filepath.Join(moved, rel), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dest := filepath.Join(root, "pkg")
	if err := os.Rename(moved, dest); err != nil {
		t.Skipf("cannot move a directory across the temp dirs: %v", err)
	}
	w.handle(backendEvent{Path: dest, Op: opCreate})

	want := []string{"A.kt", "B.java", "script.kts"}
	if got := drainedBases(ws); len(got) != len(want) || got[0] != want[0] || got[1] != want[1] || got[2] != want[2] {
		t.Fatalf("dirty after a directory move-in = %v, want %v", got, want)
	}
}
