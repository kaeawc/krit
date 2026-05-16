package serve

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/daemon"
)

// TestListRulesVerb_RoundTrip exercises the daemon list-rules verb
// end-to-end: register handler, dial socket, decode MetaResult.
// Asserts the daemon emits the same canonical "Available rules:"
// header and "A=active by default" footer the in-process flag
// produces — guarding against silent drift if the listing format
// changes only on one path.
func TestListRulesVerb_RoundTrip(t *testing.T) {
	socket, _ := startServerForTest(t)

	var got daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbListRules, daemon.ListRulesArgs{}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", got.ExitCode, string(got.Stderr))
	}
	stdout := string(got.Stdout)
	if !strings.HasPrefix(stdout, "Available rules:\n") {
		t.Errorf("expected stdout to start with 'Available rules:'; got prefix %q", firstLine(stdout))
	}
	if !strings.Contains(stdout, "A=active by default") {
		t.Errorf("expected legend footer in stdout; got tail %q", lastLine(stdout))
	}
}

// TestListRulesVerb_BadMaturityReturnsExitCode2 pins the no-os.Exit
// refactor: the daemon must surface bad-maturity errors via
// MetaResult.ExitCode + Stderr instead of crashing the process.
func TestListRulesVerb_BadMaturityReturnsExitCode2(t *testing.T) {
	socket, _ := startServerForTest(t)

	var got daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbListRules, daemon.ListRulesArgs{Maturity: "bogus"}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.ExitCode != 2 {
		t.Fatalf("expected exit 2 for bad --maturity, got %d", got.ExitCode)
	}
	if !strings.Contains(string(got.Stderr), "unknown --maturity value") {
		t.Errorf("expected error message in stderr; got %q", string(got.Stderr))
	}
}

// TestListExperimentsVerb_JSONShape asserts the daemon emits the
// canonical {"version":...,"experiments":[...]} JSON envelope when
// Format is "json" — same as runListExperimentsFlag's default path.
func TestListExperimentsVerb_JSONShape(t *testing.T) {
	socket, _ := startServerForTest(t)

	var got daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbListExperiments, daemon.ListExperimentsArgs{Format: "json"}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", got.ExitCode)
	}
	var env struct {
		Version     string                   `json:"version"`
		Experiments []map[string]interface{} `json:"experiments"`
	}
	if err := json.Unmarshal(got.Stdout, &env); err != nil {
		t.Fatalf("decode envelope: %v\n%s", err, string(got.Stdout))
	}
	if env.Version == "" {
		t.Errorf("expected non-empty version field")
	}
	if env.Experiments == nil {
		t.Errorf("expected non-nil experiments slice")
	}
}

// TestListExperimentsVerb_PlainHeader asserts the plain format
// includes the "Experiments (N total)" header so the daemon path
// matches ListExperimentsLifecyclePlain byte-for-byte.
func TestListExperimentsVerb_PlainHeader(t *testing.T) {
	socket, _ := startServerForTest(t)

	var got daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbListExperiments, daemon.ListExperimentsArgs{Format: "plain"}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", got.ExitCode)
	}
	if !strings.HasPrefix(string(got.Stdout), "Experiments (") {
		t.Errorf("expected plain header; got prefix %q", firstLine(string(got.Stdout)))
	}
}

// TestValidateConfigVerb_ResidentConfigPasses exercises the
// resident-config path: empty ConfigPath, daemon's autodetected
// (and possibly absent) krit.yml validates cleanly.
func TestValidateConfigVerb_ResidentConfigPasses(t *testing.T) {
	socket, _ := startServerForTest(t)

	var got daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbValidateConfig, daemon.ValidateConfigArgs{}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("expected exit 0 for empty resident config, got %d (stderr=%q)", got.ExitCode, string(got.Stderr))
	}
	if !strings.Contains(string(got.Stderr), "Config validation passed") {
		t.Errorf("expected pass message in stderr; got %q", string(got.Stderr))
	}
}

// TestValidateConfigVerb_ExplicitPathLoadsFile pins the
// --config FILE override path: an explicit ConfigPath forces a
// one-shot load instead of reusing the resident config.
func TestValidateConfigVerb_ExplicitPathLoadsFile(t *testing.T) {
	socket, _ := startServerForTest(t)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "krit.yml")
	if err := os.WriteFile(cfgPath, []byte("style:\n  MagicNumber:\n    ignoreNumbers: ['0', '1']\n"), 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}

	var got daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbValidateConfig, daemon.ValidateConfigArgs{ConfigPath: cfgPath}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("expected exit 0 for valid explicit config, got %d (stderr=%q)", got.ExitCode, string(got.Stderr))
	}
}

// TestOracleFilterFingerprintVerb_EmitsJSONReport drives the
// fingerprint verb against an empty temp dir and asserts the
// canonical report shape (ruleSet, fingerprint, oracleRules).
func TestOracleFilterFingerprintVerb_EmitsJSONReport(t *testing.T) {
	socket, state := startServerForTest(t)

	// Drop a token Kotlin file so CollectKotlinFiles has something
	// to walk; the fingerprint is stable for an empty marked-files
	// set, which is fine for this round-trip assertion.
	dir := state.root
	if err := os.WriteFile(filepath.Join(dir, "Foo.kt"), []byte("fun main() {}\n"), 0o644); err != nil {
		t.Fatalf("write Foo.kt: %v", err)
	}

	var got daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbOracleFilterFingerprint,
		daemon.OracleFilterFingerprintArgs{Paths: []string{dir}}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d (stderr=%q)", got.ExitCode, string(got.Stderr))
	}
	var report struct {
		RuleSet     string   `json:"ruleSet"`
		Fingerprint string   `json:"fingerprint"`
		OracleRules []string `json:"oracleRules"`
		Root        string   `json:"root"`
	}
	if err := json.Unmarshal(got.Stdout, &report); err != nil {
		t.Fatalf("decode report: %v\n%s", err, string(got.Stdout))
	}
	if report.RuleSet != "default" {
		t.Errorf("expected ruleSet=default, got %q", report.RuleSet)
	}
	if report.Fingerprint == "" {
		t.Errorf("expected non-empty fingerprint")
	}
}

// TestOracleFilterFingerprintVerb_AllRulesLabelsRuleSet flips
// AllRules and confirms the report's "ruleSet" string changes —
// guards against the daemon silently ignoring the AllRules arg.
func TestOracleFilterFingerprintVerb_AllRulesLabelsRuleSet(t *testing.T) {
	socket, state := startServerForTest(t)

	if err := os.WriteFile(filepath.Join(state.root, "Foo.kt"), []byte("fun main() {}\n"), 0o644); err != nil {
		t.Fatalf("write Foo.kt: %v", err)
	}

	var got daemon.MetaResult
	if err := daemon.Call(socket, daemon.VerbOracleFilterFingerprint,
		daemon.OracleFilterFingerprintArgs{Paths: []string{state.root}, AllRules: true}, &got); err != nil {
		t.Fatalf("call: %v", err)
	}
	if got.ExitCode != 0 {
		t.Fatalf("expected exit 0, got %d", got.ExitCode)
	}
	var report struct {
		RuleSet string `json:"ruleSet"`
	}
	if err := json.Unmarshal(got.Stdout, &report); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if report.RuleSet != "all-rules" {
		t.Errorf("expected ruleSet=all-rules with AllRules=true; got %q", report.RuleSet)
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

func lastLine(s string) string {
	s = strings.TrimRight(s, "\n")
	if i := strings.LastIndexByte(s, '\n'); i >= 0 {
		return s[i+1:]
	}
	return s
}
