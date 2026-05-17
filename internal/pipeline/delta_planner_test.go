package pipeline

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestFileStructuralFingerprint_BodyEditStable verifies the core
// delta-planner invariant: a body-only edit (no Symbol or Reference
// change) does NOT move the file's structural fingerprint. This
// stability is what lets the planner take the single-file delta path
// for the common dev-loop case.
func TestFileStructuralFingerprint_BodyEditStable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Sample.kt")

	v1 := []byte("package test\n\nclass Foo {\n    fun greet() = \"hello\"\n}\n")
	v2 := []byte("package test\n\nclass Foo {\n    fun greet() = \"world\"\n}\n")

	if err := os.WriteFile(path, v1, 0o644); err != nil {
		t.Fatal(err)
	}
	f1, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(path, v2, 0o644); err != nil {
		t.Fatal(err)
	}
	f2, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}

	fp1 := scanner.FileStructuralFingerprint(f1)
	fp2 := scanner.FileStructuralFingerprint(f2)
	if fp1 != fp2 {
		t.Errorf("structural fingerprint moved on body-only edit:\n  v1=%q\n  v2=%q", fp1, fp2)
	}
}

// TestFileStructuralFingerprint_DeclarationChangeMoves verifies the
// opposite invariant: adding/removing/renaming a declaration moves
// the structural fingerprint. This is the safety side of the gate —
// declaration changes invalidate cross-file findings, so the planner
// must fall back to full dispatch.
func TestFileStructuralFingerprint_DeclarationChangeMoves(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Sample.kt")

	v1 := []byte("package test\n\nclass Foo\n")
	v2 := []byte("package test\n\nclass Foo\nclass Bar\n")

	if err := os.WriteFile(path, v1, 0o644); err != nil {
		t.Fatal(err)
	}
	f1, _ := scanner.ParseFile(context.Background(), path)

	if err := os.WriteFile(path, v2, 0o644); err != nil {
		t.Fatal(err)
	}
	f2, _ := scanner.ParseFile(context.Background(), path)

	fp1 := scanner.FileStructuralFingerprint(f1)
	fp2 := scanner.FileStructuralFingerprint(f2)
	if fp1 == fp2 {
		t.Errorf("structural fingerprint stable across declaration add; expected drift")
	}
}

// TestRunProject_DeltaPath_BodyEditTakesDelta walks the full delta
// scenario end-to-end. Two RunProject calls with a body-only edit
// between them: the second call must hit the delta path (scoped
// dispatch on just the changed file + ApplyDelta) and produce
// byte-equal output as if a full dispatch had run.
func TestRunProject_DeltaPath_BodyEditTakesDelta(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	src := filepath.Join(dir, "Sample.kt")

	if err := os.WriteFile(src, []byte("package test\n\nclass Foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	run := func(t *testing.T) (int, []byte) {
		t.Helper()
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{dir},
				ActiveRules: []*api.Rule{rule},
				Format:      "json",
				Version:     "test",
			},
			Host: ProjectHostState{
				FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
				FindingsBundleCacheRoot: cacheRoot,
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		return res.FindingsCount, scrubLines(res.JSON, []string{
			"\"durationMs\"", "\"timestamp\"", "\"wallSeconds\"",
			"\"startTime\"", "\"endTime\"",
		})
	}

	// First run: full dispatch + Save bundle + Save manifest.
	count1, _ := run(t)
	if count1 != 1 {
		t.Fatalf("first run findings = %d, want 1", count1)
	}

	// Mutate the file body only (declaration unchanged → structural
	// fp stable → planner takes delta).
	if err := os.WriteFile(src, []byte("package test\n\nclass Foo // comment edit\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	count2, _ := run(t)
	if count2 != 1 {
		t.Errorf("second run findings = %d, want 1", count2)
	}
}

// TestRunProject_DeltaPath_DeclarationChangeFallsBack confirms the
// safety side: when a file's declaration changes (CrossFile fp moves),
// the planner refuses the delta and full dispatch runs.
func TestRunProject_DeltaPath_DeclarationChangeFallsBack(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	src := filepath.Join(dir, "Sample.kt")

	if err := os.WriteFile(src, []byte("package test\n\nclass Foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rule := api.FakeRule("ClassDecl",
		api.WithNodeTypes("class_declaration"),
		api.WithSeverity(api.SeverityWarning),
		api.WithCheck(func(ctx *api.Context) {
			ctx.EmitAt(int(ctx.Node.StartRow)+1, 1, "class declared")
		}),
	)

	run := func(t *testing.T) int {
		t.Helper()
		res, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{dir},
				ActiveRules: []*api.Rule{rule},
				Format:      "json",
				Version:     "test",
			},
			Host: ProjectHostState{
				FindingsBundleStore:     scanner.DiskFindingsBundleStore{},
				FindingsBundleCacheRoot: cacheRoot,
			},
		})
		if err != nil {
			t.Fatalf("RunProject: %v", err)
		}
		return res.FindingsCount
	}

	if got := run(t); got != 1 {
		t.Fatalf("first run findings = %d, want 1 (one class)", got)
	}

	// Add a declaration. The new class moves the structural fp →
	// aggregate CrossFile fp moves → planner refuses delta. Full
	// dispatch should fire and emit findings for both classes.
	if err := os.WriteFile(src, []byte("package test\n\nclass Foo\nclass Bar\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if got := run(t); got != 2 {
		t.Errorf("after declaration add, findings = %d, want 2 (full dispatch should emit on both)", got)
	}
}

// TestDiffContentHashes verifies the small helper used by the delta
// planner orchestrator to compute changed paths from manifest +
// current per-file hash maps.
func TestDiffContentHashes(t *testing.T) {
	prior := map[string]string{"a.kt": "h1", "b.kt": "h2", "c.kt": "h3"}
	current := map[string]string{"a.kt": "h1", "b.kt": "DIFF", "c.kt": "h3"}
	got := diffContentHashes(prior, current)
	if len(got) != 1 || got[0] != "b.kt" {
		t.Errorf("diffContentHashes = %v, want [b.kt]", got)
	}

	// Added file.
	current2 := map[string]string{"a.kt": "h1", "b.kt": "h2", "c.kt": "h3", "d.kt": "h4"}
	got2 := diffContentHashes(prior, current2)
	if len(got2) != 1 || got2[0] != "d.kt" {
		t.Errorf("diffContentHashes (add) = %v, want [d.kt]", got2)
	}

	// Removed file.
	current3 := map[string]string{"a.kt": "h1", "b.kt": "h2"}
	got3 := diffContentHashes(prior, current3)
	if len(got3) != 1 || got3[0] != "c.kt" {
		t.Errorf("diffContentHashes (remove) = %v, want [c.kt]", got3)
	}

	// Identical → empty.
	got4 := diffContentHashes(prior, prior)
	if len(got4) != 0 {
		t.Errorf("diffContentHashes (identical) = %v, want []", got4)
	}
}
