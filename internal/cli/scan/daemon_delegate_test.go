package scan

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/cli/daemonclient"
	"github.com/kaeawc/krit/internal/daemon"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestTryDaemonDelegate_NoDaemonShortCircuits guards the
// `--no-daemon` contract: even if a socket is reachable, the CLI must
// not attempt the dial when the user has opted out.
func TestTryDaemonDelegate_NoDaemonShortCircuits(t *testing.T) {
	socketDir := startMockDaemon(t, mockBehavior{})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	f := freshScanFlags(t)
	*f.NoDaemon = true

	handled, code := tryDaemonDelegate(f, []string{root}, root)
	if handled {
		t.Fatalf("expected fall-through; got handled=true code=%d", code)
	}
}

// TestTryDaemonDelegate_StartFailureFallsBack is the auto-start path:
// if no daemon can be started, the CLI should hand off to in-process
// by returning handled=false.
func TestTryDaemonDelegate_StartFailureFallsBack(t *testing.T) {
	root := newRoot(t)
	f := freshScanFlags(t)
	stubEnsureDaemonForScan(t, func(string, daemonclient.SpawnOptions) (*daemonclient.Client, bool, error) {
		return nil, false, fmt.Errorf("boom")
	})

	handled, _ := tryDaemonDelegate(f, []string{root}, root)
	if handled {
		t.Fatalf("expected fall-through when daemon start fails")
	}
}

func TestTryDaemonDelegate_AutoStartDisabledKeepsExistingDiscoveryBehavior(t *testing.T) {
	t.Setenv("KRIT_NO_DAEMON_AUTOSTART", "1")
	root := newRoot(t)
	f := freshScanFlags(t)
	stubEnsureDaemonForScan(t, func(string, daemonclient.SpawnOptions) (*daemonclient.Client, bool, error) {
		t.Fatal("ensureDaemonForScan should not be called when autostart is disabled")
		return nil, false, nil
	})

	handled, _ := tryDaemonDelegate(f, []string{root}, root)
	if handled {
		t.Fatalf("expected fall-through with autostart disabled and no running daemon")
	}
}

func TestTryDaemonDelegate_AutoStartsDaemon(t *testing.T) {
	socketDir := startMockDaemon(t, mockBehavior{
		findings:      `{"rules":["X"],"line":[1]}`,
		findingsCount: 1,
	})
	root := newRoot(t)
	f := freshScanFlags(t)
	stubEnsureDaemonForScan(t, func(repoRoot string, opts daemonclient.SpawnOptions) (*daemonclient.Client, bool, error) {
		if repoRoot != root {
			t.Fatalf("repoRoot = %q, want %q", repoRoot, root)
		}
		if !opts.AutoRestart {
			t.Fatal("AutoRestart = false, want true")
		}
		linkSock(t, root, filepath.Join(socketDir, "d.sock"))
		client, ok := daemonclient.Discover(root)
		return client, ok, nil
	})

	out := redirectStdout(t)
	handled, code := tryDaemonDelegate(f, []string{root}, root)
	if !handled {
		t.Fatalf("expected handled=true after daemon auto-start")
	}
	if code != 1 {
		t.Fatalf("code = %d, want 1", code)
	}
	if want := `{"rules":["X"],"line":[1]}`; out() != want {
		t.Errorf("findings byte mismatch:\n got %q\nwant %q", out(), want)
	}
}

// TestTryDaemonDelegate_HashMismatchFallsBack confirms the daemon's
// rejection lands as a fallback rather than an error: the CLI prints
// a warning (not asserted here) and continues in-process.
func TestTryDaemonDelegate_HashMismatchFallsBack(t *testing.T) {
	socketDir := startMockDaemon(t, mockBehavior{rejectHash: true})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	f := freshScanFlags(t)
	handled, _ := tryDaemonDelegate(f, []string{root}, root)
	if handled {
		t.Fatalf("expected fall-through after mismatch; got handled=true")
	}
}

// TestTryDaemonDelegate_FixRoutesViaDaemon confirms `--fix` now
// delegates to the daemon: the daemon ships FindingColumns under
// the optional "columns" wire segment and the CLI replays
// fixer.ApplyAllFixesColumns locally so the "daemon never writes
// user files" invariant holds. The mock daemon returns empty
// columns, so the CLI's runDaemonFix short-circuits with an
// "info: No auto-fixable issues found." line; the assertion here
// is on the routing (handled=true), not the message.
func TestTryDaemonDelegate_FixRoutesViaDaemon(t *testing.T) {
	socketDir := startMockDaemon(t, mockBehavior{
		columns: `{"n":0}`,
	})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	f := freshScanFlags(t)
	*f.Fix = true

	handled, _ := tryDaemonDelegate(f, []string{root}, root)
	if !handled {
		t.Fatalf("expected handled=true with --fix routed through daemon; got fall-through")
	}
}

// TestTryDaemonDelegate_CheckstyleRoutesViaColumns confirms `-f
// checkstyle` rides the FindingColumns wire segment (IncludeColumns=true)
// so the daemon's findings JSON byte stream cannot corrupt the envelope.
// The CLI renders the columnar Checkstyle XML locally; the daemon's
// AnalyzeProjectResult.Findings carries inert JSON that's discarded by
// writeDaemonFindings on this branch.
func TestTryDaemonDelegate_CheckstyleRoutesViaColumns(t *testing.T) {
	cols := scanner.CollectFindings([]scanner.Finding{{
		File:     "foo.kt",
		Line:     12,
		Col:      3,
		Severity: "warning",
		RuleSet:  "rs",
		Rule:     "R1",
		Message:  "msg",
	}})
	colsBytes, err := json.Marshal(&cols)
	if err != nil {
		t.Fatalf("marshal columns: %v", err)
	}
	socketDir := startMockDaemon(t, mockBehavior{
		findings:      `{"rules":["R1"],"line":[12]}`,
		findingsCount: 1,
		columns:       string(colsBytes),
	})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	out := redirectStdout(t)
	f := freshScanFlags(t)
	*f.Format = "checkstyle"

	handled, code := tryDaemonDelegate(f, []string{root}, root)
	if !handled {
		t.Fatalf("expected handled=true for -f checkstyle via daemon")
	}
	if code != 1 {
		t.Errorf("expected exit 1 with findings; got %d", code)
	}
	got := out()
	if !strings.HasPrefix(got, "<?xml") {
		t.Fatalf("expected checkstyle XML output; got %q", got)
	}
	if err := xml.Unmarshal([]byte(got), new(struct {
		XMLName xml.Name `xml:"checkstyle"`
	})); err != nil {
		t.Fatalf("checkstyle output does not parse as XML: %v\nbody: %s", err, got)
	}
	if !strings.Contains(got, "foo.kt") || !strings.Contains(got, "R1") {
		t.Errorf("checkstyle output missing finding fields:\n%s", got)
	}
}

// TestTryDaemonDelegate_PlainRoutesViaColumns mirrors the checkstyle
// case for `-f plain`: the daemon ships FindingColumns, the CLI runs
// FormatPlainColumns locally.
func TestTryDaemonDelegate_PlainRoutesViaColumns(t *testing.T) {
	cols := scanner.CollectFindings([]scanner.Finding{{
		File:     "bar.kt",
		Line:     7,
		Col:      4,
		Severity: "error",
		RuleSet:  "rs",
		Rule:     "R2",
		Message:  "boom",
	}})
	colsBytes, err := json.Marshal(&cols)
	if err != nil {
		t.Fatalf("marshal columns: %v", err)
	}
	socketDir := startMockDaemon(t, mockBehavior{
		findings:      `{"rules":["R2"],"line":[7]}`,
		findingsCount: 1,
		columns:       string(colsBytes),
	})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	out := redirectStdout(t)
	f := freshScanFlags(t)
	*f.Format = "plain"

	handled, code := tryDaemonDelegate(f, []string{root}, root)
	if !handled {
		t.Fatalf("expected handled=true for -f plain via daemon")
	}
	if code != 1 {
		t.Errorf("expected exit 1 with findings; got %d", code)
	}
	got := out()
	if got == "" {
		t.Fatal("expected non-empty plain output")
	}
	if !strings.Contains(got, "bar.kt") || !strings.Contains(got, "R2") || !strings.Contains(got, "boom") {
		t.Errorf("plain output missing finding fields:\n%s", got)
	}
}

// TestBuildDaemonAnalyzeArgs_ForwardsColumnsForNonJSONFormat pins the
// daemon-side optimisation: -f checkstyle / -f plain set IncludeColumns
// (so the columnar wire segment ships back) and rewrite Format=json on
// the wire so the daemon does not waste cycles formatting XML/text the
// CLI is going to discard anyway.
func TestBuildDaemonAnalyzeArgs_ForwardsColumnsForNonJSONFormat(t *testing.T) {
	for _, format := range []string{"checkstyle", "plain"} {
		t.Run(format, func(t *testing.T) {
			f := freshScanFlags(t)
			*f.Format = format
			args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
			if !args.IncludeColumns {
				t.Errorf("IncludeColumns = false, want true for -f %s", format)
			}
			if args.Format != "json" {
				t.Errorf("wire Format = %q, want %q for -f %s", args.Format, "json", format)
			}
		})
	}
}

// TestBuildDaemonAnalyzeArgs_RespectsReportOverride pins the regression
// path that broke CI: --report (or resolveEffectiveFormat's auto-promote
// json -> plain when stdout is a char device) must flow into the wire
// args alongside writeDaemonFindings's same decision. If the wire side
// uses raw *f.Format while the local renderer uses resolveEffectiveFormat,
// the daemon ships no columns ("json" wire, IncludeColumns=false) and
// the CLI tries to render "plain" from an empty columns segment, exit 2.
// CI's run_lint_test redirects -f json to /dev/null (char-device on
// Linux) and tripped exactly this skew.
func TestBuildDaemonAnalyzeArgs_RespectsReportOverride(t *testing.T) {
	f := freshScanFlags(t)
	*f.Format = "json"
	*f.Report = "plain"
	args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
	if !args.IncludeColumns {
		t.Errorf("IncludeColumns = false, want true when --report=plain promotes the effective format")
	}
	if args.Format != "json" {
		t.Errorf("wire Format = %q, want %q (plain riding the columns wire)", args.Format, "json")
	}
}

// TestTryDaemonDelegate_HappyPathReturnsExitFromFindings closes the
// loop: on a clean delegation, the CLI surfaces FindingsCount via
// the standard exit-code rule (0 = clean, 1 = any finding).
func TestTryDaemonDelegate_HappyPathReturnsExitFromFindings(t *testing.T) {
	socketDir := startMockDaemon(t, mockBehavior{
		findings:      `{"rules":["X"],"line":[1]}`,
		findingsCount: 1,
	})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	out := redirectStdout(t)
	f := freshScanFlags(t)
	handled, code := tryDaemonDelegate(f, []string{root}, root)
	if !handled {
		t.Fatalf("expected handled=true on clean delegation")
	}
	if code != 1 {
		t.Errorf("expected exit 1 with findings; got %d", code)
	}
	if want := `{"rules":["X"],"line":[1]}`; out() != want {
		t.Errorf("findings byte mismatch:\n got %q\nwant %q", out(), want)
	}
}

// freshScanFlags builds a default-valued scanFlags via a fresh
// FlagSet so each test gets independent storage.
func freshScanFlags(t *testing.T) *scanFlags {
	t.Helper()
	fs := flag.NewFlagSet("scan-test", flag.ContinueOnError)
	return registerScanFlags(fs)
}

func stubEnsureDaemonForScan(t *testing.T, fn func(string, daemonclient.SpawnOptions) (*daemonclient.Client, bool, error)) {
	t.Helper()
	orig := ensureDaemonForScan
	ensureDaemonForScan = fn
	t.Cleanup(func() { ensureDaemonForScan = orig })
}

// linkSock places root/.krit/daemon.sock pointing at socket so the
// default-discovery path inside TryConnect succeeds.
func linkSock(t *testing.T, root, socket string) {
	t.Helper()
	target := daemon.DefaultSocketPath(root)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.Symlink(socket, target); err != nil {
		t.Fatalf("symlink: %v", err)
	}
}

// newRoot returns a short-path tempdir under /tmp so the
// .krit/daemon.sock path stays under macOS's 104-byte cap.
func newRoot(t *testing.T) string {
	t.Helper()
	r, err := os.MkdirTemp("/tmp", "krit-deleg-")
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(r) })
	return r
}

// mockBehavior controls what startMockDaemon's analyze-project verb
// returns. rejectHash forces the daemon to refuse any non-empty
// client hash with the documented prefix; findings/findingsCount let
// tests assert on byte-for-byte response shape and exit code logic.
type mockBehavior struct {
	rejectHash    bool
	findings      string
	findingsCount int
	// columns, when non-empty, populates AnalyzeProjectResult.Columns
	// so callers can exercise the daemon-routed --fix / --rule-audit /
	// --baseline-audit / --delta / --remove-dead-code paths that
	// consume the optional FindingColumns wire segment.
	columns string
}

// startMockDaemon spins up a minimal server with status, shutdown,
// and analyze-project verbs registered. The analyze-project handler
// applies the configured behavior so each test can drive a different
// reply shape without re-implementing the wire protocol.
func startMockDaemon(t *testing.T, b mockBehavior) string {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-deleg-srv-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")
	srv := daemon.NewServer(socket)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true}, nil
	})
	srv.Register(daemon.VerbShutdown, func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	srv.Register(daemon.VerbAnalyzeProject, func(_ context.Context, raw json.RawMessage) (any, error) {
		var args daemon.AnalyzeProjectArgs
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &args); err != nil {
				return nil, err
			}
		}
		if b.rejectHash {
			return nil, fmt.Errorf("%s (daemon=ZZZZ client=%s)", daemon.ErrBinaryHashMismatchPrefix, args.ClientBinaryHash)
		}
		findings := b.findings
		if findings == "" {
			findings = `{"rules":[],"line":[]}`
		}
		result := daemon.AnalyzeProjectResult{
			Findings: json.RawMessage(findings),
			Stats:    daemon.AnalyzeProjectStats{FindingsCount: b.findingsCount},
		}
		if b.columns != "" {
			result.Columns = json.RawMessage(b.columns)
		}
		return result, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if daemon.Available(socket) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return socketDir
}

// redirectStdout swaps os.Stdout for a pipe and returns a thunk that
// drains the captured bytes. Used to assert byte-identical findings
// emission from the daemon path without spinning up an external
// process.
func redirectStdout(t *testing.T) func() string {
	t.Helper()
	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	t.Cleanup(func() { os.Stdout = orig })
	return func() string {
		_ = w.Close()
		buf, _ := io.ReadAll(r)
		return string(buf)
	}
}

// TestTryDaemonDelegate_ListRulesRoutesViaMetaVerb exercises the
// read-only-meta route added in the daemon-readonly-verbs PR: the
// CLI must dispatch --list-rules through the daemon's list-rules
// verb (capturing stdout/stderr/exit) instead of falling back to the
// in-process flag handler when a daemon is reachable.
func TestTryDaemonDelegate_ListRulesRoutesViaMetaVerb(t *testing.T) {
	socketDir := startMockMetaDaemon(t, "list-rules", daemon.MetaResult{
		Stdout:   []byte("Available rules: from-daemon\n"),
		ExitCode: 0,
	})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	out := redirectStdout(t)
	f := freshScanFlags(t)
	*f.List = true

	handled, code := tryDaemonDelegate(f, []string{root}, root)
	if !handled {
		t.Fatalf("expected handled=true; got fall-through")
	}
	if code != 0 {
		t.Errorf("expected exit 0; got %d", code)
	}
	if got := out(); got != "Available rules: from-daemon\n" {
		t.Errorf("expected daemon stdout replayed verbatim; got %q", got)
	}
}

// TestTryDaemonDelegate_ListRulesWithCustomJarsFallsThrough pins the
// CustomRuleJars carve-out: --custom-rule-jars requires the
// krit-types JVM the daemon doesn't manage on behalf of meta queries,
// so the CLI must fall back to in-process even with a daemon
// reachable.
func TestTryDaemonDelegate_ListRulesWithCustomJarsFallsThrough(t *testing.T) {
	socketDir := startMockMetaDaemon(t, "list-rules", daemon.MetaResult{
		Stdout:   []byte("daemon should NOT be called\n"),
		ExitCode: 0,
	})
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))

	f := freshScanFlags(t)
	*f.List = true
	*f.CustomRuleJars = "/tmp/fake.jar"

	handled, _ := tryDaemonDelegate(f, []string{root}, root)
	if handled {
		t.Fatalf("expected fall-through with --custom-rule-jars; got handled=true")
	}
}

// startMockMetaDaemon spins up a mock daemon that registers a single
// meta verb (verb name + canned MetaResult). Used by the delegate
// tests above to assert routing without depending on the real
// in-process meta-flag handlers.
func startMockMetaDaemon(t *testing.T, verb string, reply daemon.MetaResult) string {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-deleg-meta-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")
	srv := daemon.NewServer(socket)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true}, nil
	})
	srv.Register(verb, func(_ context.Context, _ json.RawMessage) (any, error) {
		return reply, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if daemon.Available(socket) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return socketDir
}

// TestDaemonCompatibleFlags_PerfAllowed pins the contract added with
// daemon-side perf tracking: --perf, --perf-rules, --profile-dispatch,
// --cpuprofile, and --memprofile must all be served by the daemon now
// that the wire carries dispatch_profile fan-out and the daemon wraps
// analyze-project in pprof capture for CPU/mem profile paths.
//
// --create-baseline and --dry-run are now daemon-served too: the
// daemon computes baseline IDs / fixable file lists and the CLI
// performs the local file write or stdout print.
//
// --rule-audit, --baseline-audit, and --delta are daemon-served via
// the AnalyzeProjectResult.Columns wire segment (IncludeColumns=true
// asks the daemon to ship post-pipeline FindingColumns alongside the
// findings JSON). The CLI runs the audit / delta filter locally.
// --fix / --remove-dead-code / --fix-binary remain in-process pending
// a fix-payload-over-the-wire design.
func TestDaemonCompatibleFlags_PerfAllowed(t *testing.T) {
	tests := []struct {
		name string
		set  func(*scanFlags)
		want bool
	}{
		{"no flags", func(f *scanFlags) {}, true},
		{"--perf", func(f *scanFlags) { *f.Perf = true }, true},
		{"--perf-rules", func(f *scanFlags) { *f.PerfRules = true }, true},
		{"--perf and --perf-rules", func(f *scanFlags) { *f.Perf = true; *f.PerfRules = true }, true},
		{"--profile-dispatch", func(f *scanFlags) { *f.ProfileDispatch = true }, true},
		{"--cpuprofile", func(f *scanFlags) { *f.CPUProfile = "/tmp/cpu.pprof" }, true},
		{"--memprofile", func(f *scanFlags) { *f.MemProfile = "/tmp/mem.pprof" }, true},
		// --fix, --fix-binary, --remove-dead-code are daemon-served:
		// the daemon ships FindingColumns on the wire under the
		// optional "columns" segment (no fixes applied daemon-side
		// because args.Fix / args.FixBinary are not forwarded across
		// the wire), and the CLI replays fixer.ApplyAllFixesColumns /
		// fixer.ApplyBinaryFixesBatchColumns / RunDeadCodeRemovalColumns
		// locally, preserving the "daemon never writes user files"
		// invariant.
		{"--fix", func(f *scanFlags) { *f.Fix = true }, true},
		{"--fix-binary", func(f *scanFlags) { *f.FixBinary = true }, true},
		{"--remove-dead-code", func(f *scanFlags) { *f.RemoveDeadCode = true }, true},
		// --no-cache rides on AnalyzeProjectArgs.NoCache; daemon
		// nils its on-disk cache pointers for the call but stays
		// the right place to serve it.
		{"--no-cache", func(f *scanFlags) { *f.NoCache = true }, true},
		// --clear-cache and --clear-matrix-cache are early-exit
		// verbs handled outside the analyze path via
		// tryDaemonClearCache / tryDaemonClearMatrixCache, so
		// daemonCompatibleFlags must NOT short-circuit on them the
		// way the analyze path used to.
		{"--clear-cache", func(f *scanFlags) { *f.ClearCache = true }, true},
		{"--clear-matrix-cache", func(f *scanFlags) { *f.ClearMatrixCache = true }, true},
		// --input-types and --sample-rule are daemon-served via the
		// AnalyzeProjectArgs.InputTypesPath wire field / JSON-envelope
		// post-processing on the CLI side.
		{"--input-types", func(f *scanFlags) { *f.InputTypes = "/tmp/types.json" }, true},
		{"--sample-rule", func(f *scanFlags) { *f.SampleRule = "MyRule" }, true},
		// --create-baseline and --dry-run are daemon-served: the
		// daemon computes BaselineIDs / fixable-file list, the CLI
		// performs the local write/print.
		{"--create-baseline", func(f *scanFlags) { *f.CreateBaseline = "/tmp/baseline.xml" }, true},
		{"--dry-run", func(f *scanFlags) { *f.DryRun = true }, true},
		// --rule-audit and --baseline-audit are daemon-served: the
		// daemon ships FindingColumns on the wire under the optional
		// "columns" segment and the CLI replays
		// RunRuleAuditColumns / RunBaselineAuditColumns locally.
		{"--rule-audit", func(f *scanFlags) { *f.RuleAudit = true }, true},
		{"--baseline-audit", func(f *scanFlags) { *f.BaselineAudit = true }, true},
		// --base-path is daemon-compatible — it's forwarded as
		// AnalyzeProjectArgs.BasePath so daemon-side baseline IDs
		// match in-process resolution.
		{"--base-path", func(f *scanFlags) { *f.BasePath = "/tmp/base" }, true},
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

// TestTryDaemonClearCache_NoClearFlagNoOp pins the entry-gate
// contract: when --clear-cache is not set, tryDaemonClearCache must
// not even attempt a dial.
func TestTryDaemonClearCache_NoClearFlagNoOp(t *testing.T) {
	root := newRoot(t)
	f := freshScanFlags(t)
	if tryDaemonClearCache(f, root) {
		t.Fatalf("expected handled=false when --clear-cache is unset")
	}
}

// TestTryDaemonClearCache_NoDaemonShortCircuits guards the
// --no-daemon flag for the clear path: even if a socket is reachable
// and --clear-cache is set, --no-daemon falls through to in-process.
func TestTryDaemonClearCache_NoDaemonShortCircuits(t *testing.T) {
	socketDir := startMockClearCacheDaemon(t, false)
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))
	f := freshScanFlags(t)
	*f.ClearCache = true
	*f.NoDaemon = true
	if tryDaemonClearCache(f, root) {
		t.Fatalf("expected fall-through with --no-daemon")
	}
}

// TestTryDaemonClearCache_HappyPath confirms the verb is invoked and
// the CLI exits cleanly (code 0) when the daemon reports success.
func TestTryDaemonClearCache_HappyPath(t *testing.T) {
	socketDir := startMockClearCacheDaemon(t, true)
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))
	f := freshScanFlags(t)
	*f.ClearCache = true
	if !tryDaemonClearCache(f, root) {
		t.Fatalf("expected handled=true on daemon success")
	}
}

// TestTryDaemonClearMatrixCache_NoClearFlagNoOp pins the entry-gate
// contract: when --clear-matrix-cache is not set,
// tryDaemonClearMatrixCache must not even attempt a dial.
func TestTryDaemonClearMatrixCache_NoClearFlagNoOp(t *testing.T) {
	root := newRoot(t)
	f := freshScanFlags(t)
	if tryDaemonClearMatrixCache(f, root) {
		t.Fatalf("expected handled=false when --clear-matrix-cache is unset")
	}
}

// TestTryDaemonClearMatrixCache_NoDaemonShortCircuits guards the
// --no-daemon flag for the matrix-clear path: even if a socket is
// reachable and --clear-matrix-cache is set, --no-daemon falls
// through to in-process.
func TestTryDaemonClearMatrixCache_NoDaemonShortCircuits(t *testing.T) {
	socketDir := startMockClearMatrixCacheDaemon(t, false)
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))
	f := freshScanFlags(t)
	*f.ClearMatrixCache = true
	*f.NoDaemon = true
	if tryDaemonClearMatrixCache(f, root) {
		t.Fatalf("expected fall-through with --no-daemon")
	}
}

// TestTryDaemonClearMatrixCache_HappyPath confirms the verb is
// invoked and the CLI exits cleanly (code 0) when the daemon reports
// success.
func TestTryDaemonClearMatrixCache_HappyPath(t *testing.T) {
	socketDir := startMockClearMatrixCacheDaemon(t, true)
	root := newRoot(t)
	linkSock(t, root, filepath.Join(socketDir, "d.sock"))
	f := freshScanFlags(t)
	*f.ClearMatrixCache = true
	if !tryDaemonClearMatrixCache(f, root) {
		t.Fatalf("expected handled=true on daemon success")
	}
}

// startMockClearMatrixCacheDaemon stands up a minimal daemon that
// implements just enough of clear-matrix-cache to drive
// tryDaemonClearMatrixCache assertions.
func startMockClearMatrixCacheDaemon(t *testing.T, cleared bool) string {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-clearmx-srv-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")
	srv := daemon.NewServer(socket)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true}, nil
	})
	srv.Register(daemon.VerbShutdown, func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	srv.Register(daemon.VerbClearMatrixCache, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.ClearMatrixCacheResult{Cleared: cleared}, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if daemon.Available(socket) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return socketDir
}

// startMockClearCacheDaemon stands up a minimal daemon that
// implements just enough of clear-cache to drive
// tryDaemonClearCache assertions.
func startMockClearCacheDaemon(t *testing.T, cleared bool) string {
	t.Helper()
	socketDir, err := os.MkdirTemp("/tmp", "krit-clear-srv-")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(socketDir) })
	socket := filepath.Join(socketDir, "d.sock")
	srv := daemon.NewServer(socket)
	srv.Register(daemon.VerbStatus, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.StatusResult{Ready: true}, nil
	})
	srv.Register(daemon.VerbShutdown, func(_ context.Context, _ json.RawMessage) (any, error) {
		return map[string]bool{"ok": true}, nil
	})
	srv.Register(daemon.VerbClearCache, func(_ context.Context, _ json.RawMessage) (any, error) {
		return daemon.ClearCacheResult{Cleared: cleared, ResidentInvalidated: true}, nil
	})
	if err := srv.Start(context.Background()); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(srv.Stop)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if daemon.Available(socket) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	return socketDir
}

// TestBuildDaemonAnalyzeArgs_ForwardsIncludeColumns confirms each
// column-consuming flag (--rule-audit, --baseline-audit, --delta,
// --fix, --fix-binary, --remove-dead-code) flips
// AnalyzeProjectArgs.IncludeColumns to true so the daemon emits the
// post-pipeline FindingColumns under the optional "columns" wire
// segment. Other flag sets keep it false to preserve the original
// {findings,stats[,dispatch_profile]} envelope.
func TestBuildDaemonAnalyzeArgs_ForwardsIncludeColumns(t *testing.T) {
	tests := []struct {
		name string
		set  func(*scanFlags)
		want bool
	}{
		{"default", func(*scanFlags) {}, false},
		{"--rule-audit", func(f *scanFlags) { *f.RuleAudit = true }, true},
		{"--baseline-audit", func(f *scanFlags) { *f.BaselineAudit = true }, true},
		{"--delta", func(f *scanFlags) { *f.Delta = "main" }, true},
		{"--rule-audit and --delta", func(f *scanFlags) { *f.RuleAudit = true; *f.Delta = "main" }, true},
		{"--fix", func(f *scanFlags) { *f.Fix = true }, true},
		{"--fix-binary", func(f *scanFlags) { *f.FixBinary = true }, true},
		{"--remove-dead-code", func(f *scanFlags) { *f.RemoveDeadCode = true }, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := freshScanFlags(t)
			tt.set(f)
			args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
			if args.IncludeColumns != tt.want {
				t.Errorf("IncludeColumns = %v, want %v", args.IncludeColumns, tt.want)
			}
		})
	}
}

// TestBuildDaemonAnalyzeArgs_ForwardsNoCache confirms --no-cache
// propagates into AnalyzeProjectArgs.NoCache so the daemon honours
// the on-disk-cache bypass.
func TestBuildDaemonAnalyzeArgs_ForwardsNoCache(t *testing.T) {
	f := freshScanFlags(t)
	*f.NoCache = true
	args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
	if !args.NoCache {
		t.Errorf("NoCache = false, want true")
	}
	// Negative — default is false.
	f2 := freshScanFlags(t)
	args2 := buildDaemonAnalyzeArgs(f2, []string{"/tmp"})
	if args2.NoCache {
		t.Errorf("NoCache = true with default flags, want false")
	}
}

// TestBuildDaemonAnalyzeArgs_ForwardsCustomRuleJars confirms the
// --custom-rule-jars flag rides on AnalyzeProjectArgs.CustomRuleJars
// so the daemon's analyze-project path loads the plugin rules.
// Without this the daemon runs the scan without plugin dispatch and
// returns zero plugin findings.
func TestBuildDaemonAnalyzeArgs_ForwardsCustomRuleJars(t *testing.T) {
	f := freshScanFlags(t)
	*f.CustomRuleJars = "/tmp/a.jar,/tmp/b.jar"
	args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
	want := []string{"/tmp/a.jar", "/tmp/b.jar"}
	if !reflect.DeepEqual(args.CustomRuleJars, want) {
		t.Errorf("CustomRuleJars = %v, want %v", args.CustomRuleJars, want)
	}
	f2 := freshScanFlags(t)
	args2 := buildDaemonAnalyzeArgs(f2, []string{"/tmp"})
	if len(args2.CustomRuleJars) != 0 {
		t.Errorf("CustomRuleJars = %v with default flags, want empty", args2.CustomRuleJars)
	}
}

// TestBuildDaemonAnalyzeArgs_ForwardsProfilingFlags confirms the
// profiling knob translates into the wire fields the daemon-side
// streamingAnalyzeResponse keys on. --profile-dispatch arms the
// per-file fan-out; --cpuprofile/--memprofile pass an absolute path
// (so the daemon writes next to the CLI's cwd, not the daemon's).
func TestBuildDaemonAnalyzeArgs_ForwardsProfilingFlags(t *testing.T) {
	f := freshScanFlags(t)
	*f.ProfileDispatch = true
	*f.CPUProfile = "cpu.pprof"
	*f.MemProfile = "mem.pprof"

	args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
	if !args.ProfileDispatch {
		t.Errorf("ProfileDispatch = false, want true")
	}
	if args.CPUProfilePath == "" || !filepath.IsAbs(args.CPUProfilePath) {
		t.Errorf("CPUProfilePath = %q, want non-empty absolute", args.CPUProfilePath)
	}
	if args.MemProfilePath == "" || !filepath.IsAbs(args.MemProfilePath) {
		t.Errorf("MemProfilePath = %q, want non-empty absolute", args.MemProfilePath)
	}
}

// TestBuildDaemonAnalyzeArgs_ForwardsPerfFlags confirms --perf and
// --perf-rules translate into the wire's ShowPerf / PerfRules fields
// (and --perf-rules implies ShowPerf, so the daemon builds a tracker).
func TestBuildDaemonAnalyzeArgs_ForwardsPerfFlags(t *testing.T) {
	tests := []struct {
		name         string
		set          func(*scanFlags)
		wantShow     bool
		wantPerfRule bool
	}{
		{"neither", func(f *scanFlags) {}, false, false},
		{"--perf", func(f *scanFlags) { *f.Perf = true }, true, false},
		{"--perf-rules implies show", func(f *scanFlags) { *f.PerfRules = true }, true, true},
		{"both", func(f *scanFlags) { *f.Perf = true; *f.PerfRules = true }, true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := freshScanFlags(t)
			tt.set(f)
			args := buildDaemonAnalyzeArgs(f, []string{"/tmp"})
			if args.ShowPerf != tt.wantShow {
				t.Errorf("ShowPerf = %v, want %v", args.ShowPerf, tt.wantShow)
			}
			if args.PerfRules != tt.wantPerfRule {
				t.Errorf("PerfRules = %v, want %v", args.PerfRules, tt.wantPerfRule)
			}
		})
	}
}
