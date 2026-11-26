package scan

import (
	"flag"
	"testing"

	"github.com/kaeawc/krit/internal/android"
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

// --min-sdk / --target-sdk become the SDK levels of files that declare none;
// a negative level is a usage error (#1682).
func TestApplySDKLevelFlags(t *testing.T) {
	t.Cleanup(func() { android.SetDefaultSDKLevels(android.SDKLevels{}) })
	f := registerScanFlags(flag.NewFlagSet("krit", flag.ContinueOnError))
	*f.MinSdk, *f.TargetSdk = 24, 34
	r := &runner{f: f}
	if code, err := r.applySDKLevelFlags(); err != nil || code != 0 {
		t.Fatalf("applySDKLevelFlags = %d, %v", code, err)
	}
	if got := android.DefaultSDKLevels(); got != (android.SDKLevels{MinSdk: 24, TargetSdk: 34}) {
		t.Fatalf("DefaultSDKLevels = %+v", got)
	}
	*f.MinSdk = -1
	if code, err := r.applySDKLevelFlags(); err == nil || code != 2 {
		t.Fatalf("negative --min-sdk: %d, %v; want exit 2 and an error", code, err)
	}
}
