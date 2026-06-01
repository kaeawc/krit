package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// crossFileHelperRule emits one finding per file that references the symbol
// "Helper", but only while "Helper" is still declared somewhere in the project
// index. So an edit that removes the Helper declaration flips the finding on
// its *referrers* — files that were not themselves edited. That is exactly the
// cross-file dependency the affected-set replay must regenerate.
//
// It declares NeedsCrossFile only (not NeedsParsedFiles, which would flip the
// cross-file classification and leave CodeIndex nil). Cross-file rules are
// invoked once with the project CodeIndex and ctx.File nil, so it iterates the
// referrers itself and emits findings with an explicit File path.
func crossFileHelperRule() *api.Rule {
	return api.FakeRule("HelperRef",
		api.WithNeeds(api.NeedsCrossFile),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			if ctx.CodeIndex == nil {
				return
			}
			if len(ctx.CodeIndex.SymbolsNamed("Helper")) == 0 {
				return // Helper no longer declared anywhere.
			}
			for path := range ctx.CodeIndex.ReferenceFiles("Helper") {
				ctx.Emit(scanner.Finding{File: path, Line: 1, Col: 1, Message: "references live Helper"})
			}
		}),
	)
}

// TestRunProject_AffectedSetReplay_RegeneratesDependent is the end-to-end #608
// regression for the warm+ABI affected-set replay path. It drives RunProject
// twice against shared cross-file + findings-bundle caches:
//
//   - Run 1 (cold): B.kt references Helper (declared in A.kt) and emits a
//     finding. The cross-file cache and findings bundle are persisted.
//   - Run 2 (warm, replay ON): A.kt's Helper is renamed (an ABI change). The
//     incremental overlay rebuild populates the removed-edge data, the replay
//     path fires, and B.kt — a dependent that was NOT edited — must be
//     re-dispatched so its now-stale finding is dropped.
//
// The test asserts both that the replay path actually fired (perf reason ==
// "hit") and that the resulting findings match the correct post-edit answer
// (zero — Helper is gone), which a stale-row replay would fail.
func TestRunProject_AffectedSetReplay_RegeneratesDependent(t *testing.T) {
	t.Setenv("KRIT_AFFECTED_SET_REPLAY", "on")
	dir := t.TempDir()
	bundleRoot := t.TempDir()

	// 5 files keeps the affected set ({A,B}) under the replay ratio gate.
	write := func(name, content string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	a := write("A.kt", "package p\n\nclass Helper\n")
	write("B.kt", "package p\n\nclass Client {\n  fun make(): Helper = Helper()\n}\n")
	write("C.kt", "package p\n\nclass Unrelated1\n")
	write("D.kt", "package p\n\nclass Unrelated2\n")
	write("E.kt", "package p\n\nclass Unrelated3\n")

	run := func(t *testing.T, tracker perf.Tracker) ProjectResult {
		t.Helper()
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{dir},
				ActiveRules: []*api.Rule{crossFileHelperRule()},
				Format:      "json",
				Version:     "test",
			},
			Host: ProjectHostState{
				Tracker:                 tracker,
				CrossFileCacheDir:       scanner.CrossFileCacheDir(dir),
				FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
				FindingsBundleCacheRoot: bundleRoot,
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		return res
	}

	// Run 1 (cold): a live Helper -> at least one referrer finding.
	if got := run(t, nil).FindingsCount; got == 0 {
		t.Fatalf("cold run findings = 0, want >0 (referrers of live Helper)")
	}

	// ABI edit: rename Helper so B's reference dangles. B is NOT re-edited.
	if err := os.WriteFile(a, []byte("package p\n\nclass Helper2\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tracker := perf.New(true)
	res2 := run(t, tracker)

	// The replay path must have fired (not silently fallen back to full
	// dispatch). The bail label, if any, pinpoints which gate blocked it.
	entry, found := findTiming(tracker.GetTimings(), "dispatchAffectedSetPath")
	if !found {
		t.Fatalf("dispatchAffectedSetPath perf entry missing; replay path was not reached")
	}
	if reason := entry.Attributes["reason"]; reason != "hit" {
		t.Fatalf("affected-set replay did not fire: reason = %q, want \"hit\"", reason)
	}

	// Correctness: Helper is gone, so B's finding must be dropped. A replay
	// that failed to re-dispatch the dependent B would leave the stale finding.
	if res2.FindingsCount != 0 {
		t.Errorf("warm replay findings = %d, want 0 (Helper renamed; B's finding must be regenerated away)", res2.FindingsCount)
	}
}
