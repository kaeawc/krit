package pipeline

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/perf"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// The one-shot CLI spells scanned files absolutely; the daemon enters the
// caller's directory and keeps a relative scan path ("."), so its files are
// relative. Both share .krit's findings bundle. A replay that merged one
// spelling's new rows over the other's prior rows kept the edited file's old
// rows beside the new ones: duplicates that persisted (#1832).
func TestReplayAcrossPathSpellingsReplacesEditedFile(t *testing.T) {
	t.Setenv("KRIT_AFFECTED_SET_REPLAY", "on")
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
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

	classRule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)
	run := func(t *testing.T, paths []string, cacheRoot, bundles string) (ProjectResult, string) {
		t.Helper()
		tracker := perf.New(true)
		cachePath := filepath.Join(cacheRoot, ".krit-test", "krit.cache")
		if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
			t.Fatal(err)
		}
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       paths,
				ActiveRules: []*api.Rule{crossFileHelperRule(), classRule},
				Format:      "json",
				Version:     "test",
			},
			Host: ProjectHostState{
				Tracker:                 tracker,
				AnalysisCache:           cache.Load(cachePath),
				AnalysisCacheFilePath:   cachePath,
				AnalysisCacheLookup:     true,
				CrossFileCacheDir:       scanner.CrossFileCacheDir(cacheRoot),
				FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
				FindingsBundleCacheRoot: bundles,
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		reason := ""
		if entry, ok := findTiming(tracker.GetTimings(), "dispatchAffectedSetPath"); ok {
			reason = entry.Attributes["reason"]
		}
		return res, reason
	}
	keys := func(res ProjectResult) []string {
		var out []string
		for _, f := range res.FinalFindings.Findings() {
			file := f.File
			if !filepath.IsAbs(file) {
				file = filepath.Join(dir, file)
			}
			out = append(out, fmt.Sprintf("%s %s:%d %s", f.Rule, file, f.Line, f.Message))
		}
		sort.Strings(out)
		return out
	}

	// CLI history: absolute scan paths, cold then an ABI edit.
	run(t, []string{dir}, dir, bundleRoot)
	write("A.kt", "package p\n\nclass Helper\n\nclass Added1\n")
	run(t, []string{dir}, dir, bundleRoot)

	// The daemon: the caller's directory and a relative scan path, then
	// another ABI edit to the same file.
	t.Chdir(dir)
	run(t, []string{"."}, dir, bundleRoot)
	write("A.kt", "package p\n\nclass Helper\n\nclass Added1\n\nclass Added2\n")
	warm, reason := run(t, []string{"."}, dir, bundleRoot)
	if reason != replayHit {
		t.Logf("affected-set replay reason = %q", reason)
	}
	clean, _ := run(t, []string{"."}, t.TempDir(), t.TempDir())
	got, want := keys(warm), keys(clean)
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("warm run after a spelling change:\n  %v\nclean scan:\n  %v", got, want)
	}
}
