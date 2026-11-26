package scan

import (
	"flag"
	"testing"

	"github.com/kaeawc/krit/internal/perf"
)

// --no-type-oracle (and --depth fast, which sets it) must reach the project
// args: the findings bundle's rule hash is the only input that separates such
// a run from an oracle-backed one over the same tree, so without it a
// pre-parse bundle hit replays the other run's findings.
func TestProjectInputPassesNoTypeOracle(t *testing.T) {
	for _, noOracle := range []bool{false, true} {
		f := registerScanFlags(flag.NewFlagSet("krit", flag.ContinueOnError))
		*f.NoTypeOracle = noOracle
		r := &runner{f: f, sess: &Session{}, tracker: perf.New(false)}
		if got := r.projectInput().Args.NoTypeOracle; got != noOracle {
			t.Fatalf("--no-type-oracle=%v: ProjectArgs.NoTypeOracle = %v", noOracle, got)
		}
	}
}

// --base-path bounds module discovery for file scan paths on the CLI path as
// it does in the daemon, so both discover the same module graph (#1704).
func TestProjectInputPassesBasePathAsModuleRootCeiling(t *testing.T) {
	dir := t.TempDir()
	for _, tc := range []struct{ flag, want string }{{"", ""}, {dir, dir}} {
		f := registerScanFlags(flag.NewFlagSet("krit", flag.ContinueOnError))
		*f.BasePath = tc.flag
		r := &runner{f: f, sess: &Session{}, tracker: perf.New(false)}
		if got := r.projectInput().Args.ModuleRootCeiling; got != tc.want {
			t.Fatalf("--base-path=%q: ProjectArgs.ModuleRootCeiling = %q, want %q", tc.flag, got, tc.want)
		}
	}
}
