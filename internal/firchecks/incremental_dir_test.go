package firchecks

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
)

func incrementalDirOf(t *testing.T, target CompileTarget) (string, bool) {
	t.Helper()
	data, _, err := encodeCheckRequest(1, []fileRef{{Path: "/p/A.kt"}}, []string{"/p"}, nil, nil, nil, FileFacts{}, target)
	if err != nil {
		t.Fatal(err)
	}
	var req map[string]any
	if err := json.Unmarshal(data, &req); err != nil {
		t.Fatal(err)
	}
	dir, ok := req["incrementalDir"].(string)
	return dir, ok
}

func TestIncrementalDirOnTheCheckRequest(t *testing.T) {
	repo := t.TempDir()
	checker := &ProductionFirChecker{RepoDir: repo}
	want := filepath.Join(repo, ".krit", "fir-incremental")

	if got, ok := incrementalDirOf(t, checker.CompileTarget()); !ok || got != want {
		t.Fatalf("incrementalDir = %q (present %v), want %q", got, ok, want)
	}

	checker.NoCache = true
	if got, ok := incrementalDirOf(t, checker.CompileTarget()); ok {
		t.Fatalf("--no-cache must not send an incrementalDir, got %q", got)
	}

	if got, ok := incrementalDirOf(t, (&ProductionFirChecker{}).CompileTarget()); ok {
		t.Fatalf("no repo must not send an incrementalDir, got %q", got)
	}
}

func TestIncrementalDirIsNotPartOfTheCompileIdentity(t *testing.T) {
	a := CompileTarget{JvmTarget: "17", IncrementalDir: "/a/.krit/fir-incremental"}
	b := CompileTarget{JvmTarget: "17"}
	if !sameCompileTarget(a, b) {
		t.Fatal("the persistence directory must not split daemons")
	}
	if a.registrySuffix() != b.registrySuffix() {
		t.Fatal("the persistence directory must not change the daemon registry key")
	}
	if FirInvocationFingerprint(nil, "jar", nil, nil, FileFacts{}, a) != FirInvocationFingerprint(nil, "jar", nil, nil, FileFacts{}, b) {
		t.Fatal("the persistence directory must not change the check-cache fingerprint")
	}
	if strings.Contains(a.registrySuffix(), "incremental") {
		t.Fatal("registry suffix leaks the directory")
	}
}

func TestClearCacheRemovesThePersistedIncrementalBundles(t *testing.T) {
	repo := t.TempDir()
	dir := IncrementalDir(repo)
	if err := os.MkdirAll(filepath.Join(dir, "v1-abc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "v1-abc", "bundle.bin"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	sibling := filepath.Join(repo, ".krit", "fir-cache")
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := cacheutil.ClearAll(cacheutil.ClearContext{RepoDir: repo}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("fir-incremental survived --clear-cache: %v", err)
	}
	if _, err := os.Stat(sibling); !os.IsNotExist(err) {
		t.Fatalf("fir-cache survived --clear-cache: %v", err)
	}
	if err := cacheutil.ClearAll(cacheutil.ClearContext{}); err != nil {
		t.Fatalf("no repo is a no-op: %v", err)
	}
}
