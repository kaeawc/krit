package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// The daemon runs `krit --fir` through ProjectHostState.FindingsPostPass.
// Its result must reach the output on every call, including warm calls
// whose findings come from the bundle cache.
func TestRunProject_FindingsPostPassRewritesOutputOnEveryRun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Sample.kt"), []byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)
	bundle := &recordingBundleStore{}
	cacheRoot := t.TempDir()
	var sawPaths [][]string
	post := func(parsed ParseResult, findings []scanner.Finding) []scanner.Finding {
		sawPaths = append(sawPaths, parsed.KotlinPaths)
		out := make([]scanner.Finding, 0, len(findings))
		for _, f := range findings {
			f.Message = "post-pass verdict"
			out = append(out, f)
		}
		return out
	}
	for run := 1; run <= 2; run++ {
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{dir},
				ActiveRules: []*api.Rule{rule},
				Format:      "json",
				JSONCompact: true,
				Version:     "test",
			},
			Host: ProjectHostState{
				FindingsBundleStore:     bundle,
				FindingsBundleCacheRoot: cacheRoot,
				FindingsPostPass:        post,
			},
		})
		if err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if !strings.Contains(string(res.JSON), "post-pass verdict") || strings.Contains(string(res.JSON), "class declared") {
			t.Fatalf("run %d output does not reflect the post pass:\n%s", run, res.JSON)
		}
	}
	if len(sawPaths) != 2 || len(sawPaths[1]) != 1 {
		t.Fatalf("post pass must run on every call with the collected Kotlin paths; saw %v", sawPaths)
	}
}

func TestBundleOutputShortcutsOffWithFindingsPostPass(t *testing.T) {
	host := ProjectHostState{
		FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
		FindingsBundleCacheRoot: t.TempDir(),
		FindingsPostPass:        func(_ ParseResult, f []scanner.Finding) []scanner.Finding { return f },
	}
	if canUsePostParseBundleOutputShortcut(ProjectArgs{Format: "json", JSONCompact: true}, host) {
		t.Fatal("post-parse bundle output shortcut must be off when a post pass is installed")
	}
	if _, ok, err := tryLoadFindingsBundleBeforeParse(context.Background(), time.Now(), "json", ProjectArgs{Format: "json"}, host, nil, nil); ok || err != nil {
		t.Fatalf("pre-parse bundle shortcut must be off when a post pass is installed (ok=%v err=%v)", ok, err)
	}
}
