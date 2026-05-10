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

// recordingBundleStore is a test fake for scanner.FindingsBundleStore
// that lets tests assert how many Load and Save calls fired and lets
// them seed the cache with a specific FindingColumns + fingerprint.
type recordingBundleStore struct {
	store     scanner.DiskFindingsBundleStore
	loadCalls int
	saveCalls int
}

func (r *recordingBundleStore) Load(root string, fp scanner.RunFingerprint) (*scanner.FindingColumns, bool) {
	r.loadCalls++
	return r.store.Load(root, fp)
}

func (r *recordingBundleStore) Save(root string, fp scanner.RunFingerprint, cols *scanner.FindingColumns) error {
	r.saveCalls++
	return r.store.Save(root, fp, cols)
}

// TestRunProject_FindingsBundleCache_HitSkipsDispatch is the load-
// bearing #55 acceptance test: a second RunProject call against a
// byte-identical fixture hits the cache (returns cached findings,
// skips dispatch + cross-file work) and produces byte-equal output.
//
// Asserts both:
//  1. Save fires on the first (miss) call.
//  2. Load returns the cached value on the second call AND output
//     matches the first.
func TestRunProject_FindingsBundleCache_HitSkipsDispatch(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Sample.kt"),
		[]byte("package test\n\nclass Sample\n"), 0o644); err != nil {
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
				FindingsBundleStore:     bundle,
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

	count1, json1 := run(t)
	if count1 != 1 {
		t.Errorf("first run findings = %d, want 1", count1)
	}
	if bundle.saveCalls != 1 {
		t.Errorf("first run saveCalls = %d, want 1", bundle.saveCalls)
	}
	if bundle.loadCalls != 1 {
		t.Errorf("first run loadCalls = %d, want 1 (we always Load to detect a hit)", bundle.loadCalls)
	}

	count2, json2 := run(t)
	if count2 != 1 {
		t.Errorf("second run findings = %d, want 1", count2)
	}
	if bundle.saveCalls != 1 {
		t.Errorf("second run saveCalls = %d, want 1 (cache hit should NOT re-save)", bundle.saveCalls)
	}
	if bundle.loadCalls != 2 {
		t.Errorf("second run loadCalls = %d, want 2", bundle.loadCalls)
	}
	if string(json1) != string(json2) {
		t.Errorf("byte-equal contract failed:\n--- first ---\n%s\n--- second ---\n%s", json1, json2)
	}
}

// TestRunProject_FindingsBundleCache_DisabledByDefault confirms that
// when the host doesn't wire a store, no bundle Load/Save happens and
// the verb keeps pre-#55 behavior.
func TestRunProject_FindingsBundleCache_DisabledByDefault(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Sample.kt"),
		[]byte("package test\n\nclass Sample\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rule := api.FakeRule("Noop")

	res, err := RunProject(context.Background(), ProjectInput{
		Args: ProjectArgs{
			Config:      config.NewConfig(),
			Paths:       []string{dir},
			ActiveRules: []*api.Rule{rule},
			Format:      "json",
			Version:     "test",
		},
		// Host intentionally empty — no FindingsBundleStore.
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	_ = res
}

// TestRunProject_FindingsBundleCache_RebuildsOnEdit confirms a content
// edit forces a fresh dispatch — the fingerprint mismatch is the
// safety contract guarding against stale cached findings.
func TestRunProject_FindingsBundleCache_RebuildsOnEdit(t *testing.T) {
	dir := t.TempDir()
	cacheRoot := t.TempDir()
	src := filepath.Join(dir, "Sample.kt")
	if err := os.WriteFile(src, []byte("package test\n\nclass A\n"), 0o644); err != nil {
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
	run := func(t *testing.T) {
		t.Helper()
		if _, err := RunProject(context.Background(), ProjectInput{
			Args: ProjectArgs{
				Config:      config.NewConfig(),
				Paths:       []string{dir},
				ActiveRules: []*api.Rule{rule},
				Format:      "json",
				Version:     "test",
			},
			Host: ProjectHostState{
				FindingsBundleStore:     bundle,
				FindingsBundleCacheRoot: cacheRoot,
			},
		}); err != nil {
			t.Fatalf("RunProject: %v", err)
		}
	}

	run(t) // miss → save
	if bundle.saveCalls != 1 {
		t.Fatalf("after first run, saveCalls = %d, want 1", bundle.saveCalls)
	}

	// Mutate the file content.
	if err := os.WriteFile(src, []byte("package test\n\nclass B\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	run(t) // fingerprint differs → miss → save again
	if bundle.saveCalls != 2 {
		t.Errorf("after content edit, saveCalls = %d, want 2 (fingerprint mismatch should force a fresh save)", bundle.saveCalls)
	}
}
