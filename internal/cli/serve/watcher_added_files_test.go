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
	// The file exists before the watcher starts, so the real filesystem
	// events cannot add a Create of their own to the dirty set; the test
	// delivers the Create by hand.
	created := filepath.Join(root, "New.kt")
	if err := os.WriteFile(created, []byte("class New\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := pipeline.NewWorkspaceState(root)
	w, err := startFileWatcherWithState(context.Background(), root, ws, nil, withDebounceWindow(time.Hour))
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()
	<-w.Ready()

	w.handle(backendEvent{Path: created, Op: opCreate})
	if got := drainedBases(ws); len(got) != 1 || got[0] != "New.kt" {
		t.Fatalf("dirty after a create = %v, want [New.kt] (an hour-long debounce must not delay it)", got)
	}

	// A plain write is reported at once too (#832): the first event of a burst
	// invalidates immediately, and only the repeat for later writes in the
	// burst waits out the debounce window.
	// The event is delivered by hand for a path nothing writes on disk, so no
	// real filesystem event joins its burst.
	edited := filepath.Join(root, "Edited.kt")
	w.handle(backendEvent{Path: edited, Op: opWrite})
	if got := drainedBases(ws); len(got) != 1 || got[0] != "Edited.kt" {
		t.Fatalf("dirty after a write = %v, want [Edited.kt] (the first event of a burst is not debounced)", got)
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

// A directory moved out of the tree delivers one event for the directory and
// none for the files in it; the watcher must still report it.
func TestFileWatcher_DirectoryMovedOutIsReported(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "S.kt"), []byte("class S\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := pipeline.NewWorkspaceState(root)
	w, err := startFileWatcherWithState(context.Background(), root, ws, nil, withDebounceWindow(time.Hour))
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()
	<-w.Ready()

	if err := os.Rename(sub, filepath.Join(t.TempDir(), "sub")); err != nil {
		t.Skipf("cannot move a directory out of the project: %v", err)
	}
	var got []string
	if !waitForCondition(func() bool {
		got = append(got, drainedBases(ws)...)
		for _, b := range got {
			if b == "sub" {
				return true
			}
		}
		return false
	}) {
		t.Fatalf("dirty after moving a directory out = %v, want it to include sub", got)
	}
}
