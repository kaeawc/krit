package parity_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
)

// isolationProbeRule is the Go rule the test-only ThrowingProbe checker
// impersonates: Go only sends catalog rule ids to krit-fir, and the
// production jar has no MagicNumber checker for the probe to collide with.
const isolationProbeRule = "MagicNumber"

// probeClassDir is where Gradle compiles the krit-fir test sources,
// relative to the repository root.
const probeClassDir = "tools/krit-fir/build/classes/kotlin/test"

// probeClassPrefix is ThrowingProbe's class files (the object and its
// nested checkers) inside probeClassDir.
const probeClassPrefix = "dev/jasonpearson/krit/fir/checkers/protocol/ThrowingProbe"

var isolationFixtures = map[string]string{
	"src/main/kotlin/generated/kotlinx/coroutines/Stubs.kt": coroutineStubs,
	"src/main/kotlin/app/Api.kt": `package app

fun throwingProbeCrash() {}
`,
	// The probe's checker throws on the throwingProbeCrash() call. Its
	// MagicNumber verdict falls back to Go here; InjectDispatcher on the
	// same file keeps the FIR verdict.
	"src/main/kotlin/app/Throwing.kt": `package app

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class Throwing {
    suspend fun load(): Int = withContext(Dispatchers.IO) { 1 }
    fun scaled(x: Int): Int {
        throwingProbeCrash()
        return x * 1234
    }
}
`,
	// The probe runs cleanly here and reports nothing, so the FIR verdict
	// drops Go's MagicNumber finding.
	"src/main/kotlin/app/Clean.kt": `package app

class Clean {
    fun scaled(x: Int): Int {
        return x * 1234
    }
}
`,
}

// A checker that throws costs only its own rule on the file it threw on: the
// compile carries on, every other file and rule keeps the FIR verdict, and
// that (file, rule) pair keeps Go's findings. Runs the production jar with
// the test-only ThrowingProbe classes added.
func TestFirCheckerExceptionIsIsolatedToRuleAndFileEndToEnd(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	probeClasses, _ := filepath.Glob(filepath.Join(root, probeClassDir, filepath.FromSlash(probeClassPrefix)+"*.class"))
	if len(probeClasses) == 0 {
		t.Skip("krit-fir test classes not found; run `cd tools/krit-fir && ./gradlew test`")
	}
	bin := buildVerdictKrit(t, root)
	withProbe := filepath.Join(filepath.Dir(bin), "krit-fir.jar")
	writeJarWithProbe(t, jar, withProbe, filepath.Join(root, probeClassDir), probeClasses)
	project := writeIsolationProject(t)

	goFindings, _ := runIsolationKrit(t, bin, project, false)
	firFindings, stderr := runIsolationKrit(t, bin, project, true)

	if want := []string{"InjectDispatcher app/Throwing.kt:7", "MagicNumber app/Clean.kt:5", "MagicNumber app/Throwing.kt:10"}; !equalStrings(goFindings, want) {
		t.Fatalf("Go-only findings = %v, want %v", goFindings, want)
	}
	want := []string{
		"InjectDispatcher app/Throwing.kt:7", // confirmed by FIR on the file the probe threw on
		// MagicNumber app/Clean.kt:5 dropped: the probe ran cleanly there
		"MagicNumber app/Throwing.kt:10", // the probe threw here: Go's finding kept
	}
	if !equalStrings(firFindings, want) {
		t.Fatalf("--fir findings = %v, want %v\nstderr:\n%s", firFindings, want, stderr)
	}
	throwing := filepath.Join(project, "src", "main", "kotlin", "app", "Throwing.kt")
	for _, line := range []string{
		"verbose: FIR verdict: 3 authoritative files, 0 gated (compiler error or crash), 0 excluded (scripts or not in a JVM source set), 1 rule errors",
		"verbose: FIR rule error: " + isolationProbeRule + ": " + throwing + ": krit-fir: checker threw java.lang.IllegalStateException: probe",
		"verbose: FIR verdict " + isolationProbeRule + ": confirmed=0 go-dropped=1 fir-added=0",
		"verbose: FIR verdict InjectDispatcher: confirmed=1 go-dropped=0 fir-added=0",
	} {
		if !strings.Contains(stderr, line) {
			t.Fatalf("verbose output missing %q; stderr:\n%s", line, stderr)
		}
	}
}

// writeJarWithProbe copies jar to dst and adds the probe class files.
func writeJarWithProbe(t *testing.T, jar, dst, classRoot string, classes []string) {
	t.Helper()
	zr, err := zip.OpenReader(jar)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	out, err := os.Create(dst)
	if err != nil {
		t.Fatal(err)
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			// zip.Writer.Copy rejects directory entries that carry an
			// (empty) deflate stream; recreate them without data.
			if _, err := zw.CreateHeader(&zip.FileHeader{Name: f.Name, Method: zip.Store, Modified: f.Modified}); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := zw.Copy(f); err != nil {
			t.Fatal(err)
		}
	}
	for _, class := range classes {
		rel, err := filepath.Rel(classRoot, class)
		if err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(class)
		if err != nil {
			t.Fatal(err)
		}
		w, err := zw.Create(filepath.ToSlash(rel))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeIsolationProject(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range isolationFixtures {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// runIsolationKrit scans project with the jar next to bin and returns
// "Rule file:line" for every finding, file relative to the source root.
func runIsolationKrit(t *testing.T, bin, project string, fir bool) ([]string, string) {
	t.Helper()
	args := []string{"--no-daemon", "--no-cache", "--no-type-oracle", "-f", "json", "-q",
		"--enable-rules", "InjectDispatcher," + isolationProbeRule}
	if fir {
		args = append(args, "--fir", "--no-fir-daemon", "-v")
	}
	cmd := exec.Command(bin, append(args, project)...)
	cmd.Dir = project
	cmd.Env = append(os.Environ(), "JAVA_TOOL_OPTIONS=-Dkrit.fir.test.throwingProbeRuleId="+isolationProbeRule)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() > 1) {
		t.Fatalf("krit %v: %v\nstderr:\n%s", args, err, stderr.String())
	}
	var out struct {
		Findings []verdictFinding `json:"findings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		t.Fatalf("parse krit output: %v\n%s\nstderr:\n%s", err, stdout.String(), stderr.String())
	}
	srcRoot := filepath.Join(project, "src", "main", "kotlin")
	var lines []string
	for _, f := range out.Findings {
		if f.Rule != "InjectDispatcher" && f.Rule != isolationProbeRule {
			continue
		}
		path := f.File
		if !filepath.IsAbs(path) {
			path = filepath.Join(project, path)
		}
		if rel, err := filepath.Rel(srcRoot, path); err == nil {
			path = filepath.ToSlash(rel)
		}
		lines = append(lines, f.Rule+" "+path+":"+strconv.Itoa(f.Line))
	}
	sort.Strings(lines)
	return lines, stderr.String()
}
