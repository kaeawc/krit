// Package playgroundrules_test exercises the playground custom-rule
// project end-to-end. The :custom-rules module ships two pure
// line-scan KritRule implementations (playground.NoHardcodedSecret and
// playground.NoTodoComment); the kotlin-webservice module wires them
// into kritCheck via the kritCustomRules resolvable configuration.
//
// This test takes the same rule jar produced by
// `./gradlew :custom-rules:kritRuleJar` and runs it directly against
// the kotlin-webservice sources via krit's --custom-rule-jars flag, so
// the JVM-loaded rule path stays exercised independently of the
// Gradle integration.
//
// Gates on prerequisites being present (java on PATH, locatable
// krit-types.jar) and t.Skips otherwise so `make integration` stays
// green on developer machines without a JVM. The dedicated
// `playground-rules` CI job stages the prerequisites and exercises
// the test for real.
package playgroundrules_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/kaeawc/krit/internal/oracle"
)

const (
	secretRuleID = "playground.NoHardcodedSecret"
	todoRuleID   = "playground.NoTodoComment"

	// Constants.kt holds two top-level `val NAME = "..."` declarations
	// whose names match the rule's credential-segment regex: the rule
	// must fire on both and only those two — `MIN_PASSWORD_LENGTH = 8`
	// at line 11 has a numeric literal and must NOT fire.
	constantsRelPath             = "src/main/kotlin/com/example/utils/Constants.kt"
	expectedDatabasePasswordLine = 26
	expectedJwtSecretLine        = 28

	// Application.kt has a single `// TODO:` comment that must fire
	// playground.NoTodoComment; the rule strips strings and comments
	// to avoid false positives, so this is the only match.
	applicationRelPath = "src/main/kotlin/com/example/Application.kt"
	expectedTodoLine   = 10
)

func TestPlaygroundCustomRules_EndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping playground custom-rule integration test in -short mode")
	}
	if _, err := exec.LookPath("java"); err != nil {
		t.Skip("skipping: `java` not on PATH (JDK 21+ required)")
	}
	if oracle.FindJar(nil) == "" {
		t.Skip("skipping: krit-types.jar not located. Set KRIT_TYPES_JAR or run " +
			"`./gradlew -p tools/krit-types shadowJar`")
	}

	repoRoot := repoRoot(t)
	playgroundDir := filepath.Join(repoRoot, "playground", "kotlin-webservice")
	customRulesDir := filepath.Join(repoRoot, "playground", "custom-rules")
	sourcesDir := filepath.Join(playgroundDir, "src", "main", "kotlin")
	constantsAbs := mustAbs(t, filepath.Join(playgroundDir, constantsRelPath))
	applicationAbs := mustAbs(t, filepath.Join(playgroundDir, applicationRelPath))

	kritBin := buildKritBinary(t, repoRoot)
	jarPath := buildPlaygroundRuleJar(t, playgroundDir, customRulesDir)

	t.Run("list-rules surfaces both playground rules with the plugin marker", func(t *testing.T) {
		stdout := runKrit(t, kritBin, repoRoot,
			"--list-rules",
			"--custom-rule-jars", jarPath,
			sourcesDir,
		)
		for _, id := range []string{secretRuleID, todoRuleID} {
			if !strings.Contains(stdout, id) {
				t.Errorf("expected --list-rules to include %q; got:\n%s", id, stdout)
			}
		}
		// `P` marker column distinguishes plugin-loaded rules from
		// built-ins. Loose match — column width can shift.
		if !strings.Contains(stdout, " P ") {
			t.Errorf("expected --list-rules to include the `P ` plugin marker; got:\n%s", stdout)
		}
	})

	t.Run("kritCheck fires both rules on the expected lines", func(t *testing.T) {
		stdout := runKrit(t, kritBin, repoRoot,
			"--custom-rule-jars", jarPath,
			"--daemon",
			"-f", "json",
			"--no-cache",
			"-q",
			sourcesDir,
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

		var secretLines, todoLines []int
		for _, f := range payload.Findings {
			abs, err := filepath.Abs(f.File)
			if err != nil {
				t.Fatalf("absolutize finding %q: %v", f.File, err)
			}
			switch f.Rule {
			case secretRuleID:
				if abs != constantsAbs {
					t.Errorf("unexpected %s outside Constants.kt: %s:%d",
						secretRuleID, f.File, f.Line)
					continue
				}
				secretLines = append(secretLines, f.Line)
			case todoRuleID:
				if abs != applicationAbs {
					t.Errorf("unexpected %s outside Application.kt: %s:%d",
						todoRuleID, f.File, f.Line)
					continue
				}
				todoLines = append(todoLines, f.Line)
			}
		}

		sort.Ints(secretLines)
		sort.Ints(todoLines)
		wantSecretLines := []int{expectedDatabasePasswordLine, expectedJwtSecretLine}
		if !slices.Equal(secretLines, wantSecretLines) {
			t.Errorf("NoHardcodedSecret: got lines %v on Constants.kt, want %v "+
				"(DATABASE_PASSWORD and JWT_SECRET; not MIN/MAX_PASSWORD_LENGTH which are numeric)",
				secretLines, wantSecretLines)
		}
		if !slices.Equal(todoLines, []int{expectedTodoLine}) {
			t.Errorf("NoTodoComment: got lines %v on Application.kt, want [%d]",
				todoLines, expectedTodoLine)
		}
	})
}

// buildPlaygroundRuleJar runs `./gradlew :custom-rules:kritRuleJar` from
// the kotlin-webservice playground (which composite-builds the krit
// Gradle plugins and krit-rule-api locally) and returns the path to the
// produced stamped rule jar. Output is captured and only surfaced on
// failure so successful runs stay quiet under `go test -v`.
func buildPlaygroundRuleJar(t *testing.T, playgroundDir, customRulesDir string) string {
	t.Helper()
	gradlew := filepath.Join(playgroundDir, "gradlew")
	if runtime.GOOS == "windows" {
		gradlew = filepath.Join(playgroundDir, "gradlew.bat")
	}

	cmd := exec.Command(gradlew, "--no-daemon", "--quiet", ":custom-rules:kritRuleJar")
	cmd.Dir = playgroundDir
	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined
	start := time.Now()
	if err := cmd.Run(); err != nil {
		t.Fatalf("gradlew :custom-rules:kritRuleJar (after %s): %v\n%s",
			time.Since(start), err, combined.String())
	}

	libsDir := filepath.Join(customRulesDir, "build", "libs")
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

// runKrit invokes the krit binary, tolerating exit code 1 (findings
// present) as a success since that is exactly what this test wants to
// observe. Stderr is logged so daemon-spawn warnings stay visible
// under `go test -v`.
func runKrit(t *testing.T, bin, repoRoot string, args ...string) string {
	t.Helper()
	cmd := exec.Command(bin, args...)
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if stderr.Len() > 0 {
		t.Logf("krit %v stderr:\n%s", args, stderr.String())
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return string(out)
		}
		t.Fatalf("krit %v: %v\nstderr: %s\nstdout: %s",
			args, err, stderr.String(), string(out))
	}
	return string(out)
}

func mustAbs(t *testing.T, p string) string {
	t.Helper()
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("absolutize %q: %v", p, err)
	}
	return abs
}
