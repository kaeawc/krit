package serve

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/oracle"
)

func TestAnalyzeProjectPreflightUsesForwardedModelAndMarker(t *testing.T) {
	root := t.TempDir()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "bin", "java"), []byte("#!/bin/sh\necho 'openjdk version \"21\"' >&2\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("JAVA_HOME", home)
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	state := newDaemonState(root)
	call := func(args daemon.AnalyzeProjectArgs) error {
		t.Helper()
		raw, err := json.Marshal(args)
		if err != nil {
			t.Fatal(err)
		}
		_, err = handleAnalyzeProject(context.Background(), state, raw)
		return err
	}
	missing := filepath.Join(root, "missing-model")
	if err := call(daemon.AnalyzeProjectArgs{Paths: []string{root}, GradleModel: missing, RequireWarm: true}); err == nil || !strings.Contains(err.Error(), "Gradle model cannot be read") {
		t.Fatalf("explicit model error = %v", err)
	}
	if err := call(daemon.AnalyzeProjectArgs{Paths: []string{root}, NoGradleModel: true, RequireWarm: true}); err == nil || !strings.Contains(err.Error(), "declared classpath source") {
		t.Fatalf("no-model error = %v", err)
	}
	t.Setenv("JAVA_HOME", filepath.Join(root, "missing-jdk"))
	if err := call(daemon.AnalyzeProjectArgs{Paths: []string{root}, FirPreflightPassed: true, RequireWarm: true}); err == nil || !strings.Contains(err.Error(), "not warm") {
		t.Fatalf("client-preflight marker should skip server preflight: %v", err)
	}
}

// A delegated `krit --fir` scan gets the FIR pass installed as the
// pipeline's findings post pass; without --fir the daemon path is unchanged.
func TestBuildProjectInput_InstallsFirPostPassOnlyForFir(t *testing.T) {
	root := t.TempDir()
	state := newDaemonState(root)
	cfg := config.NewConfig()
	state.cachedConfig = cfg

	off, err := state.buildProjectInput(daemon.AnalyzeProjectArgs{Paths: []string{root}, NoCache: true}, oracle.BackendKAA)
	if err != nil {
		t.Fatal(err)
	}
	if off.Host.FindingsPostPass != nil {
		t.Fatal("no --fir: the daemon must not install a findings post pass")
	}

	on, err := state.buildProjectInput(daemon.AnalyzeProjectArgs{Paths: []string{root}, NoCache: true, Fir: true}, oracle.BackendKAA)
	if err != nil {
		t.Fatal(err)
	}
	if on.Host.FindingsPostPass == nil {
		t.Fatal("--fir: the daemon must install the FIR pass as the findings post pass")
	}
}

func TestAnalyzeProjectDefaultsToFIRPreflight(t *testing.T) {
	t.Setenv("JAVA_HOME", t.TempDir())
	socket, _ := startServerForTest(t)
	var result daemon.AnalyzeProjectResult
	err := daemon.Call(socket, daemon.VerbAnalyzeProject, daemon.AnalyzeProjectArgs{}, &result)
	if err == nil || !strings.Contains(err.Error(), "Java 21") {
		t.Fatalf("default daemon scan should require FIR Java 21: %v", err)
	}
}
