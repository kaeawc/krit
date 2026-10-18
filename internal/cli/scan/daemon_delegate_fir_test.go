package scan

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// `krit --daemon --fir` must behave like a direct `krit --fir`: the wire
// carries the FIR switches so the daemon runs the same verdict pass.
func TestBuildDaemonAnalyzeArgs_ForwardsFir(t *testing.T) {
	cases := []struct {
		name                 string
		fir, noFir, noDaemon bool
		wantFir, wantNoDaemo bool
	}{
		{name: "off by default"},
		{name: "--fir", fir: true, wantFir: true},
		{name: "--fir --no-fir", fir: true, noFir: true},
		{name: "--fir --no-fir-daemon", fir: true, noDaemon: true, wantFir: true, wantNoDaemo: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := freshScanFlags(t)
			*f.Fir, *f.NoFir, *f.NoFirDaemon = tc.fir, tc.noFir, tc.noDaemon
			if !daemonCompatibleFlags(f) {
				t.Fatal("FIR flags must not bypass daemon delegation")
			}
			args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
			if args.Fir != tc.wantFir || args.NoFirDaemon != tc.wantNoDaemo {
				t.Fatalf("Fir=%v NoFirDaemon=%v, want %v %v", args.Fir, args.NoFirDaemon, tc.wantFir, tc.wantNoDaemo)
			}
			raw, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(raw), `"fir":true`); got != tc.wantFir {
				t.Fatalf("wire %s: fir present=%v, want %v", raw, got, tc.wantFir)
			}
		})
	}
}

func TestBuildDaemonAnalyzeArgs_ForwardsGradlePreflight(t *testing.T) {
	f := freshScanFlags(t)
	*f.GradleModel = "/tmp/explicit-model"
	*f.NoGradleModel = true
	f.firPreflightPassed = true
	f.modelClasspath = []string{"/tmp/model.jar"}
	args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
	if args.GradleModel != *f.GradleModel || !args.NoGradleModel || !args.FirPreflightPassed || len(args.OracleClasspath) == 0 {
		t.Fatalf("forwarded args: %+v", args)
	}
}

// Issue #855: `--no-type-oracle` and `--no-fir` must reach the daemon on the
// wire; dropping them made a delegated structural scan run the oracle anyway.
func TestBuildDaemonAnalyzeArgs_ForwardsNoTypeOracleAndNoFir(t *testing.T) {
	cases := []struct {
		name                        string
		noTypeOracle, fir, noFir    bool
		wantNoTypeOracle, wantNoFir bool
	}{
		{name: "defaults", wantNoFir: true},
		{name: "--no-type-oracle", noTypeOracle: true, wantNoTypeOracle: true, wantNoFir: true},
		{name: "--no-fir", noFir: true, wantNoFir: true},
		{name: "--no-type-oracle --no-fir", noTypeOracle: true, noFir: true, wantNoTypeOracle: true, wantNoFir: true},
		{name: "--fir", fir: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := freshScanFlags(t)
			*f.NoTypeOracle, *f.Fir, *f.NoFir = tc.noTypeOracle, tc.fir, tc.noFir
			if !daemonCompatibleFlags(f) {
				t.Fatal("oracle/FIR toggles must not bypass daemon delegation")
			}
			args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
			if args.NoTypeOracle != tc.wantNoTypeOracle || args.NoFir != tc.wantNoFir {
				t.Fatalf("NoTypeOracle=%v NoFir=%v, want %v %v", args.NoTypeOracle, args.NoFir, tc.wantNoTypeOracle, tc.wantNoFir)
			}
			raw, err := json.Marshal(args)
			if err != nil {
				t.Fatal(err)
			}
			if got := strings.Contains(string(raw), `"no_type_oracle":true`); got != tc.wantNoTypeOracle {
				t.Fatalf("wire %s: no_type_oracle present=%v, want %v", raw, got, tc.wantNoTypeOracle)
			}
		})
	}
}

// --depth=fast (or analysis.depth: fast) disables the oracle; a delegated
// scan must resolve it before building the daemon request.
func TestResolveDepthForDelegation_FastDisablesOracleOnTheWire(t *testing.T) {
	dir := t.TempDir()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	f := registerScanFlags(fs)
	if err := fs.Parse([]string{"--depth=fast", dir}); err != nil {
		t.Fatal(err)
	}
	resolveDepthForDelegation(f, fs, fs.Args())
	if args := buildDaemonAnalyzeArgs(f, fs.Args()); !args.NoTypeOracle {
		t.Fatal("--depth=fast must reach the daemon as no_type_oracle")
	}

	// From krit.yml too.
	cfgDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(cfgDir, "krit.yml"), []byte("analysis:\n  depth: fast\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	fs = flag.NewFlagSet("test", flag.ContinueOnError)
	f = registerScanFlags(fs)
	if err := fs.Parse([]string{"--config", filepath.Join(cfgDir, "krit.yml"), dir}); err != nil {
		t.Fatal(err)
	}
	resolveDepthForDelegation(f, fs, fs.Args())
	if args := buildDaemonAnalyzeArgs(f, fs.Args()); !args.NoTypeOracle {
		t.Fatal("analysis.depth: fast must reach the daemon as no_type_oracle")
	}

	// An explicit --no-type-oracle=false still wins over the preset.
	fs = flag.NewFlagSet("test", flag.ContinueOnError)
	f = registerScanFlags(fs)
	if err := fs.Parse([]string{"--depth=fast", "--no-type-oracle=false", dir}); err != nil {
		t.Fatal(err)
	}
	resolveDepthForDelegation(f, fs, fs.Args())
	if args := buildDaemonAnalyzeArgs(f, fs.Args()); args.NoTypeOracle {
		t.Fatal("explicit --no-type-oracle=false must override the fast preset")
	}
}
