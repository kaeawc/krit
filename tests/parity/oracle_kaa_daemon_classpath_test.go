package parity_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
)

// TestKAADaemonResolvesLibraryCallsLikeOneShot pins #852: the krit-types
// daemon that `--daemon` and `serve` start must analyze with the same
// effective classpath as the one-shot path. Use.kt calls into a Java library
// that exists only in a jar, so the call resolves only when that jar is on
// the classpath. The one-shot run and a daemon started through
// InvokeDaemonWithClasspath (the call those paths make) must report the same
// facts for the call, and a daemon started without the classpath (the old
// behavior) must not resolve it.
func TestKAADaemonResolvesLibraryCallsLikeOneShot(t *testing.T) {
	jar := kaaJarForDaemonTest(t)
	stdlib := findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache; set KOTLIN_STDLIB_JAR")
	}

	// Keep a developer's ~/.krit (cached jars, daemon state) out of the
	// result: the daemon locates its jar through EnsureJar, which reads
	// KRIT_TYPES_JAR first and the home cache after.
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("KRIT_TYPES_JAR", jar)
	t.Setenv("KRIT_DAEMON_CACHE", "off")

	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	// A conventional source root, so the daemon start's FindSourceDirs
	// finds it from the repo root as it does for a real project.
	src := filepath.Join(repo, "src", "main", "kotlin")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	use := filepath.Join(src, "Use.kt")
	writeFile(t, use, "package app\n\nimport lib.Greeter\n\nfun use(): String = Greeter.greet()\n")
	classpath := []string{stdlib, wcClasspathJar(t, filepath.Join(repo, "greeter.jar"))}

	oneShot := runOneShot(t, jar, src, filepath.Join(repo, "oneshot.json"),
		oracle.InvocationOptions{Backend: oracle.BackendKAA, Classpath: classpath})
	want := greetCall(t, "one-shot", oneShot)
	if !want.CallTargetResolved || want.CallTarget != "lib.Greeter.greet" {
		t.Fatalf("test premise broken: one-shot run should resolve Greeter.greet() to lib.Greeter.greet; got %+v", want)
	}

	withClasspath := analyzeWithDaemon(t, repo, classpath)
	greetCall(t, "daemon with classpath", withClasspath)
	daemonExprs := indexByBasename(withClasspath.Files)["Use.kt"].Expressions
	oneShotExprs := indexByBasename(oneShot.Files)["Use.kt"].Expressions
	if !reflect.DeepEqual(daemonExprs, oneShotExprs) {
		t.Errorf("daemon and one-shot disagree on Use.kt's resolved expressions:\n daemon:   %s\n one-shot: %s",
			formatExprs(daemonExprs), formatExprs(oneShotExprs))
	}

	// Regression for #852: the daemon used to start without the classpath.
	// It must not resolve the library call then, or this test cannot tell
	// the two starts apart.
	without := analyzeWithDaemon(t, repo, nil)
	if call := findGreetCall(without); call != nil && call.CallTargetResolved && call.CallTarget == want.CallTarget {
		t.Errorf("daemon started without the classpath still resolved Greeter.greet() (%+v); the parity check above proves nothing", call)
	}
}

// kaaJarForDaemonTest returns the krit-types shadow jar: KRIT_TYPES_JAR when
// set, else the one the parity CI job builds under tools/krit-types.
func kaaJarForDaemonTest(t *testing.T) string {
	t.Helper()
	jar := os.Getenv("KRIT_TYPES_JAR")
	if jar == "" {
		jar = filepath.Join(repoRoot(t), "tools", "krit-types", "build", "libs", "krit-types.jar")
	}
	if !isExecutableKAAJar(jar) {
		t.Skip("executable krit-types jar not found; run `cd tools/krit-types && ./gradlew shadowJar` or set KRIT_TYPES_JAR")
	}
	return jar
}

// analyzeWithDaemon starts a krit-types daemon the way runDaemonOracle and
// the custom-rule start do, analyzes every file, and shuts the daemon down.
func analyzeWithDaemon(t *testing.T, repo string, classpath []string) *oracle.Data {
	t.Helper()
	d, err := oracle.InvokeDaemonWithClasspath([]string{repo}, classpath, false)
	if err != nil {
		t.Fatalf("start daemon (classpath=%v): %v", classpath, err)
	}
	defer func() { _ = d.Shutdown() }()
	data, err := d.AnalyzeAll()
	if err != nil {
		t.Fatalf("daemon analyze (classpath=%v): %v", classpath, err)
	}
	return data
}

// greetCall returns the facts for the Greeter.greet() call in Use.kt.
func greetCall(t *testing.T, label string, data *oracle.Data) *oracle.ExpressionType {
	t.Helper()
	call := findGreetCall(data)
	if call == nil {
		t.Fatalf("%s: no expression facts for the Greeter.greet() call in Use.kt; files=%v", label, fileNames(data))
	}
	return call
}

// findGreetCall returns the expression in Use.kt whose call target names
// greet, or nil.
func findGreetCall(data *oracle.Data) *oracle.ExpressionType {
	file := indexByBasename(data.Files)["Use.kt"]
	if file == nil {
		return nil
	}
	for _, expr := range file.Expressions {
		if expr != nil && (expr.CallTarget == "lib.Greeter.greet" || strings.HasSuffix(expr.CallTarget, "greet")) {
			return expr
		}
	}
	return nil
}

func formatExprs(exprs map[string]*oracle.ExpressionType) string {
	keys := make([]string, 0, len(exprs))
	for k := range exprs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%s=%+v ", k, *exprs[k])
	}
	return b.String()
}

func fileNames(data *oracle.Data) []string {
	names := make([]string, 0, len(data.Files))
	for name := range data.Files {
		names = append(names, name)
	}
	return names
}
