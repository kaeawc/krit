package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

// A NeedsParsedFiles rule is promised every parsed file, so its verdict on an
// affected file can rest on a file two reference hops away, outside the
// affected set (#1817). Use.kt calls mid() in Mid.kt, which calls leaf() in
// Leaf.kt; the rule reports Use.kt only while some parsed file carries the
// marker, and only Leaf.kt does. An ABI edit to Use.kt makes {Use, Mid} the
// affected set, and the replay must still match a clean scan.
func TestRunProject_AffectedSetReplay_ParsedFilesRuleSeesWholeProject(t *testing.T) {
	t.Setenv("KRIT_AFFECTED_SET_REPLAY", "on")
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("Use.kt", "package p\n\nfun useMid(): Int = mid()\n")
	write("Mid.kt", "package p\n\nfun mid(): Int = leaf()\n")
	write("Leaf.kt", "package p\n\n// MARKER\nfun leaf(): Int = 1\n")
	write("C.kt", "package p\n\nclass Unrelated1\n")
	write("D.kt", "package p\n\nclass Unrelated2\n")
	write("E.kt", "package p\n\nclass Unrelated3\n")
	write("F.kt", "package p\n\nclass Unrelated4\n")

	markerRule := api.FakeRule("MarkedUse",
		api.WithNeeds(api.NeedsParsedFiles),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			marked := false
			for _, f := range ctx.ParsedFiles {
				if strings.Contains(string(f.Content), "MARKER") {
					marked = true
				}
			}
			if !marked {
				return
			}
			for _, f := range ctx.ParsedFiles {
				if strings.Contains(string(f.Content), "useMid") {
					ctx.Emit(scanner.Finding{File: f.Path, Line: 3, Col: 1, Message: "uses a marked chain"})
				}
			}
		}),
	)
	run := func(t *testing.T, tracker perf.Tracker, cacheRoot, bundleRoot string) ProjectResult {
		t.Helper()
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{dir},
				ActiveRules: []*api.Rule{crossFileHelperRule(), markerRule},
				Format:      "json",
				Version:     "test",
			},
			Host: ProjectHostState{
				Tracker:                 tracker,
				CrossFileCacheDir:       scanner.CrossFileCacheDir(cacheRoot),
				CrossFindingsCacheDir:   scanner.CrossFindingsCacheDir(cacheRoot),
				FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
				FindingsBundleCacheRoot: bundleRoot,
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		return res
	}

	bundleRoot := t.TempDir()
	if got := run(t, nil, dir, bundleRoot).FindingsCount; got != 1 {
		t.Fatalf("cold run findings = %d, want 1", got)
	}
	// ABI edit to Use.kt only; Leaf.kt is two hops away.
	write("Use.kt", "package p\n\nfun useMid(): Int = mid()\n\nfun added(): Int = 2\n")
	tracker := perf.New(true)
	warm := run(t, tracker, dir, bundleRoot)
	if entry, found := findTiming(tracker.GetTimings(), "dispatchAffectedSetPath"); !found || entry.Attributes["reason"] != "hit" {
		t.Fatalf("affected-set replay did not fire: %+v", entry.Attributes)
	}
	clean := run(t, nil, t.TempDir(), t.TempDir())
	if warm.FindingsCount != clean.FindingsCount {
		t.Fatalf("warm replay findings = %d, clean scan = %d", warm.FindingsCount, clean.FindingsCount)
	}
	// The cross-findings cache keys on the index fingerprint alone, so a
	// replay that ran these rules on the affected subset stored its subset
	// result under the full project's key, and every later run that read the
	// cache served it.
	if again := run(t, nil, dir, t.TempDir()); again.FindingsCount != clean.FindingsCount {
		t.Fatalf("run over the replay's cross-findings cache = %d findings, clean scan = %d", again.FindingsCount, clean.FindingsCount)
	}
}

// A rule panic during the replay leaves its regenerated rows missing while the
// prior ones are dropped, so the replay must fall back to full dispatch rather
// than serve (and cache) the loss.
func TestRunProject_AffectedSetReplay_RulePanicFallsBack(t *testing.T) {
	t.Setenv("KRIT_AFFECTED_SET_REPLAY", "on")
	dir := t.TempDir()
	bundleRoot := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("A.kt", "package p\n\nclass Helper\n")
	write("B.kt", "package p\n\nclass Client {\n  fun make(): Helper = Helper()\n}\n")
	write("C.kt", "package p\n\nclass Unrelated1\n")
	write("D.kt", "package p\n\nclass Unrelated2\n")
	write("E.kt", "package p\n\nclass Unrelated3\n")

	panicRule := api.FakeRule("PanicsAfterEdit",
		api.WithNeeds(api.NeedsParsedFiles),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			for _, f := range ctx.ParsedFiles {
				if strings.Contains(string(f.Content), "added") {
					panic("rule bug")
				}
			}
		}),
	)
	run := func(t *testing.T, tracker perf.Tracker) {
		t.Helper()
		if _, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{dir},
				ActiveRules: []*api.Rule{crossFileHelperRule(), panicRule},
				Format:      "json",
				Version:     "test",
			},
			Host: ProjectHostState{
				Tracker:                 tracker,
				CrossFileCacheDir:       scanner.CrossFileCacheDir(dir),
				FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
				FindingsBundleCacheRoot: bundleRoot,
			},
		}); err != nil {
			t.Fatalf("RunProject: %v", err)
		}
	}

	run(t, nil)
	write("A.kt", "package p\n\nclass Helper\n\nfun added(): Int = 1\n")
	tracker := perf.New(true)
	run(t, tracker)
	if entry, found := findTiming(tracker.GetTimings(), "dispatchAffectedSetPath"); !found || entry.Attributes["reason"] != replayBailRuleError {
		t.Fatalf("affected-set replay reason = %+v, want %q", entry.Attributes, replayBailRuleError)
	}
}

// The warm+ABI replay must keep every finding when the scan root is relative
// (krit scanned as `krit .`). Found by the benchmark harness (#787): after a
// public-API edit, the replay dropped the prior rows of the edited file and
// its dependents but regenerated none of them, because the affected set and
// the parsed files spelled the same paths differently.
func TestRunProject_AffectedSetReplay_RelativeScanRootKeepsFindings(t *testing.T) {
	t.Setenv("KRIT_AFFECTED_SET_REPLAY", "on")
	dir := t.TempDir()
	bundleRoot := t.TempDir()
	t.Chdir(dir)
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("A.kt", "package p\n\nclass Helper\n")
	write("B.kt", "package p\n\nclass Client {\n  fun make(): Helper = Helper()\n}\n")
	write("C.kt", "package p\n\nclass Unrelated1\n")
	write("D.kt", "package p\n\nclass Unrelated2\n")
	write("E.kt", "package p\n\nclass Unrelated3\n")

	classRule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)
	run := func(t *testing.T, tracker perf.Tracker, cacheRoot string) ProjectResult {
		t.Helper()
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{"."},
				ActiveRules: []*api.Rule{crossFileHelperRule(), classRule},
				Format:      "json",
				Version:     "test",
			},
			Host: ProjectHostState{
				Tracker:                 tracker,
				CrossFileCacheDir:       scanner.CrossFileCacheDir(cacheRoot),
				FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
				FindingsBundleCacheRoot: bundleRoot,
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		return res
	}

	run(t, nil, dir)
	// ABI edit: a new public declaration in A.kt.
	write("A.kt", "package p\n\nclass Helper\n\nfun added(): Int = 1\n")
	tracker := perf.New(true)
	warm := run(t, tracker, dir)
	if entry, found := findTiming(tracker.GetTimings(), "dispatchAffectedSetPath"); !found || entry.Attributes["reason"] != "hit" {
		t.Fatalf("affected-set replay did not fire: %+v", entry.Attributes)
	}

	bundleRoot = t.TempDir()
	clean := run(t, nil, t.TempDir())
	if warm.FindingsCount != clean.FindingsCount {
		t.Fatalf("warm replay findings = %d, clean scan = %d", warm.FindingsCount, clean.FindingsCount)
	}
}
