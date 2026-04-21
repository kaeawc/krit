package main

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/pipeline"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

var binPath string

func TestMain(m *testing.M) {
	// Build the binary once for all integration tests.
	tmp, err := os.MkdirTemp("", "krit-integration-*")
	if err != nil {
		log.Fatal(err)
	}
	binPath = filepath.Join(tmp, "krit-test")
	cmd := exec.Command("go", "build", "-o", binPath, ".")
	cmd.Dir = "."
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		log.Fatalf("failed to build krit binary: %v", err)
	}

	code := m.Run()

	os.RemoveAll(tmp)
	os.Exit(code)
}

// runKrit executes the built binary with the given args.
// Returns stdout, stderr, and the exit code.
func runKrit(t *testing.T, args ...string) (string, string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			t.Fatalf("unexpected error running krit: %v", err)
		}
	}
	return stdout.String(), stderr.String(), exitCode
}

// writeTempKt creates a temp directory with a .kt file and returns the dir path.
func writeTempKt(t *testing.T, filename, content string) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, filename), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// --- Tests ---

func TestVersion(t *testing.T) {
	stdout, _, code := runKrit(t, "--version")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "krit") {
		t.Fatalf("expected version output to contain 'krit', got: %s", stdout)
	}
}

func TestBasicAnalysis_JSON(t *testing.T) {
	// UnusedVariable is active by default and easy to trigger.
	dir := writeTempKt(t, "Test.kt", "package test\n\nfun example() {\n    val x = 1\n}\n")

	stdout, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", dir)
	if code != 1 {
		t.Fatalf("expected exit 1 (findings), got %d", code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s", err, stdout)
	}
	findings, ok := result["findings"].([]interface{})
	if !ok || len(findings) == 0 {
		t.Fatalf("expected at least one finding, got: %s", stdout)
	}

	// Verify at least one finding mentions UnusedVariable
	found := false
	for _, f := range findings {
		fm := f.(map[string]interface{})
		if fm["rule"] == "UnusedVariable" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected an UnusedVariable finding in output")
	}
}

func TestExitCode_Clean(t *testing.T) {
	// A file with no violations should produce exit 0.
	// Disable InvalidPackageDeclaration since temp dirs never match package names.
	dir := writeTempKt(t, "Clean.kt", "package clean\n\nfun main() {\n    println(\"hello\")\n}\n")

	configContent := `naming:
  InvalidPackageDeclaration:
    active: false
`
	configPath := filepath.Join(dir, "krit.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", "--config", configPath, dir)
	if code != 0 {
		t.Fatalf("expected exit 0 for clean file, got %d\nstderr: %s", code, stderr)
	}
}

func TestExitCode_Error(t *testing.T) {
	// An invalid format should produce exit 2.
	dir := writeTempKt(t, "Any.kt", "package any\n")

	_, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-f", "badformat", dir)
	if code != 2 {
		t.Fatalf("expected exit 2 for unknown format, got %d", code)
	}
}

func TestFormatPlain(t *testing.T) {
	dir := writeTempKt(t, "Test.kt", "package test\n\nfun example() {\n    val x = 1\n}\n")

	stdout, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "plain", dir)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	// Plain output has file:line:col format
	if !strings.Contains(stdout, "Test.kt") {
		t.Fatalf("expected plain output to mention Test.kt, got: %s", stdout)
	}
	if !strings.Contains(stdout, "UnusedVariable") {
		t.Fatalf("expected plain output to mention UnusedVariable, got: %s", stdout)
	}
}

func TestFormatSARIF(t *testing.T) {
	dir := writeTempKt(t, "Test.kt", "package test\n\nfun example() {\n    val x = 1\n}\n")

	stdout, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "sarif", dir)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}

	var sarif map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &sarif); err != nil {
		t.Fatalf("invalid SARIF JSON: %v\nstdout: %s", err, stdout)
	}
	runs, ok := sarif["runs"].([]interface{})
	if !ok || len(runs) == 0 {
		t.Fatalf("expected SARIF runs array, got: %s", stdout)
	}
	run0 := runs[0].(map[string]interface{})
	results, ok := run0["results"].([]interface{})
	if !ok || len(results) == 0 {
		t.Fatalf("expected SARIF results, got: %s", stdout)
	}
}

func TestFormatCheckstyle(t *testing.T) {
	dir := writeTempKt(t, "Test.kt", "package test\n\nfun example() {\n    val x = 1\n}\n")

	stdout, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "checkstyle", dir)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}

	// Verify it parses as valid XML
	type Checkstyle struct {
		XMLName xml.Name `xml:"checkstyle"`
		File    []struct {
			Name  string `xml:"name,attr"`
			Error []struct {
				Line    string `xml:"line,attr"`
				Message string `xml:"message,attr"`
				Source  string `xml:"source,attr"`
			} `xml:"error"`
		} `xml:"file"`
	}
	var cs Checkstyle
	if err := xml.Unmarshal([]byte(stdout), &cs); err != nil {
		t.Fatalf("invalid Checkstyle XML: %v\nstdout: %s", err, stdout)
	}
	if len(cs.File) == 0 {
		t.Fatalf("expected at least one file element in checkstyle output")
	}
}

func TestConfigDisablesRule(t *testing.T) {
	dir := writeTempKt(t, "Test.kt", "package test\n\nfun example() {\n    val x = 1\n}\n")

	// Write a config that disables UnusedVariable
	configContent := `style:
  UnusedVariable:
    active: false
`
	configPath := filepath.Join(dir, "krit.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, _, _ := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", "--config", configPath, dir)

	// Parse findings and ensure UnusedVariable is not present
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v\nstdout: %s", err, stdout)
	}
	findings, _ := result["findings"].([]interface{})
	for _, f := range findings {
		fm := f.(map[string]interface{})
		if fm["rule"] == "UnusedVariable" {
			t.Fatalf("UnusedVariable should be disabled by config, but was reported")
		}
	}
}

func TestRunActiveIconChecksColumnsMatchesUnderlyingRules(t *testing.T) {
	gifIcon := &android.IconFile{
		Path:    "res/drawable-mdpi/anim.gif",
		Name:    "anim",
		Density: "mdpi",
		Format:  "gif",
		Size:    2048,
	}
	pngIcon := &android.IconFile{
		Path:    "res/drawable-mdpi/large.png",
		Name:    "large",
		Density: "mdpi",
		Format:  "png",
		Size:    12 * 1024,
	}
	idx := &android.IconIndex{
		Densities: map[string][]*android.IconFile{
			"mdpi": {gifIcon, pngIcon},
		},
		Icons: []*android.IconFile{gifIcon, pngIcon},
	}
	activeNames := map[string]bool{
		"GifUsage":                 true,
		"ConvertToWebp":            true,
		"IconMissingDensityFolder": true,
	}

	columns := pipeline.RunActiveIconChecksColumns(idx, activeNames)
	got := columns.Findings()
	wantCollector := scanner.NewFindingCollector(0)
	rules.CheckGifUsage(idx, wantCollector)
	rules.CheckConvertToWebp(idx, wantCollector)
	rules.CheckIconMissingDensityFolder(idx, wantCollector)
	normalizedWant := wantCollector.Columns().Findings()

	if !reflect.DeepEqual(got, normalizedWant) {
		t.Fatalf("icon columns mismatch:\nwant: %#v\ngot:  %#v", normalizedWant, got)
	}
	wrapperCols := pipeline.RunActiveIconChecksColumns(idx, activeNames)
	if wrapper := wrapperCols.Findings(); !reflect.DeepEqual(wrapper, normalizedWant) {
		t.Fatalf("icon wrapper mismatch:\nwant: %#v\ngot:  %#v", normalizedWant, wrapper)
	}
}

func TestRunAndroidProjectAnalysisColumns_EmptyProject(t *testing.T) {
	project := &android.AndroidProject{}
	result, err := (pipeline.AndroidPhase{}).Run(context.Background(), pipeline.AndroidInput{Project: project})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Findings.Len() != 0 {
		t.Fatalf("expected no findings for empty project, got %d", result.Findings.Len())
	}
}

func TestFixApplied(t *testing.T) {
	// NewLineAtEndOfFile is active by default and fixable at cosmetic level.
	// Create a file that does NOT end with a newline.
	dir := writeTempKt(t, "Fix.kt", "package test\n\nfun example() {\n    println(\"hi\")\n}")
	filePath := filepath.Join(dir, "Fix.kt")

	// Suppress InvalidPackageDeclaration so the only issue is NewLineAtEndOfFile.
	// Actually, use a config to disable all rules except NewLineAtEndOfFile.
	configContent := `naming:
  InvalidPackageDeclaration:
    active: false
`
	configPath := filepath.Join(dir, "krit.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "--fix", "--fix-level", "cosmetic", "--config", configPath, dir)
	// --fix should exit 0 when all findings are fixable, or 1 if unfixable remain
	if code == 2 {
		t.Fatalf("unexpected error exit code 2")
	}

	// Read the file back and verify it now ends with a newline
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) == 0 || content[len(content)-1] != '\n' {
		t.Fatalf("expected file to end with newline after fix, got: %q", string(content[max(0, len(content)-20):]))
	}
}

func TestDryRun(t *testing.T) {
	// File missing trailing newline triggers NewLineAtEndOfFile (fixable, cosmetic).
	dir := writeTempKt(t, "DryRun.kt", "package test\n\nfun example() {\n    println(\"hi\")\n}")
	filePath := filepath.Join(dir, "DryRun.kt")

	originalContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}

	stdout, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "--dry-run", "--fix-level", "cosmetic", dir)
	// dry-run lists files and then exits; may be 0 or 1 depending on unfixable findings
	if code == 2 {
		t.Fatalf("unexpected error exit code 2")
	}
	if !strings.Contains(stdout, "DryRun.kt") {
		t.Fatalf("expected dry-run output to list DryRun.kt, got: %s", stdout)
	}

	// File should NOT be modified
	afterContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	if string(afterContent) != string(originalContent) {
		t.Fatalf("dry-run should not modify the file")
	}
}

func TestGenerateSchema(t *testing.T) {
	stdout, _, code := runKrit(t, "--generate-schema")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}

	var schema map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &schema); err != nil {
		t.Fatalf("--generate-schema output is not valid JSON: %v", err)
	}
	// JSON Schema should have a "type" or "$schema" key
	if _, ok := schema["type"]; !ok {
		if _, ok := schema["$schema"]; !ok {
			t.Fatalf("expected JSON Schema output with 'type' or '$schema' key, got keys: %v", keysOf(schema))
		}
	}
}

func TestFixBinaryDryRun(t *testing.T) {
	// --fix-binary with --dry-run should not crash even on a dir with no binary files.
	dir := writeTempKt(t, "NoBinary.kt", "package test\n\nfun main() {\n    println(\"hi\")\n}\n")

	_, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "--fix", "--fix-binary", "--dry-run", dir)
	// Should not crash; exit 0 is expected for clean + no binary fixes needed.
	if code == 2 {
		t.Fatalf("--fix-binary --dry-run should not produce error exit code 2")
	}
}

func TestRemoveDeadCodeDryRun(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Dead.kt"), []byte("fun unusedHelper() = 42\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "--report", "json", "--remove-dead-code", "--dry-run", dir)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\nstderr: %s", code, stderr)
	}

	var report struct {
		Action  string `json:"action"`
		Summary struct {
			Declarations int `json:"declarations"`
			Files        int `json:"files"`
			Kinds        []struct {
				Kind  string `json:"kind"`
				Count int    `json:"count"`
			} `json:"kinds"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid dead-code removal JSON: %v\nstdout: %s", err, stdout)
	}
	if report.Action != "dry-run" {
		t.Fatalf("expected dry-run action, got %q", report.Action)
	}
	if report.Summary.Declarations != 0 || report.Summary.Files != 0 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}
	if len(report.Summary.Kinds) != 0 {
		t.Fatalf("expected no removable kind summary, got %+v", report.Summary.Kinds)
	}
}

func TestRemoveDeadCodeApplyNoop(t *testing.T) {
	dir := t.TempDir()
	apiPath := filepath.Join(dir, "Api.kt")
	if err := os.WriteFile(apiPath, []byte("fun keep() = 1\nfun dead() = 2\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Main.kt"), []byte("fun main() { keep() }\n"), 0644); err != nil {
		t.Fatal(err)
	}
	original, err := os.ReadFile(apiPath)
	if err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "--report", "json", "--remove-dead-code", dir)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\nstderr: %s\nstdout: %s", code, stderr, stdout)
	}

	var report struct {
		Action  string `json:"action"`
		Summary struct {
			Declarations int `json:"declarations"`
			Files        int `json:"files"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(stdout), &report); err != nil {
		t.Fatalf("invalid dead-code removal JSON: %v\nstdout: %s", err, stdout)
	}
	if report.Action != "apply" {
		t.Fatalf("expected apply action, got %q", report.Action)
	}
	if report.Summary.Declarations != 0 || report.Summary.Files != 0 {
		t.Fatalf("unexpected summary: %+v", report.Summary)
	}

	content, err := os.ReadFile(apiPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != string(original) {
		t.Fatalf("expected no-op apply to leave file unchanged, got: %s", string(content))
	}
}

func TestNoFilesFound(t *testing.T) {
	// An empty directory with no .kt files should exit 0.
	dir := t.TempDir()

	_, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", dir)
	if code != 0 {
		t.Fatalf("expected exit 0 for no files, got %d", code)
	}
}

func TestListRules(t *testing.T) {
	stdout, _, code := runKrit(t, "--list-rules")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "TrailingWhitespace") {
		t.Fatalf("expected --list-rules to include TrailingWhitespace")
	}
	if !strings.Contains(stdout, "Total:") {
		t.Fatalf("expected --list-rules to include summary line")
	}
}

func TestOutputToFile(t *testing.T) {
	dir := writeTempKt(t, "Out.kt", "package test\n\nfun example() {\n    val x = 1   \n}\n")
	outFile := filepath.Join(t.TempDir(), "results.json")

	_, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", "-o", outFile, dir)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}

	content, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(content, &result); err != nil {
		t.Fatalf("output file is not valid JSON: %v", err)
	}
}

func TestReportFlagAlias(t *testing.T) {
	// --report should work as alias for -f
	dir := writeTempKt(t, "Report.kt", "package test\n\nfun example() {\n    val x = 1\n}\n")

	stdout, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "--report", "plain", dir)
	if code != 1 {
		t.Fatalf("expected exit 1, got %d", code)
	}
	if !strings.Contains(stdout, "UnusedVariable") {
		t.Fatalf("expected --report plain to produce plain output with rule name, got: %s", stdout)
	}
}

// keysOf returns the keys of a map for diagnostic output.
func keysOf(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestPlaygroundWebService(t *testing.T) {
	out, err := exec.Command(binPath, "-f", "json", "-no-type-inference", "-no-type-oracle", "-q", "../../playground/kotlin-webservice/").CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Expected - has findings
		} else {
			t.Fatalf("unexpected error: %v\n%s", err, out)
		}
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	findings, ok := result["findings"].([]interface{})
	if !ok {
		t.Fatal("missing findings array")
	}
	if len(findings) < 20 {
		t.Errorf("expected 20+ findings on playground/kotlin-webservice, got %d", len(findings))
	}
}

func TestPlaygroundAndroidApp(t *testing.T) {
	out, err := exec.Command(binPath, "-f", "json", "-no-type-inference", "-no-type-oracle", "-q", "../../playground/android-app/").CombinedOutput()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// Expected
		} else {
			t.Fatalf("unexpected error: %v\n%s", err, out)
		}
	}
	var result map[string]interface{}
	if err := json.Unmarshal(out, &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	findings, ok := result["findings"].([]interface{})
	if !ok {
		t.Fatal("missing findings array")
	}
	if len(findings) < 30 {
		t.Errorf("expected 30+ findings on playground/android-app, got %d", len(findings))
	}
}

func TestAllRulesFlag(t *testing.T) {
	dir := writeTempKt(t, "Magic.kt", "package test\n\nfun example() {\n    val x = 42\n    println(x)\n}\n")

	// Run without --all-rules
	stdoutDefault, _, _ := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", dir)
	var resultDefault map[string]interface{}
	if err := json.Unmarshal([]byte(stdoutDefault), &resultDefault); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	defaultRuleCount := resultDefault["rules"]

	// Run with --all-rules
	stdoutAll, _, _ := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", "--all-rules", dir)
	var resultAll map[string]interface{}
	if err := json.Unmarshal([]byte(stdoutAll), &resultAll); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	allRuleCount := resultAll["rules"]

	defaultCount, ok1 := defaultRuleCount.(float64)
	allCount, ok2 := allRuleCount.(float64)
	if !ok1 || !ok2 {
		t.Fatalf("expected numeric rules, got default=%v all=%v", defaultRuleCount, allRuleCount)
	}
	if allCount <= defaultCount {
		t.Fatalf("expected --all-rules to activate more rules than default (%v vs %v)", allCount, defaultCount)
	}
}

func TestWarningsAsErrorsFlag(t *testing.T) {
	dir := writeTempKt(t, "Warn.kt", "package test\n\nfun example() {\n    val x = 1\n}\n")

	stdout, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", "--warnings-as-errors", dir)
	if code != 1 {
		t.Fatalf("expected exit 1 with --warnings-as-errors, got %d", code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	findings, ok := result["findings"].([]interface{})
	if !ok || len(findings) == 0 {
		t.Fatal("expected at least one finding")
	}
	for _, f := range findings {
		fm := f.(map[string]interface{})
		sev, _ := fm["severity"].(string)
		if sev == "warning" {
			t.Fatalf("expected all warnings elevated to error, but found severity=warning for rule %v", fm["rule"])
		}
	}
}

func TestDisableRulesFlag(t *testing.T) {
	// MagicNumber is active by default; disable it via --disable-rules
	dir := writeTempKt(t, "Disable.kt", "package test\n\nfun example() {\n    val x = 1\n}\n")

	stdout, _, _ := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", "--disable-rules", "UnusedVariable", dir)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	findings, _ := result["findings"].([]interface{})
	for _, f := range findings {
		fm := f.(map[string]interface{})
		if fm["rule"] == "UnusedVariable" {
			t.Fatalf("UnusedVariable should be disabled via --disable-rules but was found in output")
		}
	}
}

func TestEnableRulesFlag(t *testing.T) {
	// UndocumentedPublicFunction is inactive by default; enable it via --enable-rules
	dir := writeTempKt(t, "Enable.kt", "package test\n\nfun publicFunction() {\n    println(\"hello\")\n}\n")

	stdout, _, code := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", "--enable-rules", "UndocumentedPublicFunction", dir)
	if code != 1 {
		t.Fatalf("expected exit 1 with enabled rule finding, got %d", code)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	findings, ok := result["findings"].([]interface{})
	if !ok || len(findings) == 0 {
		t.Fatal("expected at least one finding")
	}
	found := false
	for _, f := range findings {
		fm := f.(map[string]interface{})
		if fm["rule"] == "UndocumentedPublicFunction" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected UndocumentedPublicFunction finding after --enable-rules, got: %s", stdout)
	}
}

func TestPerfFlag(t *testing.T) {
	dir := writeTempKt(t, "Perf.kt", "package test\n\nfun main() {\n    println(\"hi\")\n}\n")

	stdout, _, _ := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", "--perf", dir)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	perfData, ok := result["perfTiming"]
	if !ok {
		t.Fatalf("expected 'perfTiming' key in JSON output when --perf is used, got keys: %v", keysOf(result))
	}
	perfArr, ok := perfData.([]interface{})
	if !ok || len(perfArr) == 0 {
		t.Fatalf("expected perfTiming to be a non-empty array, got: %v", perfData)
	}
}

func TestPerfRulesFlag_JSON(t *testing.T) {
	dir := writeTempKt(t, "PerfRules.kt", "package test\n\nfun example() {\n    val x = 1\n}\n")

	stdout, _, _ := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", "--perf-rules", dir)

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	perfData, ok := result["perfTiming"].([]interface{})
	if !ok || len(perfData) == 0 {
		t.Fatalf("expected --perf-rules to imply perfTiming, got: %v", result["perfTiming"])
	}
	ruleData, ok := result["perfRuleStats"].([]interface{})
	if !ok || len(ruleData) == 0 {
		t.Fatalf("expected perfRuleStats in JSON output, got: %v", result["perfRuleStats"])
	}
	first := ruleData[0].(map[string]interface{})
	for _, key := range []string{"rule", "family", "invocations", "durationNs", "avgNs", "sharePct"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("expected perfRuleStats row to include %q, got: %#v", key, first)
		}
	}
}

func TestListExperiments_JSON(t *testing.T) {
	stdout, _, code := runKrit(t, "--list-experiments")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s", err, stdout)
	}
	experiments, ok := result["experiments"].([]interface{})
	if !ok || len(experiments) == 0 {
		t.Fatalf("expected experiments list, got %v", result)
	}
}

func TestExperimentFlag_RecordedInJSON(t *testing.T) {
	dir := writeTempKt(t, "Perf.kt", "package test\n\nfun main() {\n    val x = 1\n}\n")
	stdout, _, _ := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-q", "-f", "json", "--experiment", "magic-number-ancestor-scan", dir)
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid JSON output: %v\nstdout: %s", err, stdout)
	}
	experiments, ok := result["experiments"].([]interface{})
	if !ok || len(experiments) != 1 || experiments[0] != "magic-number-ancestor-scan" {
		t.Fatalf("expected experiment to be recorded, got %v", result["experiments"])
	}
}

func TestExperimentMatrix_BaselineAndSingles(t *testing.T) {
	dir := writeTempKt(t, "Perf.kt", "package test\n\nfun main() {\n    val x = 1\n}\n")
	stdout, _, code := runKrit(t,
		"--no-cache", "--no-type-inference", "--no-type-oracle",
		"--perf", "-q", "-f", "json",
		"--experiment-candidates", "magic-number-ancestor-scan,unnecessary-safe-call-local-nullability",
		"--experiment-matrix", "baseline,singles",
		"--experiment-runs", "1",
		dir,
	)
	if code != 0 {
		t.Fatalf("expected exit 0, got %d\nstdout: %s", code, stdout)
	}
	var result struct {
		Cases []struct {
			Name    string   `json:"name"`
			Enabled []string `json:"enabled"`
			Samples []struct {
				PerfBucketsMs map[string]int64 `json:"perfBucketsMs"`
			} `json:"samples"`
		} `json:"cases"`
	}
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("invalid matrix JSON output: %v\nstdout: %s", err, stdout)
	}
	if len(result.Cases) != 3 {
		t.Fatalf("expected 3 matrix cases, got %d", len(result.Cases))
	}
	if result.Cases[0].Name != "baseline" {
		t.Fatalf("expected first case to be baseline, got %s", result.Cases[0].Name)
	}
	if len(result.Cases[0].Samples) != 1 {
		t.Fatalf("expected one sample per case, got %d", len(result.Cases[0].Samples))
	}
	if _, ok := result.Cases[0].Samples[0].PerfBucketsMs["ruleExecution"]; !ok {
		t.Fatalf("expected perf buckets in matrix sample, got %#v", result.Cases[0].Samples[0].PerfBucketsMs)
	}
}

func TestVerboseFlag(t *testing.T) {
	dir := writeTempKt(t, "Verb.kt", "package test\n\nfun main() {\n    println(\"hi\")\n}\n")

	_, stderr, _ := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", "-f", "json", "-v", dir)

	if !strings.Contains(stderr, "verbose:") {
		t.Fatalf("expected stderr to contain 'verbose:' with -v flag, got: %s", stderr)
	}
}

func TestInitFlag(t *testing.T) {
	dir := t.TempDir()

	// Run --init from within the temp dir
	cmd := exec.Command(binPath, "--init")
	cmd.Dir = dir
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		t.Fatalf("--init failed: %v\nstderr: %s", err, stderr.String())
	}

	// Verify krit.yml was created
	configPath := filepath.Join(dir, "krit.yml")
	content, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("expected krit.yml to be created: %v", err)
	}
	if !strings.Contains(string(content), "MagicNumber") {
		t.Fatalf("expected starter config to mention MagicNumber, got: %s", string(content))
	}
	if !strings.Contains(stdout.String(), "Created krit.yml") {
		t.Fatalf("expected stdout to confirm creation, got: %s", stdout.String())
	}
}

func TestDoctorFlag(t *testing.T) {
	stdout, _, code := runKrit(t, "--doctor")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "krit doctor") {
		t.Fatalf("expected output to contain 'krit doctor', got: %s", stdout)
	}
	if !strings.Contains(stdout, "version") {
		t.Fatalf("expected output to contain 'version', got: %s", stdout)
	}
	if !strings.Contains(stdout, "rules:") {
		t.Fatalf("expected output to contain rule count, got: %s", stdout)
	}
}

func TestValidateConfigFlag_Valid(t *testing.T) {
	dir := t.TempDir()
	configContent := `style:
  MagicNumber:
    active: true
`
	configPath := filepath.Join(dir, "krit.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runKrit(t, "--validate-config", "--config", configPath)
	if code != 0 {
		t.Fatalf("expected exit 0 for valid config, got %d\nstderr: %s", code, stderr)
	}
	if !strings.Contains(stderr, "validation passed") {
		t.Fatalf("expected 'validation passed' in stderr, got: %s", stderr)
	}
}

func TestValidateConfigFlag_Invalid(t *testing.T) {
	dir := t.TempDir()
	// Use an unknown rule name which should trigger a validation warning/error
	configContent := `style:
  CompletelyBogusRuleName12345:
    active: true
`
	configPath := filepath.Join(dir, "krit.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	_, stderr, code := runKrit(t, "--validate-config", "--config", configPath)
	// Should report issues (exit 0 for warnings, exit 1 for errors)
	if code == 2 {
		t.Fatalf("unexpected error exit code 2\nstderr: %s", stderr)
	}
	if !strings.Contains(stderr, "CompletelyBogusRuleName12345") && !strings.Contains(stderr, "validation") {
		t.Fatalf("expected validation output mentioning the bogus rule or validation status, got: %s", stderr)
	}
}

func TestCompletionsFlag_Bash(t *testing.T) {
	stdout, _, code := runKrit(t, "--completions", "bash")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	// Bash completions typically start with a comment or function definition
	if !strings.Contains(stdout, "complete") && !strings.Contains(stdout, "bash") && !strings.Contains(stdout, "krit") {
		t.Fatalf("expected bash completion script, got: %.200s", stdout)
	}
}

func TestCompletionsFlag_Zsh(t *testing.T) {
	stdout, _, code := runKrit(t, "--completions", "zsh")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "compdef") && !strings.Contains(stdout, "zsh") && !strings.Contains(stdout, "krit") {
		t.Fatalf("expected zsh completion script, got: %.200s", stdout)
	}
}

func TestCompletionsFlag_Fish(t *testing.T) {
	stdout, _, code := runKrit(t, "--completions", "fish")
	if code != 0 {
		t.Fatalf("expected exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "complete") && !strings.Contains(stdout, "fish") && !strings.Contains(stdout, "krit") {
		t.Fatalf("expected fish completion script, got: %.200s", stdout)
	}
}

func TestCompletionsFlag_Invalid(t *testing.T) {
	_, _, code := runKrit(t, "--completions", "powershell")
	if code != 1 {
		t.Fatalf("expected exit 1 for unsupported shell, got %d", code)
	}
}

// --- --baseline flag tests ---

func TestBaselineFlag_SuppressesKnownIssues(t *testing.T) {
	dir := t.TempDir()
	ktFile := filepath.Join(dir, "app.kt")
	os.WriteFile(ktFile, []byte("package test\nfun example() {\n    val x = 42\n}\n"), 0644)

	// First run: get findings count without baseline
	stdout1, _, _ := runKrit(t, "--no-cache", "--no-type-inference", "--no-type-oracle", dir)
	var report1 struct {
		Summary struct{ Total int } `json:"summary"`
	}
	json.Unmarshal([]byte(stdout1), &report1)
	if report1.Summary.Total == 0 {
		t.Skip("no findings to baseline")
	}

	// Create baseline from current findings
	baselinePath := filepath.Join(dir, "baseline.xml")
	runKrit(t, "--create-baseline", baselinePath, "--no-cache", "--no-type-inference", "--no-type-oracle", dir)

	if _, err := os.Stat(baselinePath); err != nil {
		t.Fatalf("baseline file not created: %v", err)
	}

	// Second run: with baseline, should suppress known issues
	stdout2, _, _ := runKrit(t, "--baseline", baselinePath, "--no-cache", "--no-type-inference", "--no-type-oracle", dir)
	var report2 struct {
		Summary struct{ Total int } `json:"summary"`
	}
	json.Unmarshal([]byte(stdout2), &report2)

	if report2.Summary.Total >= report1.Summary.Total {
		t.Errorf("baseline should suppress findings: got %d with baseline, %d without", report2.Summary.Total, report1.Summary.Total)
	}
}

func TestCreateBaselineFlag_CreatesValidXML(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "app.kt"), []byte("package test\nfun f() { val x = 42 }\n"), 0644)

	baselinePath := filepath.Join(dir, "baseline.xml")
	runKrit(t, "--create-baseline", baselinePath, "--no-cache", "--no-type-inference", "--no-type-oracle", dir)

	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Fatalf("failed to read baseline: %v", err)
	}

	// Verify valid XML
	type SmellBaseline struct {
		XMLName xml.Name `xml:"SmellBaseline"`
	}
	var bl SmellBaseline
	if err := xml.Unmarshal(data, &bl); err != nil {
		t.Fatalf("invalid baseline XML: %v", err)
	}
}

func TestBaselineAuditSubcommand_FindsDeadEntries(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "app.kt")
	configPath := filepath.Join(dir, "krit.yml")
	if err := os.WriteFile(sourcePath, []byte("package test\nfun f() {\n    println(42)\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(configPath, []byte("naming:\n  InvalidPackageDeclaration:\n    active: false\n"), 0644); err != nil {
		t.Fatal(err)
	}

	baselinePath := filepath.Join(dir, "baseline.xml")
	baselineXML := `<?xml version="1.0" encoding="UTF-8"?>
<SmellBaseline>
  <ManuallySuppressedIssues>
    <ID>OldRuleName:missing.kt:fun gone()</ID>
  </ManuallySuppressedIssues>
  <CurrentIssues>
    <ID>LongMethod:app.kt:$LongMethod$msg</ID>
    <ID>LongMethod:missing.kt:$LongMethod$msg</ID>
  </CurrentIssues>
</SmellBaseline>`
	if err := os.WriteFile(baselinePath, []byte(baselineXML), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runKrit(t, "baseline-audit", "-f", "plain", "--baseline", baselinePath, "--config", configPath, "--no-cache", "--no-type-inference", "--no-type-oracle", dir)
	if code != 0 {
		t.Fatalf("baseline-audit failed: exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "Baseline audit") {
		t.Fatalf("expected baseline audit header, got: %s", stdout)
	}
	if !strings.Contains(stdout, "missing.kt :: OldRuleName (rule deleted)") {
		t.Fatalf("expected deleted-rule entry, got: %s", stdout)
	}
	if !strings.Contains(stdout, "missing.kt :: LongMethod (file no longer exists)") {
		t.Fatalf("expected missing-file entry, got: %s", stdout)
	}
	if !strings.Contains(stdout, "app.kt ::") || !strings.Contains(stdout, "(finding no longer exists)") {
		t.Fatalf("expected dead finding entry, got: %s", stdout)
	}
}

func TestHarvestSubcommand_WritesFixture(t *testing.T) {
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "Sample.kt")
	outPath := filepath.Join(dir, "fixtures", "MagicNumber-extra.kt")
	if err := os.WriteFile(sourcePath, []byte("package test\n\nfun answer() {\n    println(42)\n}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runKrit(t, "harvest", sourcePath+":4", "--rule", "MagicNumber", "--out", outPath)
	if code != 0 {
		t.Fatalf("harvest failed: exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "Harvested MagicNumber") {
		t.Fatalf("expected harvest summary, got: %s", stdout)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read harvested fixture: %v", err)
	}
	if !strings.Contains(string(data), "42") {
		t.Fatalf("expected harvested fixture to contain 42, got %q", string(data))
	}
}

func TestRenameSubcommand_PlansAndReturnsTodo(t *testing.T) {
	dir := t.TempDir()
	oldNamePath := filepath.Join(dir, "src", "main", "kotlin", "com", "example", "OldName.kt")
	featurePath := filepath.Join(dir, "src", "main", "kotlin", "com", "example", "Feature.kt")
	if err := os.MkdirAll(filepath.Dir(oldNamePath), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldNamePath, []byte("package com.example\n\nclass OldName\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(featurePath, []byte("package com.example\n\nfun use(value: OldName) = value\n"), 0644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, code := runKrit(t, "rename", "com.example.OldName", "com.example.NewName", dir)
	if code != 2 {
		t.Fatalf("rename failed: exit=%d stderr=%s stdout=%s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "Rename planning found") {
		t.Fatalf("expected rename planning summary, got: %s", stdout)
	}
	if !strings.Contains(stderr, "rename apply is not implemented yet") {
		t.Fatalf("expected TODO stderr, got: %s", stderr)
	}
}
