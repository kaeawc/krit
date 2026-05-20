//go:build kotlin_corpus

package serve

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestAnalyzeProject_RepeatedABIEditFindingsStable_KotlinCorpus is the
// regression repro for issue #254. With AndroidCacheWriter wired on
// the daemon path, post-edit analyze runs on $KRIT_KOTLIN_CORPUS collapse
// from ~87k findings to ~62 starting on the second run.
//
// Flow:
//  1. Spin daemon at the corpus.
//  2. Warm baseline analyze (no edit) — record findings count.
//  3. ABI-edit one Kotlin file (rename a public symbol).
//  4. Analyze run 1..N, capturing findings count after each.
//  5. Assert every post-edit run produces ≥ baselineCount-10 findings
//     (allow tiny drift for the edited file itself; the symptom is a
//     catastrophic collapse to ~62, not single-digit churn).
func TestAnalyzeProject_RepeatedABIEditFindingsStable_KotlinCorpus(t *testing.T) {
	root := os.Getenv("KRIT_KOTLIN_CORPUS")
	if root == "" {
		t.Skip("KRIT_KOTLIN_CORPUS not set")
	}
	if fi, err := os.Stat(root); err != nil || !fi.IsDir() {
		t.Skipf("KRIT_KOTLIN_CORPUS=%q is not a directory: %v", root, err)
	}

	socket, state := startServerWith(t, root, 10*time.Second)

	// Real watcher rooted at corpus — drives the dirty set that the
	// bundle-fingerprint check uses to invalidate on ABI edits. Without
	// this, daemon.Call(VerbAnalyzeProject) sees an empty dirty set
	// and trusts the cached bundle even after we mutate disk.
	w, err := startFileWatcher(context.Background(), state.root, state.workspace, nil)
	if err != nil {
		t.Fatalf("startFileWatcher: %v", err)
	}
	t.Cleanup(w.Stop)

	var baseline daemon.AnalyzeProjectResult
	if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{}, &baseline); err != nil {
		t.Fatalf("warm baseline analyze: %v", err)
	}
	baselineCount := baseline.Stats.FindingsCount
	t.Logf("warm baseline: %d findings", baselineCount)
	if baselineCount < 1000 {
		t.Fatalf("baseline findings = %d, expected ≫ 1000 (corpus misconfigured?)", baselineCount)
	}

	// Pick a Kotlin file with a top-level public class to ABI-edit.
	target, original := pickKotlinTarget(t, root)
	defer func() {
		_ = os.WriteFile(target, []byte(original), 0o644)
	}()

	// Rename the symbol by injecting a fresh suffix on every run so
	// the watcher sees a real content change.
	abiEdit := func(suffix string) {
		t.Helper()
		// Append a fresh public top-level declaration. That bumps the
		// file's ABI surface without touching existing identifiers, so
		// the rest of the corpus still compiles structurally.
		patched := original + "\nclass __KritAbiProbe" + suffix + "__ {}\n"
		if err := os.WriteFile(target, []byte(patched), 0o644); err != nil {
			t.Fatalf("write abi edit: %v", err)
		}
	}

	const runs = 4
	for i := 1; i <= runs; i++ {
		abiEdit(suffix(i))
		// Wait for the watcher to register the change.
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) && state.workspace.DirtyCount() == 0 {
			time.Sleep(20 * time.Millisecond)
		}
		var got daemon.AnalyzeProjectResult
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
			daemon.AnalyzeProjectArgs{}, &got); err != nil {
			t.Fatalf("run %d analyze: %v", i, err)
		}
		t.Logf("run %d: %d findings (Δ vs baseline = %d, bundleHit=%v, dirty=%d)",
			i, got.Stats.FindingsCount, got.Stats.FindingsCount-baselineCount,
			got.Stats.FindingsBundleHit, got.Stats.DirtyFiles)
		if got.Stats.FindingsCount < baselineCount-10 {
			t.Fatalf("run %d findings collapsed: got %d, expected ≥ baseline-10 (%d)",
				i, got.Stats.FindingsCount, baselineCount-10)
		}
	}
}

// pickKotlinTarget walks the corpus for a Kotlin file with a public
// top-level class declaration. Returns its absolute path and original
// content.
func pickKotlinTarget(t *testing.T, root string) (string, string) {
	t.Helper()
	var picked string
	var content string
	stop := filepath.SkipDir
	_ = stop
	walkErr := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || picked != "" {
			return nil
		}
		if d.IsDir() {
			// Skip build/test/script dirs that are unstable across runs.
			name := d.Name()
			if name == "build" || name == ".git" || name == ".krit" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".kt") {
			return nil
		}
		// Skip generated / sample dirs.
		if strings.Contains(path, "/build/") || strings.Contains(path, "/testData/") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		s := string(b)
		if !strings.Contains(s, "class ") && !strings.Contains(s, "object ") {
			return nil
		}
		if len(s) > 200_000 {
			return nil
		}
		picked = path
		content = s
		return filepath.SkipAll
	})
	if walkErr != nil || picked == "" {
		t.Fatalf("could not pick a Kotlin target: walkErr=%v picked=%q", walkErr, picked)
	}
	return picked, content
}

func suffix(i int) string {
	return string(rune('A' + i - 1))
}
