package parity_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
)

// A default (non-verbose) scan reports FIR coverage: one stderr line naming
// the gated share and its top cause, summary.fir in JSON, and
// --min-fir-coverage fails the scan below the threshold (#1573).
func TestFirCoverageIsReportedWithoutVerboseEndToEnd(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	bin := buildVerdictKrit(t, root)
	project, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(project, "src", "main", "kotlin", "app")
	files := map[string]string{
		filepath.Join(project, "krit.yml"):   "oracle:\n  classpath: []\n",
		filepath.Join(src, "Ok.kt"):          "package app\n\nfun ok(): Int = 1\n",
		filepath.Join(src, "UsesCompose.kt"): "package app\n\nimport androidx.compose.ui.Modifier\n\nfun m(): Modifier = Modifier\n",
		filepath.Join(src, "UsesRuntime.kt"): "package app\n\nimport androidx.compose.runtime.Composable\n\n@Composable\nfun C() {}\n",
	}
	for path, body := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	run := func(extra ...string) (map[string]any, string, int) {
		args := append([]string{"--fir", "--no-fir-daemon", "--no-daemon", "--no-cache", "--no-type-oracle", "-f", "json"}, extra...)
		cmd := exec.Command(bin, append(args, project)...)
		cmd.Dir = project
		cmd.Env = append(os.Environ(), "KRIT_FIR_JAR="+jar, "KRIT_STATE_DIR="+t.TempDir())
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		err := cmd.Run()
		code := 0
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		} else if err != nil {
			t.Fatal(err)
		}
		var report map[string]any
		_ = json.Unmarshal(stdout.Bytes(), &report)
		return report, stderr.String(), code
	}

	report, stderr, code := run()
	if code > 1 {
		t.Fatalf("exit %d; stderr:\n%s", code, stderr)
	}
	if want := "info: FIR analyzed 1 of 3 files (33%); 2 gated by compile errors"; !strings.Contains(stderr, want) {
		t.Fatalf("stderr lacks %q:\n%s", want, stderr)
	}
	if want := "top causes: Unresolved reference 'androidx' x2"; !strings.Contains(stderr, want) {
		t.Fatalf("stderr lacks %q:\n%s", want, stderr)
	}
	summary, _ := report["summary"].(map[string]any)
	fir, _ := summary["fir"].(map[string]any)
	if fir["authoritative"] != float64(1) || fir["gated"] != float64(2) {
		t.Fatalf("summary.fir = %v", fir)
	}
	causes, _ := fir["topCauses"].([]any)
	if len(causes) != 1 || causes[0].(map[string]any)["cause"] != "Unresolved reference 'androidx'" {
		t.Fatalf("summary.fir.topCauses = %v", causes)
	}

	if _, stderr, code := run("--min-fir-coverage", "0.5"); code != 2 || !strings.Contains(stderr, "error: --min-fir-coverage=0.5: FIR analyzed 33.3%") {
		t.Fatalf("--min-fir-coverage 0.5: exit %d; stderr:\n%s", code, stderr)
	}
	if _, stderr, code := run("--min-fir-coverage", "0.3"); code > 1 {
		t.Fatalf("--min-fir-coverage 0.3: exit %d; stderr:\n%s", code, stderr)
	}
}
