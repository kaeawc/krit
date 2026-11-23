package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// cliPreparseProject drives RunProjectAnalysis the way the one-shot CLI does
// with PreparseBundleShortcut on: a fresh host per run, no resident state,
// and source paths from the caller's own discovery.
type cliPreparseProject struct {
	t         *testing.T
	dir       string
	cacheRoot string
	rule      *api.Rule
	calls     *int64
	tick      int
}

func newCLIPreparseProject(t *testing.T) *cliPreparseProject {
	t.Helper()
	p := &cliPreparseProject{t: t, dir: t.TempDir(), cacheRoot: t.TempDir(), calls: new(int64)}
	calls := p.calls
	p.rule = api.FakeRule("TodoClass", api.WithNodeTypes("class_declaration"), api.WithCheck(func(ctx *api.Context) {
		atomic.AddInt64(calls, 1)
		ctx.Emit(scanner.Finding{File: ctx.File.Path, Line: ctx.File.FlatRow(ctx.Idx) + 1, Col: 1, Message: "class"})
	}))
	p.write("src/main/kotlin/A.kt", "package p\n\nclass A\n")
	p.write("src/main/kotlin/B.kt", "package p\n\nclass B\n")
	return p
}

func (p *cliPreparseProject) write(rel, body string) {
	p.t.Helper()
	path := filepath.Join(p.dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		p.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		p.t.Fatal(err)
	}
	// Distinct mtimes so a same-size rewrite still moves the recorded stat.
	p.tick++
	mtime := time.Unix(1_700_000_000+int64(p.tick), 0)
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		p.t.Fatal(err)
	}
}

func (p *cliPreparseProject) run(mutate func(*ProjectArgs)) ProjectAnalysisResult {
	p.t.Helper()
	kotlinPaths, err := scanner.CollectKotlinFiles([]string{p.dir}, nil)
	if err != nil {
		p.t.Fatal(err)
	}
	args := ProjectArgs{
		Config:      config.NewConfig(),
		Paths:       []string{p.dir},
		KotlinPaths: kotlinPaths,
		JavaPaths:   []string{},
		ActiveRules: []*api.Rule{p.rule},
		Format:      "json",
		Version:     "test",
	}
	if mutate != nil {
		mutate(&args)
	}
	res, err := RunProjectAnalysis(context.Background(), ProjectInput{
		Args: args,
		Host: ProjectHostState{
			FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
			FindingsBundleCacheRoot: p.cacheRoot,
			PreparseBundleShortcut:  true,
		},
	})
	if err != nil {
		p.t.Fatalf("RunProjectAnalysis: %v", err)
	}
	return res
}

func preparsed(res ProjectAnalysisResult) bool {
	if !res.FindingsBundleHit || len(res.ParseResult.KotlinFiles) == 0 {
		return false
	}
	for _, f := range res.ParseResult.KotlinFiles {
		if f.FlatTree != nil {
			return false
		}
	}
	return true
}

func TestRunProjectAnalysis_PreparseBundleShortcut(t *testing.T) {
	p := newCLIPreparseProject(t)

	cold := p.run(nil)
	if cold.FindingsBundleHit || cold.CrossFileResult.Findings.Len() != 2 {
		t.Fatalf("cold: hit=%v findings=%d, want a miss with 2 findings", cold.FindingsBundleHit, cold.CrossFileResult.Findings.Len())
	}

	before := atomic.LoadInt64(p.calls)
	warm := p.run(nil)
	if !preparsed(warm) {
		t.Fatalf("unchanged tree must be served before parsing (hit=%v)", warm.FindingsBundleHit)
	}
	if atomic.LoadInt64(p.calls) != before {
		t.Fatal("pre-parse hit must not dispatch rules")
	}
	if warm.CrossFileResult.Findings.Len() != 2 || warm.FilesScanned != 2 {
		t.Fatalf("pre-parse hit: findings=%d files=%d, want 2 and 2", warm.CrossFileResult.Findings.Len(), warm.FilesScanned)
	}

	// A content edit moves the file's stat: no pre-parse hit, fresh findings.
	p.write("src/main/kotlin/B.kt", "package p\n\nclass B\nclass C\n")
	edited := p.run(nil)
	if preparsed(edited) {
		t.Fatal("an edited file must not be served from the pre-parse bundle")
	}
	if edited.CrossFileResult.Findings.Len() != 3 {
		t.Fatalf("after edit: findings=%d, want 3", edited.CrossFileResult.Findings.Len())
	}
	if again := p.run(nil); !preparsed(again) || again.CrossFileResult.Findings.Len() != 3 {
		t.Fatalf("rerun after edit: pre-parse hit=%v findings=%d, want a hit with 3", preparsed(again), again.CrossFileResult.Findings.Len())
	}

	// A new file changes the source set even though every recorded stat matches.
	p.write("src/main/kotlin/D.kt", "package p\n\nclass D\n")
	added := p.run(nil)
	if preparsed(added) || added.CrossFileResult.Findings.Len() != 4 {
		t.Fatalf("after add: pre-parse hit=%v findings=%d, want a miss with 4", preparsed(added), added.CrossFileResult.Findings.Len())
	}

	// A deleted file likewise.
	if err := os.Remove(filepath.Join(p.dir, "src/main/kotlin/A.kt")); err != nil {
		t.Fatal(err)
	}
	removed := p.run(nil)
	if preparsed(removed) || removed.CrossFileResult.Findings.Len() != 3 {
		t.Fatalf("after delete: pre-parse hit=%v findings=%d, want a miss with 3", preparsed(removed), removed.CrossFileResult.Findings.Len())
	}

	// Without caller-discovered paths the shortcut stays off.
	if res := p.run(func(a *ProjectArgs) { a.KotlinPaths = nil }); preparsed(res) {
		t.Fatal("pre-parse shortcut needs the caller's source paths")
	}
	// Fix runs never take it.
	if res := p.run(func(a *ProjectArgs) { a.Fix = true }); preparsed(res) {
		t.Fatal("fix runs must not be served from the pre-parse bundle")
	}
}
