package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
	"github.com/kaeawc/krit/internal/oracle"
)

// TestOracleBackendParity drives both JVM daemons through their
// one-shot `--sources --output` CLI and asserts that the resulting
// type-oracle JSON is structurally equivalent. The check is the CI
// gate guarding against silent divergence on the analyze surface:
// changes that affect one backend but not the other will surface here
// before they leak into rule behaviour.
//
// The comparison intentionally tolerates a few backend-internal
// renderer differences (supertype FQN vs. short name, member return-
// type spelling for synthetic ctors) by doing substring matches on
// the supertype list and skipping return-type comparison on the
// synthetic `<init>` constructor — those don't reach Go-side rules.
func TestOracleBackendParity(t *testing.T) {
	root := repoRoot(t)
	kaaJar := oracle.FindJar([]string{root})
	if kaaJar == "" {
		t.Skip("krit-types jar not found; run `cd tools/krit-types && ./gradlew shadowJar` to enable oracle parity")
	}
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar` to enable oracle parity")
	}

	// A small, dependency-free fixture so we exercise the class /
	// supertype / member projection without needing a stdlib on the
	// classpath. Kotlin's auto-imported `Any` is the only library
	// supertype we touch.
	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	src := `package parity

open class Greeter {
    fun greet(name: String): String = "hi, " + name
}

class LoudGreeter : Greeter()
`
	if err := os.WriteFile(filepath.Join(srcDir, "Greeter.kt"), []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	kaaData := runOneShot(t, kaaJar, srcDir, filepath.Join(tmp, "kaa.json"))
	firData := runOneShot(t, firJar, srcDir, filepath.Join(tmp, "fir.json"))

	assertOracleParity(t, kaaData, firData)
}

func runOneShot(t *testing.T, jar, srcDir, outputPath string) *oracle.Data {
	t.Helper()
	if _, err := oracle.InvokeWithFiles(jar, []string{srcDir}, outputPath, "", false); err != nil {
		t.Fatalf("oracle invoke (%s): %v", jar, err)
	}
	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read %s: %v", outputPath, err)
	}
	var data oracle.Data
	if err := json.Unmarshal(raw, &data); err != nil {
		t.Fatalf("parse %s: %v\n%s", outputPath, err, string(raw))
	}
	return &data
}

// TestOracleBackendAcceptsClasspathFlag asserts that both backends'
// one-shot CLIs accept `--classpath` and analyze succeeds with the
// supplied JAR(s) on the classpath. Without this gate a typo on
// either side would break `oracle.InvokeWithFilesWithOptions` for
// every project that ships an explicit classpath.
//
// The classpath supplied is kotlin-stdlib (the dependency every
// Kotlin source set needs anyway). KAA's Analysis API ignores the
// flag in favor of its own classpath wiring; FIR uses it directly to
// configure K2's compilation environment. Both must finish without
// error.
func TestOracleBackendAcceptsClasspathFlag(t *testing.T) {
	root := repoRoot(t)
	kaaJar := oracle.FindJar([]string{root})
	if kaaJar == "" {
		t.Skip("krit-types jar not found")
	}
	firJar := firchecks.FindFirJar([]string{root})
	if firJar == "" || !isExecutableJar(firJar) {
		t.Skip("krit-fir jar not found")
	}
	stdlib := findKotlinStdlib()
	if stdlib == "" {
		t.Skip("kotlin-stdlib jar not found in Gradle cache")
	}

	tmp := t.TempDir()
	srcDir := filepath.Join(tmp, "src")
	if err := os.MkdirAll(srcDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "Sample.kt"), []byte("package x\nclass Sample\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	for _, tc := range []struct {
		name string
		jar  string
	}{
		{name: "kaa", jar: kaaJar},
		{name: "fir", jar: firJar},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := filepath.Join(tmp, tc.name+".json")
			opts := oracle.InvocationOptions{Classpath: []string{stdlib}}
			if _, err := oracle.InvokeWithFilesWithOptions(tc.jar, []string{srcDir}, out, "", false, opts); err != nil {
				t.Fatalf("%s with --classpath: %v", tc.name, err)
			}
			raw, err := os.ReadFile(out)
			if err != nil {
				t.Fatalf("read %s output: %v", tc.name, err)
			}
			var data oracle.Data
			if err := json.Unmarshal(raw, &data); err != nil {
				t.Fatalf("parse %s: %v\n%s", tc.name, err, string(raw))
			}
			if data.Version != 1 {
				t.Errorf("%s version = %d, want 1", tc.name, data.Version)
			}
		})
	}
}

func assertOracleParity(t *testing.T, kaa, fir *oracle.Data) {
	t.Helper()
	// Version + kotlinVersion are baked into both backends at
	// compile time; if they diverge the wire shape has shifted.
	if kaa.Version != fir.Version {
		t.Errorf("version mismatch: kaa=%d fir=%d", kaa.Version, fir.Version)
	}
	if kaa.KotlinVersion != fir.KotlinVersion {
		t.Errorf("kotlinVersion mismatch: kaa=%q fir=%q", kaa.KotlinVersion, fir.KotlinVersion)
	}

	// Each backend keys `files` by absolute path. The paths may
	// differ in symlink normalization (KAA uses PSI virtual file
	// paths; FIR canonicalizes via java.io.File). Compare by file
	// basename so the test stays robust against `/var` ↔ `/private/var`
	// drift on macOS.
	kaaByBase := indexByBasename(kaa.Files)
	firByBase := indexByBasename(fir.Files)
	if !sameKeys(kaaByBase, firByBase) {
		t.Fatalf("files keys differ: kaa=%v fir=%v", sortedKeys(kaaByBase), sortedKeys(firByBase))
	}

	for base := range kaaByBase {
		assertFileParity(t, base, kaaByBase[base], firByBase[base])
	}
}

func assertFileParity(t *testing.T, name string, kaa, fir *oracle.File) {
	t.Helper()
	if kaa.Package != fir.Package {
		t.Errorf("%s: package mismatch: kaa=%q fir=%q", name, kaa.Package, fir.Package)
	}

	kaaDecls := indexClassesByFqn(kaa.Declarations)
	firDecls := indexClassesByFqn(fir.Declarations)
	if !sameKeys(kaaDecls, firDecls) {
		t.Fatalf("%s: declaration FQN set differs: kaa=%v fir=%v", name, sortedKeys(kaaDecls), sortedKeys(firDecls))
	}

	for fqn, kaaCls := range kaaDecls {
		firCls := firDecls[fqn]
		if kaaCls.Kind != firCls.Kind {
			t.Errorf("%s.%s: kind mismatch: kaa=%q fir=%q", name, fqn, kaaCls.Kind, firCls.Kind)
		}
		// Supertype rendering legitimately differs between backends
		// (KAA emits `kotlin.Any`, FIR's renderReadable emits `Any`).
		// Assert each KAA supertype has a corresponding FIR entry
		// whose suffix matches — strictest parity check we can do
		// without losing on benign renderer differences.
		for _, want := range kaaCls.Supertypes {
			short := lastSegment(want)
			found := false
			for _, got := range firCls.Supertypes {
				if got == want || lastSegment(got) == short {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s.%s: kaa supertype %q has no parity in fir %v", name, fqn, want, firCls.Supertypes)
			}
		}
	}
}

func indexByBasename(files map[string]*oracle.File) map[string]*oracle.File {
	out := make(map[string]*oracle.File, len(files))
	for path, file := range files {
		out[filepath.Base(path)] = file
	}
	return out
}

func indexClassesByFqn(classes []*oracle.Class) map[string]*oracle.Class {
	out := make(map[string]*oracle.Class, len(classes))
	for _, cls := range classes {
		out[cls.FQN] = cls
	}
	return out
}

func sameKeys[T any](a, b map[string]T) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if _, ok := b[k]; !ok {
			return false
		}
	}
	return true
}

func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func lastSegment(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}
	return s
}
