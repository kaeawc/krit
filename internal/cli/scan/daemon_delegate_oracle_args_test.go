package scan

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestDaemonCompatibleFlags_OracleArgsAllowed pins the oracle I/O and
// sampling bucket: --input-types and --sample-rule are wire-routable
// (the CLI absolutizes the path / re-runs the sampler on returned JSON).
// --delta is daemon-served via the AnalyzeProjectResult.Columns wire
// segment: the daemon ships post-pipeline FindingColumns, the CLI
// applies the delta filter against the base-ref worktree snapshot
// locally. --output-types is daemon-served via the dump-types meta
// verb (handled outside daemonCompatibleFlags), so this gate must
// NOT short-circuit on it either — the meta-verb dispatch runs first
// and any analyze-project fall-through still needs to succeed.
func TestDaemonCompatibleFlags_OracleArgsAllowed(t *testing.T) {
	tests := []struct {
		name string
		set  func(*scanFlags)
		want bool
	}{
		{"--input-types", func(f *scanFlags) { *f.InputTypes = "/tmp/types.json" }, true},
		{"--sample-rule", func(f *scanFlags) { *f.SampleRule = "MyRule" }, true},
		{"--output-types", func(f *scanFlags) { *f.OutputTypes = "/tmp/out.json" }, true},
		{"--delta", func(f *scanFlags) { *f.Delta = "main" }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := freshScanFlags(t)
			tt.set(f)
			if got := daemonCompatibleFlags(f); got != tt.want {
				t.Errorf("daemonCompatibleFlags = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDaemonCompatibleFlags_OracleBackendForwardedNotBypassed pins
// the post-wire behavior: AnalyzeProjectArgs.OracleBackend now carries
// the user's choice to the daemon, so daemonCompatibleFlags admits
// every spelling. The serve handler validates and returns a typed
// ErrUnsupportedOracleBackendPrefix when it can't honor the request —
// runDaemonAnalyze catches that and falls back to in-process.
// Bypassing here would prevent the daemon from ever serving a backend
// that's wired but not-yet-implemented end-to-end.
func TestDaemonCompatibleFlags_OracleBackendForwardedNotBypassed(t *testing.T) {
	for _, backend := range []string{"", "kaa", "fir"} {
		t.Run("backend="+backend, func(t *testing.T) {
			f := freshScanFlags(t)
			*f.OracleBackend = backend
			if got := daemonCompatibleFlags(f); !got {
				t.Errorf("daemonCompatibleFlags(--oracle-backend=%q) = false, want true (backend now travels over the wire)", backend)
			}
		})
	}
}

// TestBuildDaemonAnalyzeArgs_ForwardsOracleBackend confirms the
// user's --oracle-backend value reaches the wire verbatim.
func TestBuildDaemonAnalyzeArgs_ForwardsOracleBackend(t *testing.T) {
	for _, backend := range []string{"", "kaa", "fir"} {
		t.Run("backend="+backend, func(t *testing.T) {
			f := freshScanFlags(t)
			*f.OracleBackend = backend
			args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
			if args.OracleBackend != backend {
				t.Errorf("OracleBackend = %q, want %q", args.OracleBackend, backend)
			}
		})
	}
}

// TestBuildDaemonAnalyzeArgs_InputTypesAbsolutised confirms the CLI
// resolves --input-types against the caller's CWD before forwarding so
// the daemon (which runs from the project root) can open the file. A
// path that already is absolute must round-trip unchanged.
func TestBuildDaemonAnalyzeArgs_InputTypesAbsolutised(t *testing.T) {
	f := freshScanFlags(t)
	*f.InputTypes = "rel/types.json"
	args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
	if !filepath.IsAbs(args.InputTypesPath) {
		t.Errorf("expected absolute path; got %q", args.InputTypesPath)
	}
	if !strings.HasSuffix(args.InputTypesPath, "rel/types.json") {
		t.Errorf("unexpected suffix: %q", args.InputTypesPath)
	}

	g := freshScanFlags(t)
	*g.InputTypes = "/abs/types.json"
	args2 := buildDaemonAnalyzeArgs(g, []string{"/tmp"})
	if args2.InputTypesPath != "/abs/types.json" {
		t.Errorf("absolute path mutated: %q", args2.InputTypesPath)
	}
}

// TestBuildDaemonAnalyzeArgs_SampleRuleForcesJSON pins the format
// override the sampler short-circuit relies on: regardless of the
// user's --format / --report choice, the daemon must return JSON so
// the CLI can decode the envelope back into FindingColumns.
func TestBuildDaemonAnalyzeArgs_SampleRuleForcesJSON(t *testing.T) {
	f := freshScanFlags(t)
	*f.Format = "plain"
	*f.SampleRule = "MyRule"
	args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
	if args.Format != "json" {
		t.Errorf("Format = %q, want json", args.Format)
	}
}

// TestTryDaemonDelegate_SampleRuleConsumesEnvelope drives the sampler
// short-circuit end-to-end: the mock daemon returns a JSONReport
// envelope, the CLI parses it, runs RunSampleFindingsColumns, and
// emits the per-rule sample to stdout instead of forwarding the JSON
// blob verbatim.
func TestTryDaemonDelegate_SampleRuleConsumesEnvelope(t *testing.T) {
	envelope := mustJSON(t, map[string]any{
		"success":    true,
		"version":    "test",
		"durationMs": 0,
		"files":      1,
		"rules":      1,
		"findings": []map[string]any{
			{
				"file":     "Foo.kt",
				"line":     1,
				"column":   1,
				"ruleSet":  "test",
				"rule":     "MyRule",
				"severity": "warning",
				"message":  "match",
			},
		},
		"summary": map[string]any{
			"total":     1,
			"byRuleSet": map[string]int{},
			"byRule":    map[string]int{},
			"fixable":   0,
		},
	})
	socketDir := startMockDaemon(t, mockBehavior{findings: envelope, findingsCount: 1})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	out := redirectStdout(t)
	f := freshScanFlags(t)
	*f.SampleRule = "MyRule"
	*f.SampleCount = 1
	handled, code := tryDaemonDelegate(f, []string{root}, root)
	if !handled {
		t.Fatalf("expected handled=true on sample-rule delegation")
	}
	if code != 0 {
		t.Errorf("expected exit 0 (sampler success); got %d", code)
	}
	captured := out()
	if !strings.Contains(captured, "MyRule") {
		t.Errorf("expected sampler output to mention rule; got %q", captured)
	}
	// Guard against the regression where the JSON envelope leaks
	// through verbatim instead of being consumed.
	if strings.Contains(captured, `"findings"`) {
		t.Errorf("sampler output leaked raw JSON envelope; got %q", captured)
	}
}

// mustJSON marshals v or fails the test. Local helper to keep the
// envelope-construction inline in the test that uses it.
func mustJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}
