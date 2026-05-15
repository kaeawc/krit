package scan

import (
	"context"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/cacheutil"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestScanRunMatchesAnalyzeProjectVerb pins the CLI-vs-RunProject
// byte-equal contract: scan.Run (with FIR, profiling, cross-file cache,
// and the oracle disabled — the subset RunProject already covers) and
// pipeline.RunProject called directly must produce identical
// findings JSON after stripping the perf/caches/timing/version
// envelope. A divergence means the two paths have drifted on rule
// output, file enumeration, suppression, or format dispatch.
func TestScanRunMatchesAnalyzeProjectVerb(t *testing.T) {
	dir := t.TempDir()
	writeContractFile(t, filepath.Join(dir, "Foo.kt"),
		"package demo\n\nimport kotlin.io.println\n\nclass Foo {\n    fun greet() = println(\"hi\")\n}\n")
	writeContractFile(t, filepath.Join(dir, "Bar.kt"),
		"package demo\n\nfun bar(unused: Int): Int { return 42 }\n")

	cliJSON := runScanCLI(t, []string{
		"krit",
		"--no-cross-file-cache",
		"--no-fir",
		"--no-type-oracle",
		"-f", "json",
		dir,
	})

	directJSON := runRunProjectDirect(t, dir)

	cli := stripVariableFields(t, cliJSON)
	direct := stripVariableFields(t, directJSON)

	// Guard against vacuous equality: both producing empty findings
	// arrays would pass the DeepEqual below but prove nothing about
	// the shared pipeline. The fixture is chosen to produce at least
	// one finding from the default rule set.
	if findings, ok := cli["findings"].([]any); !ok || len(findings) == 0 {
		t.Fatalf("CLI produced no findings; test is vacuous. cli=%s", mustPretty(t, cli))
	}

	if !reflect.DeepEqual(cli, direct) {
		t.Errorf("CLI output diverges from direct RunProject after strip\n--- cli ---\n%s\n--- direct ---\n%s",
			mustPretty(t, cli), mustPretty(t, direct))
	}
}

// runScanCLI invokes scan.Run() in-process with the supplied argv.
// It isolates global state (flag.CommandLine, os.Args, os.Stdout/Stderr,
// BaselineAuditVerb) and returns the captured stdout bytes.
func runScanCLI(t *testing.T, argv []string) []byte {
	t.Helper()

	// Save and restore every global scan.Run reaches into.
	savedArgs := os.Args
	savedFlagSet := flag.CommandLine
	savedStdout := os.Stdout
	savedStderr := os.Stderr
	savedBaselineVerb := BaselineAuditVerb
	t.Cleanup(func() {
		os.Args = savedArgs
		flag.CommandLine = savedFlagSet
		os.Stdout = savedStdout
		os.Stderr = savedStderr
		BaselineAuditVerb = savedBaselineVerb
	})

	os.Args = argv
	flag.CommandLine = flag.NewFlagSet(argv[0], flag.ContinueOnError)
	BaselineAuditVerb = false

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout = stdoutW
	os.Stderr = stderrW

	// Run scan.Run in a goroutine so we can drain stderr in parallel.
	// Without parallel drains the OS pipe buffer can fill and block
	// scan.Run's verbose writes.
	doneStderr := make(chan struct{})
	go func() {
		_, _ = io.Copy(io.Discard, stderrR)
		close(doneStderr)
	}()

	code := Run()
	_ = stdoutW.Close()
	_ = stderrW.Close()

	stdoutBytes, err := io.ReadAll(stdoutR)
	if err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	<-doneStderr

	// scan.Run returns 1 when there are findings, 0 when clean, 2 on
	// error. Anything other than 0/1 means the run failed before
	// emitting JSON.
	if code != 0 && code != 1 {
		t.Fatalf("scan.Run exit=%d (stdout=%q)", code, string(stdoutBytes))
	}
	return stdoutBytes
}

// runRunProjectDirect invokes pipeline.RunProject against the same
// fixture root with a config + rule set that mirrors what scan.Run
// constructs for the chosen flag set.
func runRunProjectDirect(t *testing.T, root string) []byte {
	t.Helper()

	cfg := config.NewConfig()
	rules.ApplyConfig(cfg)
	activeRules := rules.ActiveRulesV2(map[string]bool{}, map[string]bool{}, false, false, false)

	repoDir := oracle.FindRepoDir([]string{root})
	if repoDir == "" {
		repoDir = root
	}
	pc, err := scanner.NewParseCacheWithCap(repoDir, cacheutil.DefaultParseCacheCapBytes)
	if err != nil {
		t.Fatalf("ParseCache: %v", err)
	}
	t.Cleanup(func() { _ = pc.Close() })

	res, err := pipeline.RunProject(context.Background(), pipeline.ProjectInput{
		Args: pipeline.ProjectArgs{
			Config:      cfg,
			Paths:       []string{root},
			ActiveRules: activeRules,
			Format:      "json",
			Version:     "test",
		},
		Host: pipeline.ProjectHostState{ParseCache: pc},
	})
	if err != nil {
		t.Fatalf("RunProject: %v", err)
	}
	return res.JSON
}

// stripVariableFields decodes the JSON envelope, removes fields that
// legitimately differ between the two paths (perf tree, cache stats,
// wall-clock timestamps, version stamp), and returns the canonical
// map. The findings array stays — that's the load-bearing part of the
// contract.
func stripVariableFields(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	if len(raw) == 0 {
		t.Fatalf("empty JSON")
	}
	var out map[string]any
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatalf("unmarshal: %v\nraw=%s", err, raw)
	}
	for _, key := range []string{
		"durationMs", "wall_seconds", "wallSeconds",
		"perf", "perfTimings", "perfRuleStats",
		"cache", "caches", "cacheStats", "cacheBudget",
		"startTime", "timing", "timings",
		// version stamp comes from a compile-time var on the CLI side
		// and is caller-provided on the direct side. The contract is
		// about findings, not envelope metadata.
		"version", "kritVersion",
	} {
		delete(out, key)
	}
	if findings, ok := out["findings"].([]any); ok {
		for _, f := range findings {
			if m, ok := f.(map[string]any); ok {
				delete(m, "timeMs")
				delete(m, "tookNs")
			}
		}
	}
	return out
}

func mustPretty(t *testing.T, v any) string {
	t.Helper()
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(b)
}

func writeContractFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
