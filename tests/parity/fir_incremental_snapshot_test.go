package parity_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
)

// The incremental snapshot is built by the JVM backend, which runs only
// through the shadow jar's own classes: the krit-fir unit tests compile
// against the full compiler and never see a class the jar left out. The jar
// once dropped the IR manglers that backend needs, so every snapshot build
// failed, no module was ever checked incrementally, and the scan blamed the
// module's compile errors (#1599). This scans a module with one file that
// does not compile, twice through the jar: the first scan must build a
// snapshot around that file, and an edit elsewhere must then be checked
// incrementally.
func TestFirIncrementalSnapshotThroughShadowJar(t *testing.T) {
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
	write := func(path, body string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(filepath.Join(project, "krit.yml"), "oracle:\n  classpath: []\n")
	for i := range 20 {
		write(filepath.Join(src, fmt.Sprintf("C%d.kt", i)), fmt.Sprintf("package app\n\nclass C%d(val v: Int) {\n    fun twice(): Int = v * 2\n}\n", i))
	}
	write(filepath.Join(src, "Broken.kt"), "package app\n\nfun broken(): Int = missingSymbol + 1\n")
	stateDir := t.TempDir()
	scan := func() string {
		t.Helper()
		cmd := exec.Command(bin, "--fir", "--no-daemon", "--no-fir-daemon", "-v", "-f", "json", project)
		cmd.Dir = project
		cmd.Env = append(os.Environ(), "KRIT_FIR_JAR="+jar, "KRIT_STATE_DIR="+stateDir, "KRIT_FIR_INCREMENTAL_DEBUG=1", "KRIT_FIR_INCREMENTAL=")
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		var exit *exec.ExitError
		if err := cmd.Run(); err != nil && (!errors.As(err, &exit) || exit.ExitCode() > 1) {
			t.Fatalf("krit: %v\nstderr:\n%s", err, stderr.String())
		}
		return stderr.String()
	}

	first := scan()
	if strings.Contains(first, "incremental snapshot failed") {
		t.Fatalf("the snapshot backend failed through the shadow jar:\n%s", first)
	}
	if want := "snapshot built without 1 file with compiler errors"; !strings.Contains(first, want) {
		t.Fatalf("first scan lacks %q:\n%s", want, first)
	}

	write(filepath.Join(src, "C3.kt"), "package app\n\nclass C3(val v: Int) {\n    fun twice(): Int = v * 2\n    fun thrice(): Int = v * 3\n}\n")
	if second := scan(); !strings.Contains(second, "krit-fir incremental: path=INCREMENTAL") {
		t.Fatalf("an edit after a snapshot was not checked incrementally:\n%s", second)
	}
}
