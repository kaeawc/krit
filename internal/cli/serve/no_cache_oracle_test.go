package serve

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/oracle"
)

// The daemon runs the oracle inside the project analysis, so --no-cache must
// switch its cache off as a one-shot scan's does (#1831).
func TestNoCacheOracleFollowsNoCache(t *testing.T) {
	for _, tc := range []struct {
		args daemon.AnalyzeProjectArgs
		want bool
	}{
		{daemon.AnalyzeProjectArgs{}, false},
		{daemon.AnalyzeProjectArgs{NoCacheOracle: true}, true},
		{daemon.AnalyzeProjectArgs{NoCache: true}, true},
	} {
		if got := noCacheOracle(tc.args); got != tc.want {
			t.Errorf("noCacheOracle(NoCache=%v, NoCacheOracle=%v) = %v, want %v", tc.args.NoCache, tc.args.NoCacheOracle, got, tc.want)
		}
	}
}

// krit-fir's incremental bundle is an oracle cache too: a --fir request that
// computes its oracle facts afresh must not hand it to the resident oracle.
func TestBuildProjectInput_FIRIncrementalDirFollowsNoCacheOracle(t *testing.T) {
	root := t.TempDir()
	jar := filepath.Join(root, "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	state := newDaemonState(root)
	state.cachedConfig = config.NewConfig()
	for _, tc := range []struct {
		args    daemon.AnalyzeProjectArgs
		wantDir bool
	}{
		{daemon.AnalyzeProjectArgs{Paths: []string{root}, Fir: true}, true},
		{daemon.AnalyzeProjectArgs{Paths: []string{root}, Fir: true, NoCacheOracle: true}, false},
		{daemon.AnalyzeProjectArgs{Paths: []string{root}, Fir: true, NoCache: true}, false},
	} {
		in, err := state.buildProjectInput(tc.args, oracle.BackendFIR)
		if err != nil {
			t.Fatal(err)
		}
		if got := in.Host.FIRIncrementalDir != ""; got != tc.wantDir {
			t.Errorf("NoCache=%v NoCacheOracle=%v: FIRIncrementalDir = %q, want set=%v", tc.args.NoCache, tc.args.NoCacheOracle, in.Host.FIRIncrementalDir, tc.wantDir)
		}
	}
}
