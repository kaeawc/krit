package serve

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/pipeline"
)

// writeTree makes dirs directories of files empty files each under root.
func writeTree(t *testing.T, root string, dirs, files int) {
	t.Helper()
	for d := 0; d < dirs; d++ {
		dir := filepath.Join(root, fmt.Sprintf("d%02d", d))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		for f := 0; f < files; f++ {
			if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("F%02d.kt", f)), nil, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func waitReady(t *testing.T, w *fileWatcher) {
	t.Helper()
	select {
	case <-w.Ready():
	case <-time.After(5 * time.Second):
		t.Fatal("watcher Ready() did not fire within 5s")
	}
}

// TestFileWatcher_BudgetCapsDescriptorsAndDropsCoverage reproduces the
// macOS failure: kqueue holds a descriptor per watched file, so an
// uncapped walk of a large checkout exhausted the process limit and the
// daemon then returned zero findings. The walk must stop at the budget
// and stop vouching for the tree.
func TestFileWatcher_BudgetCapsDescriptorsAndDropsCoverage(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, 10, 10) // ~111 descriptors under kqueue
	const budget = 40
	w, err := startFileWatcher(context.Background(), root, pipeline.NewWorkspaceState(root), nil,
		withBackendKind(kindFsnotify), withWatchBudget(budget))
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()
	waitReady(t, w)
	if used := w.fdUsed.Load(); used > budget {
		t.Errorf("watcher charged %d descriptors, budget %d", used, budget)
	}
	if w.CoversTree() {
		t.Error("CoversTree() = true after the walk stopped at the budget")
	}
}

func TestFileWatcher_CoversTreeOnceFullyWatched(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, 3, 3)
	w, err := startFileWatcher(context.Background(), root, pipeline.NewWorkspaceState(root), nil,
		withBackendKind(kindFsnotify), withWatchBudget(1000))
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	defer w.Stop()
	waitReady(t, w)
	if !w.CoversTree() {
		t.Error("CoversTree() = false for a tree that fits the budget")
	}
	if used := w.fdUsed.Load(); used != 1+3+3*(1+3) {
		t.Errorf("fdUsed = %d, want root (1+3 entries) + 3 dirs of (1+3)", used)
	}
}

// TestApplyDirtySet_UncoveredWatcherProvesNothing: a warm call whose
// watcher does not cover the tree must not take the drained dirty set
// as the full list of changes, nor skip the bundle stat sweep. A file
// edited under a directory the watcher never reached reports no event,
// so any of those shortcuts replays its stale findings.
func TestApplyDirtySet_UncoveredWatcherProvesNothing(t *testing.T) {
	root := t.TempDir()
	state := &daemonState{root: root, workspace: pipeline.NewWorkspaceState(root)}
	newInput := func() pipeline.ProjectInput {
		var in pipeline.ProjectInput
		in.Args.Paths = []string{root}
		in.Args.KotlinPaths = []string{filepath.Join(root, "A.kt")}
		in.Host.ResidentFiles = state.workspace
		in.Host.BundleStatsClean = func(string) bool { return true }
		return in
	}
	dirty := []string{filepath.Join(root, "A.kt")}

	for _, tc := range []struct {
		name  string
		dirty []string
	}{{"empty dirty set", nil}, {"edit reported", dirty}} {
		t.Run(tc.name, func(t *testing.T) {
			in := newInput()
			state.applyDirtySet(&in, false, false, tc.dirty)
			if in.Host.SourceSetClean {
				t.Error("SourceSetClean = true")
			}
			if in.Host.SourceSetDirty != nil || in.Host.AnalysisCacheDirty != nil {
				t.Errorf("dirty sets = %#v / %#v, want nil (no opinion)", in.Host.SourceSetDirty, in.Host.AnalysisCacheDirty)
			}
			if in.Host.ResidentFiles != nil {
				t.Error("ResidentFiles still set, want files parsed from disk")
			}
			if in.Host.BundleStatsClean != nil {
				t.Error("BundleStatsClean still set, want the stat sweep to run")
			}
			if in.Args.KotlinPaths != nil {
				t.Error("prepopulated source paths kept, want a walk of the source roots")
			}
		})
	}

	t.Run("covering watcher keeps the edit list", func(t *testing.T) {
		in := newInput()
		state.applyDirtySet(&in, false, true, dirty)
		if len(in.Host.AnalysisCacheDirty) != 1 || in.Host.ResidentFiles == nil || in.Host.BundleStatsClean == nil {
			t.Errorf("covering watcher lost its shortcuts: %+v", in.Host)
		}
	})
}
