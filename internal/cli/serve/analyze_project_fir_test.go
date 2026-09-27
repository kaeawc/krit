package serve

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/hashutil"
	"github.com/kaeawc/krit/internal/oracle"
)

// The daemon survives multiple scans. A same-size, same-mtime edit between
// requests must not reuse the previous request's source content hash.
func TestHandleAnalyzeProjectResetsHashMemoBetweenRequests(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "Sample.kt")
	stamp := time.Unix(1577836800, 0)
	write := func(content string) {
		t.Helper()
		if err := os.WriteFile(file, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(file, stamp, stamp); err != nil {
			t.Fatal(err)
		}
	}
	hashutil.ResetDefault()
	t.Cleanup(hashutil.ResetDefault)
	write("fun a() {}\n")
	before, err := hashutil.Default().HashFile(file, nil)
	if err != nil {
		t.Fatal(err)
	}
	write("fun b() {}\n")
	args, err := json.Marshal(daemon.AnalyzeProjectArgs{RequireWarm: true})
	if err != nil {
		t.Fatal(err)
	}
	_, err = handleAnalyzeProject(context.Background(), newDaemonState(root), args)
	if !errors.Is(err, errDaemonNotWarm) {
		t.Fatalf("cold RequireWarm request returned %v, want %v", err, errDaemonNotWarm)
	}
	after, err := hashutil.Default().HashFile(file, nil)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Fatal("daemon request retained stale hash after same-size, same-mtime edit")
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
