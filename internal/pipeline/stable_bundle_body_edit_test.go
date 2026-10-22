package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Regression tests for #823: a warm scan after a single body-only edit
// used to replay the entire prior bundle, including the edited file's own
// findings, so findings added by the edit were dropped.

func bodyEditRule() *api.Rule {
	return api.FakeRule("EmptyIfLike",
		api.WithNodeTypes("if_expression"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "if in body")
		}),
	)
}

func writeBodyEditProject(t *testing.T, dir string, use string) (usePath string) {
	t.Helper()
	files := map[string]string{
		"Lib.kt": "package demo\n\nfun fresh(): Int = 1\n",
		"Alt.kt": "package demo\n\nfun alt(): Int = fresh()\n",
		"Use.kt": use,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return filepath.Join(dir, "Use.kt")
}

const (
	useBefore = "package demo\n\nfun useFresh(): Int {\n    return fresh()\n}\n"
	useAfter  = "package demo\n\nfun useFresh(): Int {\n    if (true) { return 1 }\n    return fresh()\n}\n"
)

func bodyEditFindings(t *testing.T, dir, cacheRoot string, host ProjectHostState) (int, ProjectResult) {
	t.Helper()
	host.FindingsBundleStore = scanner.DiskFindingsBundleStore{}
	host.FindingsBundleCacheRoot = cacheRoot
	if cacheRoot == "" {
		host.FindingsBundleStore = nil
	}
	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{bodyEditRule()},
			Format:      "json",
			Version:     "test",
		},
		Host: host,
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	return res.FindingsCount, res
}

func TestRunProject_BodyOnlyEdit_WarmMatchesClean_OneShot(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	usePath := writeBodyEditProject(t, dir, useBefore)

	if n, _ := bodyEditFindings(t, dir, cacheRoot, ProjectHostState{}); n != 0 {
		t.Fatalf("first run findings = %d, want 0", n)
	}
	if n, _ := bodyEditFindings(t, dir, cacheRoot, ProjectHostState{}); n != 0 {
		t.Fatalf("second run findings = %d, want 0", n)
	}

	// Precondition: the edit is invisible to the structural fingerprint,
	// which is what used to trigger the whole-bundle replay.
	before, err := scanner.ParseFile(context.Background(), usePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(usePath, []byte(useAfter), 0o644); err != nil {
		t.Fatal(err)
	}
	after, err := scanner.ParseFile(context.Background(), usePath)
	if err != nil {
		t.Fatal(err)
	}
	if scanner.FileStructuralFingerprint(before) != scanner.FileStructuralFingerprint(after) {
		t.Fatalf("test premise broken: edit moved the structural fingerprint")
	}

	warm, _ := bodyEditFindings(t, dir, cacheRoot, ProjectHostState{})
	clean, _ := bodyEditFindings(t, dir, "", ProjectHostState{})
	if clean != 1 {
		t.Fatalf("clean findings = %d, want 1", clean)
	}
	if warm != clean {
		t.Errorf("warm findings = %d, clean = %d: body edit finding dropped by bundle replay", warm, clean)
	}

	// And the finding must disappear again when the edit is reverted.
	writeBodyEditProject(t, dir, useBefore)
	if n, _ := bodyEditFindings(t, dir, cacheRoot, ProjectHostState{}); n != 0 {
		t.Errorf("after revert warm findings = %d, want 0", n)
	}
}

// TestRunProject_BodyOnlyEdit_WarmMatchesClean_Daemon mirrors the daemon
// wiring: prior manifest facts are handed to the host (PriorContentHashes
// and friends, which enables previewPostParseBundleHit) and bundles are
// kept resident in memory.
func TestRunProject_BodyOnlyEdit_WarmMatchesClean_Daemon(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	usePath := writeBodyEditProject(t, dir, useBefore)

	var mu sync.Mutex
	resident := map[string]*scanner.FindingColumns{}
	daemonHost := func() ProjectHostState {
		h := ProjectHostState{}
		manifestKey := scanner.FindingsBundleManifestKey(cacheRoot, []string{dir})
		if m, ok := scanner.LoadFindingsBundleManifest(cacheRoot, manifestKey); ok {
			h.PriorContentHashes = m.ContentHashes
			h.PriorStructuralFPs = m.StructuralFPs
			h.PriorOracleBlobHashes = m.OracleBlobHashes
			h.PriorFileStats = m.FileStats
		}
		h.SourceSetDirty = []string{usePath}
		h.ResidentBundle = func(key string) *scanner.FindingColumns {
			mu.Lock()
			defer mu.Unlock()
			return resident[key]
		}
		h.StoreResidentBundle = func(key string, cols *scanner.FindingColumns) {
			mu.Lock()
			defer mu.Unlock()
			resident[key] = cols
		}
		return h
	}

	for i := 0; i < 2; i++ {
		if n, _ := bodyEditFindings(t, dir, cacheRoot, daemonHost()); n != 0 {
			t.Fatalf("run %d findings = %d, want 0", i, n)
		}
	}
	if err := os.WriteFile(usePath, []byte(useAfter), 0o644); err != nil {
		t.Fatal(err)
	}
	warm, _ := bodyEditFindings(t, dir, cacheRoot, daemonHost())
	if warm != 1 {
		t.Errorf("daemon warm findings = %d, want 1: body edit finding dropped by bundle replay", warm)
	}
}

// Regression tests for #839: a public function/field type change used to be
// invisible to the structural fingerprint, so the delta planner treated it as
// a body-only edit and kept the prior bundle for dependents.

func structuralPlan(t *testing.T, dir, edited, before, after string) scanner.DeltaPlan {
	t.Helper()
	path := filepath.Join(dir, edited)
	fingerprint := func(src string) scanner.RunFingerprint {
		if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
		var kt, java []*scanner.File
		if filepath.Ext(path) == ".java" {
			f, err := scanner.ParseJavaFile(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			java = append(java, f)
		} else {
			f, err := scanner.ParseFile(context.Background(), path)
			if err != nil {
				t.Fatal(err)
			}
			kt = append(kt, f)
		}
		return scanner.RunFingerprint{CrossFile: crossFileStructuralFingerprint(kt, java, nil, nil)}
	}
	prev := fingerprint(before)
	cur := fingerprint(after)
	return scanner.ConservativeDeltaPlanner{}.Plan(prev, cur, []string{path})
}

func TestDeltaPlanner_TypedAPIChangeIsNotBodyEdit(t *testing.T) {
	cases := []struct {
		name, file, before, after string
		reuse                     bool
	}{
		{"kotlin param type", "Lib.kt", "package demo\nfun f(x: Int): String = \"\"\n", "package demo\nfun f(x: Long): String = \"\"\n", false},
		{"kotlin return nullability", "Lib.kt", "package demo\nfun f(x: Int): String = \"\"\n", "package demo\nfun f(x: Int): String? = null\n", false},
		{"java field type", "Lib.java", "package demo;\npublic class Lib { public String name; }\n", "package demo;\npublic class Lib { public Integer name; }\n", false},
		{"java method return", "Lib.java", "package demo;\npublic class Lib { public String f() { return null; } }\n", "package demo;\npublic class Lib { public Integer f() { return null; } }\n", false},
		{"kotlin body only", "Lib.kt", "package demo\nfun f(x: Int): String = \"a\"\n", "package demo\nfun f(x: Int): String = \"b\"\n", true},
		{"java body only", "Lib.java", "package demo;\npublic class Lib { public String f() { return null; } }\n", "package demo;\npublic class Lib { public String f() { return \"s\"; } }\n", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			plan := structuralPlan(t, t.TempDir(), tc.file, tc.before, tc.after)
			if plan.ReusePrevious != tc.reuse {
				t.Fatalf("ReusePrevious = %v, want %v", plan.ReusePrevious, tc.reuse)
			}
		})
	}
}
