package serve

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestAnalyzeProject_ManifestFromOtherCwdSpellingKeepsSourceSet is the
// regression for `make regression` scanning zero files. `go test
// ./cmd/krit` scans ../../playground/kotlin-webservice/ from cmd/krit
// and persists a findings-bundle manifest whose file paths carry that
// ../../ spelling. The regression script then scans
// playground/kotlin-webservice/ from the repo root through the daemon,
// which looked the manifest up by the scan path's absolute form, found
// the cmd/krit one, and handed its ../../ file paths to the pipeline as
// the source set. None resolve from the repo root, so every Kotlin file
// dropped out, with or without --no-cache.
func TestAnalyzeProject_ManifestFromOtherCwdSpellingKeepsSourceSet(t *testing.T) {
	repo := t.TempDir()
	project := filepath.Join(repo, "proj")
	nested := filepath.Join(repo, "cmd", "krit")
	src := filepath.Join(project, "src")
	for _, dir := range []string{src, nested} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeKotlinFile(t, src, "A.kt", "package demo\n\nclass A\n")
	writeKotlinFile(t, src, "B.kt", "package demo\n\nclass B\n")

	// The first daemon stands in for the cmd/krit test run: it scans
	// with the ../../ spelling and persists the manifest to disk.
	t.Chdir(nested)
	writer, writerState := startServerWith(t, project, time.Second)
	var first daemon.AnalyzeProjectResult
	if err := daemon.Call(writer, daemon.VerbAnalyzeProject,
		daemon.AnalyzeProjectArgs{Paths: []string{"../../proj/"}, NoFir: true}, &first); err != nil {
		t.Fatalf("nested-cwd call: %v", err)
	}
	if first.Stats.FilesScanned != 2 {
		t.Fatalf("nested-cwd scan: FilesScanned = %d, want 2", first.Stats.FilesScanned)
	}
	repoDir := writerState.repoDir
	if repoDir == "" {
		repoDir = writerState.root
	}
	manifestPath := scanner.FindingsBundleManifestPath(repoDir,
		scanner.FindingsBundleManifestKey(repoDir, []string{"../../proj/"}))
	if !waitForCondition(func() bool {
		_, err := os.Stat(manifestPath)
		return err == nil
	}) {
		t.Fatalf("nested-cwd scan never persisted its manifest at %s", manifestPath)
	}

	// A fresh daemon, like the one `make regression` spawns, then scans
	// the same project spelled from the repo root.
	t.Chdir(repo)
	socket, _ := startServerWith(t, project, time.Second)
	for _, noCache := range []bool{true, false} {
		var got daemon.AnalyzeProjectResult
		if err := daemon.Call(socket, daemon.VerbAnalyzeProject,
			daemon.AnalyzeProjectArgs{Paths: []string{"proj/"}, NoCache: noCache, NoFir: true}, &got); err != nil {
			t.Fatalf("repo-root call (NoCache=%v): %v", noCache, err)
		}
		if got.Stats.FilesScanned != 2 {
			t.Errorf("repo-root scan (NoCache=%v): FilesScanned = %d, want 2", noCache, got.Stats.FilesScanned)
		}
	}
}

// TestPriorSourceState_NoCacheSkipsManifest pins that --no-cache keeps
// the on-disk findings-bundle manifest out of the request: no
// prepopulated source paths and no prior per-file maps.
func TestPriorSourceState_NoCacheSkipsManifest(t *testing.T) {
	root := t.TempDir()
	path := writeKotlinFile(t, root, "A.kt", "package demo\n\nclass A\n")
	state := newDaemonState(root)
	paths := []string{root}
	seedResidentManifest(t, state, root, paths, path)

	kotlin, _, prior := state.priorSourceState(false, root, paths)
	if len(kotlin) != 1 || len(prior.ContentHashes) != 1 {
		t.Fatalf("cached call: kotlin=%v prior=%v, want the seeded manifest", kotlin, prior.ContentHashes)
	}

	kotlin, java, prior := state.priorSourceState(true, root, paths)
	if kotlin != nil || java != nil || prior.ContentHashes != nil || prior.StructuralFPs != nil {
		t.Errorf("NoCache call used the manifest: kotlin=%v java=%v prior=%+v", kotlin, java, prior)
	}
}

// seedResidentManifest stores a one-file manifest in the daemon's
// resident manifest cache under the key a request for paths looks up.
func seedResidentManifest(t *testing.T, state *daemonState, repoDir string, paths []string, file string) {
	t.Helper()
	key := scanner.FindingsBundleManifestKey(repoDir, paths)
	state.saveManifest(scanner.FindingsBundleManifestPath(repoDir, key), scanner.FindingsBundleManifest{
		ContentHashes: map[string]string{file: "content"},
		StructuralFPs: map[string]string{file: "structural"},
	})
}
