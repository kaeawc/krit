package parity_test

import (
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

// End-to-end `krit --fir` over a Gradle-shaped project: the FIR pass
// compiles the project's source roots with its classpath, and where a file
// compiles cleanly the compiler verdict replaces the tree-sitter one for
// every rule the jar has a checker for.

// coroutineStubs declares the kotlinx.coroutines surface the fixtures use.
// It lives under src/main/kotlin/generated/: the Go scan skips generated
// sources, so these declarations reach the checkers only through the
// compile context (the source roots), never as scanned files.
const coroutineStubs = `package kotlinx.coroutines

open class CoroutineDispatcher

object Dispatchers {
    val IO: CoroutineDispatcher = CoroutineDispatcher()
    val Default: CoroutineDispatcher = CoroutineDispatcher()
}

suspend fun <T> withContext(context: CoroutineDispatcher, block: suspend () -> T): T = block()
`

var verdictFixtures = map[string]string{
	// Go and FIR both flag it: FIR confirms Go's finding.
	"app/Confirmed.kt": `package app

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class Confirmed {
    suspend fun load(): Int = withContext(Dispatchers.IO) { 1 }
}
`,
	// (a) Needs cross-file resolution: kotlinx.coroutines.Dispatchers is
	// declared in another source file. Go misses the aliased import; the
	// FIR checker resolves it and fires.
	"app/Aliased.kt": `package app

import kotlinx.coroutines.Dispatchers as D
import kotlinx.coroutines.withContext

class Aliased {
    suspend fun load(): Int = withContext(D.Default) { 2 }
}
`,
	// (c) Go false positive: Dispatchers here is the constructor property,
	// not kotlinx.coroutines.Dispatchers. The compiler resolves it and says
	// clean, so Go's finding is dropped.
	"app/Shadowed.kt": `package app

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class DispatcherSet(val IO: CoroutineDispatcher)

class Shadowed(private val Dispatchers: DispatcherSet) {
    suspend fun load(): Int = withContext(Dispatchers.IO) { 3 }
}
`,
	// (b) Same false positive, but the file does not compile (an unresolved
	// reference): its FIR verdict is gated and Go's finding stands.
	"app/Broken.kt": `package app

import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class BrokenSet(val IO: CoroutineDispatcher)

class Broken(private val Dispatchers: BrokenSet) {
    suspend fun load(): Int = withContext(Dispatchers.IO) { missingHelper() }
}
`,
}

type verdictFinding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

func TestFirAuthoritativeVerdictEndToEnd(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	bin := buildVerdictKrit(t, root)
	project := writeVerdictProject(t, map[string]string{
		"src/main/kotlin/generated/kotlinx/coroutines/Stubs.kt": coroutineStubs,
		// A Gradle script is scanned but never compiled: compiling it would
		// fail on the Gradle API and gate every file.
		"build.gradle.kts": "plugins { kotlin(\"jvm\") }\ndependencies { implementation(project(\":core\")) }\n",
	})

	goFindings, _ := runVerdictKrit(t, bin, jar, project, false)
	firFindings, firStderr := runVerdictKrit(t, bin, jar, project, true)
	goOnly, withFir := injectDispatcherLines(goFindings), injectDispatcherLines(firFindings)

	// The Go-only scan is the baseline the verdict is measured against.
	if want := []string{"app/Broken.kt:10", "app/Confirmed.kt:7", "app/Shadowed.kt:10"}; !equalStrings(goOnly, want) {
		t.Fatalf("Go-only InjectDispatcher findings = %v, want %v", goOnly, want)
	}
	want := []string{
		"app/Aliased.kt:7",   // (a) FIR-added: resolved through another source file
		"app/Broken.kt:10",   // (b) gated by the compiler error: Go's finding kept
		"app/Confirmed.kt:7", // confirmed by FIR
		// (c) app/Shadowed.kt:10 dropped: the compiler proves it clean
	}
	if !equalStrings(withFir, want) {
		t.Fatalf("--fir InjectDispatcher findings = %v, want %v", withFir, want)
	}
	for _, line := range []string{
		"verbose: FIR verdict: 3 authoritative files, 1 gated (compiler error or crash), 1 excluded",
		"verbose: FIR verdict InjectDispatcher: confirmed=1 go-dropped=1 fir-added=1",
	} {
		if !strings.Contains(firStderr, line) {
			t.Fatalf("verbose verdict summary missing %q; stderr:\n%s", line, firStderr)
		}
	}
}

// The classpath is part of the compile context too: a library type on
// oracle.classpath resolves, so its checker verdict is authoritative.
func TestFirAuthoritativeVerdictResolvesConfiguredClasspath(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	coroutines := findGradleJar("org.jetbrains.kotlinx", "kotlinx-coroutines-core-jvm")
	if coroutines == "" {
		t.Skip("kotlinx-coroutines-core-jvm jar not found in the Gradle cache")
	}
	bin := buildVerdictKrit(t, root)
	project := writeVerdictProject(t, map[string]string{
		"krit.yml": "oracle:\n  classpath:\n    - " + coroutines + "\n",
	})

	firFindings, _ := runVerdictKrit(t, bin, jar, project, true)
	withFir := injectDispatcherLines(firFindings)
	want := []string{"app/Aliased.kt:7", "app/Broken.kt:10", "app/Confirmed.kt:7"}
	if !equalStrings(withFir, want) {
		t.Fatalf("--fir InjectDispatcher findings with the coroutines jar on oracle.classpath = %v, want %v", withFir, want)
	}
}

func writeVerdictProject(t *testing.T, extra map[string]string) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for name, body := range verdictFixtures {
		files["src/main/kotlin/"+name] = body
	}
	for name, body := range extra {
		files[name] = body
	}
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func buildVerdictKrit(t *testing.T, root string) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "krit")
	cmd := exec.Command("go", "build", "-o", bin, "./cmd/krit/")
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build krit: %v\n%s", err, out)
	}
	return bin
}

func runVerdictKrit(t *testing.T, bin, jar, project string, fir bool) ([]verdictFinding, string) {
	t.Helper()
	args := []string{"--no-daemon", "--no-cache", "--no-type-oracle", "-f", "json", "-q", "--enable-rules", "InjectDispatcher"}
	if fir {
		args = append(args, "--fir", "--no-fir-daemon", "-v")
	}
	cmd := exec.Command(bin, append(args, project)...)
	// FindFirJar also looks next to the binary; point it at the jar under test.
	if err := os.Symlink(jar, filepath.Join(filepath.Dir(bin), "krit-fir.jar")); err != nil && !os.IsExist(err) {
		t.Fatal(err)
	}
	cmd.Dir = project
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
	for i := range out.Findings {
		path := out.Findings[i].File
		if !filepath.IsAbs(path) {
			path = filepath.Join(project, path)
		}
		if rel, err := filepath.Rel(srcRoot, path); err == nil {
			out.Findings[i].File = filepath.ToSlash(rel)
		}
	}
	return out.Findings, stderr.String()
}

func injectDispatcherLines(findings []verdictFinding) []string {
	var out []string
	for _, f := range findings {
		if f.Rule == "InjectDispatcher" {
			out = append(out, f.File+":"+strconv.Itoa(f.Line))
		}
	}
	sort.Strings(out)
	return out
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func findGradleJar(group, artifact string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	matches, _ := filepath.Glob(filepath.Join(home, ".gradle", "caches", "modules-2", "files-2.1", group, artifact, "*", "*", artifact+"-*.jar"))
	sort.Strings(matches)
	for i := len(matches) - 1; i >= 0; i-- {
		name := filepath.Base(matches[i])
		if strings.Contains(name, "sources") || strings.Contains(name, "javadoc") {
			continue
		}
		return matches[i]
	}
	return ""
}
