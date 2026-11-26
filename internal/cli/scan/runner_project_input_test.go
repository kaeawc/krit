package scan

import (
	"bytes"
	"flag"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
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

// --min-sdk / --target-sdk become the default SDK levels of files whose build
// declares none, except a level the Gradle library profile declares (a
// version catalog's minSdk), which rules keep using; a negative level is a
// usage error (#1682).
func TestSDKLevelFlags(t *testing.T) {
	t.Cleanup(func() { android.SetDefaultSDKLevels(android.SDKLevels{}) })
	f := registerScanFlags(flag.NewFlagSet("krit", flag.ContinueOnError))
	*f.MinSdk, *f.TargetSdk = 24, 34
	r := &runner{f: f, sess: &Session{}, projectModelLoaded: true, tracker: perf.New(false)}
	levels, err := r.sdkLevelFlags()
	if err != nil || levels != (android.SDKLevels{MinSdk: 24, TargetSdk: 34}) {
		t.Fatalf("sdkLevelFlags = %+v, %v", levels, err)
	}
	r.applySDKLevelDefaults(levels)
	if got := android.DefaultSDKLevels(); got != levels {
		t.Fatalf("DefaultSDKLevels = %+v, want %+v", got, levels)
	}
	r.sess.LibraryFacts = librarymodel.FactsForProfile(librarymodel.ProjectProfile{MinSdkVersion: 26})
	r.applySDKLevelDefaults(levels)
	if got := android.DefaultSDKLevels(); got != (android.SDKLevels{TargetSdk: 34}) {
		t.Fatalf("with a profile minSdk: DefaultSDKLevels = %+v, want only the targetSdk", got)
	}
	r.applySDKLevelDefaults(android.SDKLevels{})
	if got := android.DefaultSDKLevels(); !got.IsZero() {
		t.Fatalf("a run without the flags left %+v", got)
	}
	*f.MinSdk = -1
	var stderr bytes.Buffer
	if _, err := r.checkedSDKLevelFlags(&stderr); err == nil || !strings.Contains(stderr.String(), "error: --min-sdk and --target-sdk must not be negative") {
		t.Fatalf("negative --min-sdk: err %v, stderr %q", err, stderr.String())
	}
}
