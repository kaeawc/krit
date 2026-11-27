package scan

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/cli/daemonclient"
	"github.com/kaeawc/krit/internal/daemon"
)

// requestJSON is the analyze-project request f produces, as the daemon sees it.
func requestJSON(t *testing.T, f *scanFlags) map[string]any {
	t.Helper()
	raw, err := json.Marshal(buildDaemonAnalyzeArgs(f, nil))
	if err != nil {
		t.Fatal(err)
	}
	var req map[string]any
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatal(err)
	}
	return req
}

// --strict excludes noisy rules; the request has carried strict since #229,
// but the CLI never set it (#1956).
func TestBuildDaemonAnalyzeArgs_ForwardsStrict(t *testing.T) {
	f := freshScanFlags(t)
	*f.Strict = true
	if req := requestJSON(t, f); req["strict"] != true {
		t.Errorf("request strict = %v, want true", req["strict"])
	}
}

// In-process, --no-type-inference turns the type oracle off (source inference
// still runs), so a delegated scan asks the daemon for no type oracle.
func TestBuildDaemonAnalyzeArgs_NoTypeInferenceTurnsOffTypeOracle(t *testing.T) {
	f := freshScanFlags(t)
	*f.NoTypeInfer = true
	if req := requestJSON(t, f); req["no_type_oracle"] != true {
		t.Errorf("request no_type_oracle = %v, want true", req["no_type_oracle"])
	}
}

// --disable-related adds the related rules of each --disable-rules entry; the
// CLI sends the expanded list, so any daemon disables them.
func TestBuildDaemonAnalyzeArgs_DisableRelatedSendsRelatedRules(t *testing.T) {
	f := freshScanFlags(t)
	*f.DisableRules = "KaptUsage"
	*f.DisableRelated = true
	got, _ := requestJSON(t, f)["disable_rules"].(string)
	if got != "KaptUsage,KotlinAndroidPluginWithBuiltInKotlin" {
		t.Errorf("request disable_rules = %q, want KaptUsage,KotlinAndroidPluginWithBuiltInKotlin", got)
	}
}

// The in-process rule selection honours --disable-related too.
func TestSelectScanRules_DisableRelatedDropsRelatedRules(t *testing.T) {
	f := freshScanFlags(t)
	*f.DisableRules = "KaptUsage"
	*f.DisableRelated = true
	selected, _, _ := selectScanRules(f, loadScanConfig(f))
	for _, r := range selected {
		if r.ID == "KaptUsage" || r.ID == "KotlinAndroidPluginWithBuiltInKotlin" {
			t.Errorf("%s is still active with --disable-rules=KaptUsage --disable-related", r.ID)
		}
	}
}

// Guard: a plain scan sends the same request as before #1956, so it is still
// delegated and served from the daemon's warm state.
func TestBuildDaemonAnalyzeArgs_PlainScanSendsNoNewFields(t *testing.T) {
	req := requestJSON(t, freshScanFlags(t))
	for _, key := range []string{"strict", "no_type_oracle", "disable_rules", "no_parse_cache", "no_cache"} {
		if v, ok := req[key]; ok {
			t.Errorf("plain request carries %s=%v", key, v)
		}
	}
}

// --clear-cache and --clear-matrix-cache go to a running daemon's clear verbs
// first. With no daemon running they must clear in-process, not start a
// daemon and run a scan that clears nothing.
func TestTryDaemonDelegate_ClearFlagsNeverStartADaemon(t *testing.T) {
	for name, set := range map[string]func(*scanFlags){
		"--clear-cache":        func(f *scanFlags) { *f.ClearCache = true },
		"--clear-matrix-cache": func(f *scanFlags) { *f.ClearMatrixCache = true },
	} {
		t.Run(name, func(t *testing.T) {
			root := newRoot(t)
			started := false
			stubEnsureDaemonForScan(t, func(string, daemonclient.SpawnOptions) (*daemonclient.Client, bool, error) {
				started = true
				return nil, false, nil
			})
			f := freshScanFlags(t)
			set(f)
			if handled, _ := tryDaemonDelegate(f, []string{root}, root); handled || started {
				t.Errorf("handled=%v daemonStarted=%v, want an in-process run and no daemon", handled, started)
			}
		})
	}
}

// Guard: a plain scan still starts (or reuses) the daemon.
func TestTryDaemonDelegate_PlainScanStillStartsTheDaemon(t *testing.T) {
	root := newRoot(t)
	started := false
	stubEnsureDaemonForScan(t, func(string, daemonclient.SpawnOptions) (*daemonclient.Client, bool, error) {
		started = true
		return nil, false, nil
	})
	tryDaemonDelegate(freshScanFlags(t), []string{root}, root)
	if !started {
		t.Error("a plain scan did not ask for the daemon")
	}
}

// A --no-parse-cache result the daemon does not confirm is discarded and the
// scan runs in-process, which prints its own -v output. The daemon's verbose
// output and "using daemon" must not be printed first, or -v shows twice.
func TestRunDaemonAnalyze_UnconfirmedNoParseCachePrintsNoDaemonOutput(t *testing.T) {
	client := &streamingVerboseClient{verboseDaemonClient{result: daemon.AnalyzeProjectResult{
		Findings: []byte(`{"rules":[],"line":[]}`), VerboseOutput: "verbose: rest\n",
	}}}
	f := freshScanFlags(t)
	*f.Verbose = true
	*f.NoParseCache = true
	redirectStdout(t)
	var handled bool
	stderr := captureStderr(t, func() { handled, _ = runDaemonAnalyze(client, f, []string{newRoot(t)}) })
	if handled {
		t.Fatal("an unconfirmed --no-parse-cache result was accepted")
	}
	if strings.Contains(stderr, "verbose:") || strings.Contains(stderr, "using daemon") {
		t.Errorf("the discarded daemon result's output was printed: %q", stderr)
	}
	if !strings.Contains(stderr, "does not support --no-parse-cache") {
		t.Errorf("missing fallback warning: %q", stderr)
	}
}

// Guard: a confirmed --no-parse-cache result prints the daemon's -v output
// once, after "using daemon".
func TestRunDaemonAnalyze_ConfirmedNoParseCachePrintsVerboseOnce(t *testing.T) {
	client := &streamingVerboseClient{verboseDaemonClient{result: daemon.AnalyzeProjectResult{
		Findings:      []byte(`{"rules":[],"line":[]}`),
		VerboseOutput: "verbose: rest\n",
		Stats:         daemon.AnalyzeProjectStats{ParseCacheBypassed: true},
	}}}
	f := freshScanFlags(t)
	*f.Verbose = true
	*f.NoParseCache = true
	redirectStdout(t)
	var handled bool
	stderr := captureStderr(t, func() { handled, _ = runDaemonAnalyze(client, f, []string{newRoot(t)}) })
	if !handled {
		t.Fatal("a confirmed --no-parse-cache result was rejected")
	}
	if !strings.HasPrefix(stderr, "info: using daemon\nverbose: rest\ninfo: Found") {
		t.Errorf("stderr = %q", stderr)
	}
}
