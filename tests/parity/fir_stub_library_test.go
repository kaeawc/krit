package parity_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
)

// The FIR parity tests compile against the same stub library as the krit-fir
// compiler tests (tools/krit-fir/compiler-tests/src/test/data/stubs, see its
// README), so a checker is judged against one set of library shapes everywhere:
//
//   - Kotlin stubs (stubs/*.kt) are sent as sources alongside the snippet.
//   - Java stubs (stubs/java/**.java) model the Android platform. The check
//     request has no Java source roots, so they are compiled once with javac
//     and put on the classpath, where K2 reads them as Java classes (statics
//     without Companion, synthetic properties, platform types).
const firStubsRel = "tools/krit-fir/compiler-tests/src/test/data/stubs"

var javaStubs struct {
	once sync.Once
	dir  string
	err  string
	skip string
}

func TestMain(m *testing.M) {
	code := m.Run()
	if javaStubs.dir != "" {
		_ = os.RemoveAll(javaStubs.dir)
	}
	os.Exit(code)
}

// firStubLibrary returns the Kotlin stub sources and the classpath entry
// holding the compiled Java stubs. It skips when javac is unavailable.
func firStubLibrary(t *testing.T) (ktFiles []string, classpath []string) {
	t.Helper()
	root := repoRoot(t)
	stubs := filepath.Join(root, firStubsRel)
	ktFiles, err := filepath.Glob(filepath.Join(stubs, "*.kt"))
	if err != nil || len(ktFiles) == 0 {
		t.Fatalf("no Kotlin stubs under %s (err=%v)", firStubsRel, err)
	}
	sort.Strings(ktFiles)

	javaStubs.once.Do(func() { compileJavaStubs(filepath.Join(stubs, "java")) })
	if javaStubs.skip != "" {
		t.Skip(javaStubs.skip)
	}
	if javaStubs.err != "" {
		t.Fatal(javaStubs.err)
	}
	return ktFiles, []string{javaStubs.dir}
}

func compileJavaStubs(javaRoot string) {
	javac := findJavac()
	if javac == "" {
		javaStubs.skip = "javac not found (set JAVA_HOME or put a JDK on PATH); needed to compile the Java stubs"
		return
	}
	var sources []string
	_ = filepath.WalkDir(javaRoot, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".java") {
			sources = append(sources, path)
		}
		return nil
	})
	if len(sources) == 0 {
		javaStubs.err = "no Java stubs under " + javaRoot
		return
	}
	dir, err := os.MkdirTemp("", "krit-parity-java-stubs-")
	if err != nil {
		javaStubs.err = err.Error()
		return
	}
	javaStubs.dir = dir
	args := append([]string{"-proc:none", "-nowarn", "-d", dir}, sources...)
	if out, err := exec.Command(javac, args...).CombinedOutput(); err != nil {
		javaStubs.err = "javac failed on the Java stubs: " + err.Error() + "\n" + string(out)
	}
}

// firJar returns the krit-fir shadow jar and a kotlin-stdlib jar, skipping
// when either is unavailable.
func firJar(t *testing.T) (jar, stdlib string) {
	t.Helper()
	jar = firchecks.FindFirJar([]string{repoRoot(t)})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	stdlib = findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache; set KOTLIN_STDLIB_JAR")
	}
	return jar, stdlib
}

// firInvoke checks files (plus the Kotlin stubs) with the given FIR rules in
// one krit-fir run, with the stdlib and the compiled Java stubs on the
// classpath. ErrorFiles lists requested files with compile errors, stubs
// included; Crashed lists files lost to a compiler crash.
func firInvoke(t *testing.T, rules []string, files []string) *firchecks.Result {
	t.Helper()
	jar, stdlib := firJar(t)
	stubs, stubClasspath := firStubLibrary(t)
	all := append(append([]string{}, stubs...), files...)
	classpath := append([]string{stdlib}, stubClasspath...)
	res, err := firchecks.InvokeCached(jar, all, nil, classpath, rules, nil, firchecks.FileFacts{}, "", false, false)
	if err != nil {
		t.Fatalf("krit-fir invoke: %v", err)
	}
	return res
}

func findJavac() string {
	if home := os.Getenv("JAVA_HOME"); home != "" {
		p := filepath.Join(home, "bin", "javac")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	p, err := exec.LookPath("javac")
	if err != nil {
		return ""
	}
	return p
}
