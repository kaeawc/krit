package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/kaeawc/krit/internal/cache"
	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// warmCrossHarness runs RunProject repeatedly over one directory with a
// shared on-disk cache and a cross rule that emits one finding per indexed
// file, so results depend on the file set and not just per-file content.
type warmCrossHarness struct {
	t          *testing.T
	dir        string
	rule       *api.Rule
	crossCalls *int64
}

func newWarmCrossHarness(t *testing.T, ruleID string, files map[string]string) *warmCrossHarness {
	t.Helper()
	h := &warmCrossHarness{t: t, dir: t.TempDir(), crossCalls: new(int64)}
	for name, body := range files {
		h.write(name, body)
	}
	calls := h.crossCalls
	h.rule = api.FakeRule(ruleID, api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(ctx *api.Context) {
		atomic.AddInt64(calls, 1)
		for _, f := range ctx.CodeIndex.Files {
			ctx.Emit(scanner.Finding{File: f.Path, Line: 1, Col: 1, Message: "cross"})
		}
	}))
	return h
}

func (h *warmCrossHarness) write(name, body string) {
	h.t.Helper()
	if err := os.WriteFile(filepath.Join(h.dir, name), []byte(body), 0o644); err != nil {
		h.t.Fatal(err)
	}
}

func (h *warmCrossHarness) remove(name string) {
	h.t.Helper()
	if err := os.Remove(filepath.Join(h.dir, name)); err != nil {
		h.t.Fatal(err)
	}
}

func (h *warmCrossHarness) calls() int64 { return atomic.LoadInt64(h.crossCalls) }

func (h *warmCrossHarness) run() ProjectResult {
	h.t.Helper()
	cachePath := filepath.Join(h.dir, ".krit-test", "krit.cache")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		h.t.Fatal(err)
	}
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{h.dir},
			ActiveRules: []*api.Rule{h.rule},
			Format:      "json",
			Version:     "test",
		},
		Host: ProjectHostState{
			AnalysisCache:         cache.Load(cachePath),
			AnalysisCacheFilePath: cachePath,
			AnalysisCacheLookup:   true,
			CrossFileCacheDir:     scanner.CrossFileCacheDir(filepath.Join(h.dir, ".krit-test")),
			CrossFindingsCacheDir: scanner.CrossFindingsCacheDir(filepath.Join(h.dir, ".krit-test")),
		},
	})
	if err != nil {
		h.t.Fatalf("RunProject: %v", err)
	}
	return res
}

func (h *warmCrossHarness) wantFindings(label string, want int) {
	h.t.Helper()
	if got := h.run().FindingsCount; got != want {
		h.t.Fatalf("%s findings=%d, want %d", label, got, want)
	}
}

// A deleted file leaves every remaining file a per-file cache hit, but the
// cross-file index changed, so warm cross findings must be recomputed rather
// than replayed from the previous file set (issue #825).
func TestRunProject_WarmCrossFindingsInvalidatedByDeletedFile(t *testing.T) {
	h := newWarmCrossHarness(t, "CrossRule", map[string]string{
		"A.kt": "package test\n\nclass A\n",
		"B.kt": "package test\n\nclass B\n",
	})
	h.wantFindings("cold", 2)
	h.wantFindings("warm", 2)
	if h.calls() != 1 {
		t.Fatalf("warm run should reuse cross findings; calls=%d want 1", h.calls())
	}
	h.remove("B.kt")
	h.wantFindings("after delete", 1)
	if h.calls() != 2 {
		t.Fatalf("delete must recompute cross findings; calls=%d want 2", h.calls())
	}
}

// Java files are always indexed, even when no active rule needs Java before
// dispatch. They must not defeat the warm hit, and deleting one must
// invalidate it.
func TestRunProject_WarmCrossFindingsJavaFileSet(t *testing.T) {
	h := newWarmCrossHarness(t, "CrossRule", map[string]string{
		"A.kt":   "package test\n\nclass A\n",
		"J.java": "package test;\n\nclass J {}\n",
	})
	h.wantFindings("cold", 2)
	h.wantFindings("warm", 2)
	if h.calls() != 1 {
		t.Fatalf("Kotlin-only cross rule with a .java file must still get a warm hit; calls=%d want 1", h.calls())
	}
	h.remove("J.java")
	h.wantFindings("after java delete", 1)
	if h.calls() != 2 {
		t.Fatalf("java delete must recompute cross findings; calls=%d want 2", h.calls())
	}
}

// The lexically-irrelevant-miss shortcut replays the last cross findings, so
// it needs the same file-set check: delete B.kt and comment-edit A.kt.
func TestRunProject_WarmCrossFindingsLexicalShortcutInvalidatedByDelete(t *testing.T) {
	h := newWarmCrossHarness(t, "OnClick", map[string]string{
		"A.kt": "package test\n\nclass A\n",
		"B.kt": "package test\n\nclass B\n",
	})
	h.wantFindings("cold", 2)
	h.remove("B.kt")
	h.write("A.kt", "package test\n\nclass A\n// comment only\n")
	h.wantFindings("after delete and comment edit", 1)
}
