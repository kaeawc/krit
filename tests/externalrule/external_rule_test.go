// Package externalrule_test exercises examples/external-rule end-to-end.
//
// Gates on prerequisites being present (java on PATH, krit-rule-api in
// ~/.m2, locatable krit-types.jar) and t.Skips otherwise so `make
// integration` stays green on developer machines without a JVM. The
// dedicated `external-rule-example` CI job stages the prerequisites
// and exercises the test for real.
package externalrule_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/oracle"
)

const (
	ruleID              = "example.NoPrintln"
	expectedPrintlnLine = 5 // samples/.../Greeter.kt: line 5 holds the println(
)

func TestExternalRuleExample_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping external-rule integration test in -short mode")
	}
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("skipping: `java` not on PATH (JDK 21+ required)")
	}

	repoRoot := repoRoot(t)
	exampleDir := filepath.Join(repoRoot, "examples", "external-rule")

	kritRuleApiVersion, ok := locateLocalKritRuleApi()
	if !ok {
		t.Skip("skipping: dev.jasonpearson.krit:krit-rule-api not in ~/.m2/repository " +
			"(run `./gradlew -p tools/krit-rule-api publishToMavenLocal -PkritVersion=<v>`)")
	}

	if oracle.FindJar(nil) == "" {
		t.Skip("skipping: krit-types.jar not located. Set KRIT_TYPES_JAR or run " +
			"`./gradlew -p tools/krit-types shadowJar`")
	}

	kritBin := buildKritBinary(t, repoRoot)
	jarPath := buildExampleRuleJar(t, exampleDir, kritRuleApiVersion)
	samplesDir := filepath.Join(exampleDir, "samples")
	positiveFile, err := filepath.Abs(filepath.Join(
		samplesDir, "src", "main", "kotlin", "com", "example", "positive", "Greeter.kt"))
	if err != nil {
		t.Fatalf("absolutize positive sample: %v", err)
	}

	t.Run("list-rules shows the plugin rule", func(t *testing.T) {
		stdout := runKrit(t, kritBin, repoRoot,
			"--list-rules",
			"--custom-rule-jars", jarPath,
			samplesDir,
		)
		if !strings.Contains(stdout, ruleID) {
			t.Fatalf("expected --list-rules to include %q; got:\n%s", ruleID, stdout)
		}
		// printListRules at early_exits.go:380 renders plugin-loaded
		// rules with a leading `  P  ` column — match loosely so column
		// width tweaks don't snap the assertion.
		if !strings.Contains(stdout, " P ") {
			t.Fatalf("expected --list-rules to include the `P ` plugin marker; got:\n%s", stdout)
		}
	})

	t.Run("findings only on positive sample", func(t *testing.T) {
		stdout := runKrit(t, kritBin, repoRoot,
			"--custom-rule-jars", jarPath,
			"--daemon",
			"-f", "json",
			"--no-cache",
			"-q",
			samplesDir,
		)

		var payload struct {
			Findings []struct {
				File string `json:"file"`
				Line int    `json:"line"`
				Rule string `json:"rule"`
			} `json:"findings"`
		}
		if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
			t.Fatalf("invalid JSON output: %v\n--- stdout ---\n%s", err, stdout)
		}

		var positiveLines []int
		for _, finding := range payload.Findings {
			if finding.Rule != ruleID {
				continue
			}
			abs, err := filepath.Abs(finding.File)
			if err != nil {
				t.Fatalf("absolutize finding path %q: %v", finding.File, err)
			}
			if abs == positiveFile {
				positiveLines = append(positiveLines, finding.Line)
			} else {
				t.Errorf("unexpected %s finding outside positive sample: %s:%d",
					ruleID, finding.File, finding.Line)
			}
		}
		if len(positiveLines) != 1 {
			t.Fatalf("expected exactly 1 %s finding in positive sample, got %d (findings=%+v)",
				ruleID, len(positiveLines), payload.Findings)
		}
		if positiveLines[0] != expectedPrintlnLine {
			t.Errorf("expected positive finding on line %d (println call), got line %d",
				expectedPrintlnLine, positiveLines[0])
		}
	})
}

// locateLocalKritRuleApi returns the version under
// ~/.m2/repository/dev/jasonpearson/krit/krit-rule-api/<v>/ when a jar
// exists there. Mirrors KritCustomRulePluginFunctionalTest's staged-jar
// probe so this test skips on the same condition.
func locateLocalKritRuleApi() (string, bool) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", false
	}
	dir := filepath.Join(home, ".m2", "repository", "dev", "jasonpearson", "krit", "krit-rule-api")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		v := e.Name()
		jar := filepath.Join(dir, v, "krit-rule-api-"+v+".jar")
		if info, err := os.Stat(jar); err == nil && !info.IsDir() {
			return v, true
		}
	}
	return "", false
}

// repoRoot resolves the repo root from the test source file location.
// `runtime.Caller` is more portable than shelling to `git rev-parse` and
// lets the test run from vendored checkouts that don't ship .git.
func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func buildKritBinary(t *testing.T, repoRoot string) string {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "krit")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/krit/")
	cmd.Dir = repoRoot
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build krit: %v\n%s", err, stderr.String())
	}
	return bin
}

func buildExampleRuleJar(t *testing.T, exampleDir, kritRuleApiVersion string) string {
	t.Helper()
	gradlew := filepath.Join(exampleDir, "gradlew")
	if runtime.GOOS == "windows" {
		gradlew = filepath.Join(exampleDir, "gradlew.bat")
	}

	cmd := exec.Command(gradlew,
		"--no-daemon",
		"--quiet",
		"-PkritVersion="+kritRuleApiVersion,
		"kritRuleJar",
	)
	cmd.Dir = exampleDir
	// Capture Gradle output and only surface it on failure to keep
	// `go test -v` logs readable on the green path.
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	start := time.Now()
	if err := cmd.Run(); err != nil {
		t.Fatalf("gradlew kritRuleJar (after %s): %v\n%s",
			time.Since(start), err, combined.String())
	}

	libsDir := filepath.Join(exampleDir, "build", "libs")
	entries, err := os.ReadDir(libsDir)
	if err != nil {
		t.Fatalf("read %s: %v", libsDir, err)
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), "krit-rules.jar") {
			return filepath.Join(libsDir, e.Name())
		}
	}
	t.Fatalf("no *-krit-rules.jar produced under %s; have: %v", libsDir, entries)
	return ""
}

func runKrit(t *testing.T, bin, repoRoot string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		// Exit code 1 = findings present; surface stdout so the caller
		// can assert on the JSON.
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return string(out)
		}
		stderr := ""
		if exitErr != nil {
			stderr = string(exitErr.Stderr)
		}
		t.Fatalf("krit %v: %v\nstderr: %s\nstdout: %s",
			args, err, stderr, string(out))
	}
	return string(out)
}
