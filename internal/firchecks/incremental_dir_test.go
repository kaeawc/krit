package firchecks

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
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
