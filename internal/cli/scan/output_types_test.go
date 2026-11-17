package scan

import (
	"io"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/store"
)

// outputTypesProject creates a project with one Kotlin source root and points
// KRIT_FIR_JAR at a placeholder jar, so RunOutputTypesTo reaches the oracle
// invocation without a JVM.
func outputTypesProject(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	src := filepath.Join(root, "src", "main", "kotlin")
	if err := os.MkdirAll(src, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "A.kt"), []byte("class A\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	jar := filepath.Join(t.TempDir(), "krit-fir.jar")
	if err := os.WriteFile(jar, []byte("jar"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KRIT_FIR_JAR", jar)
	return root
}

// captureOutputTypesInvocations replaces both oracle invocations and records
// the options each received.
func captureOutputTypesInvocations(t *testing.T) (cached, full *[]oracle.InvocationOptions) {
	t.Helper()
	prevCached, prevFull := invokeOutputTypesCached, invokeOutputTypesFull
	t.Cleanup(func() { invokeOutputTypesCached, invokeOutputTypesFull = prevCached, prevFull })
	cached, full = &[]oracle.InvocationOptions{}, &[]oracle.InvocationOptions{}
	invokeOutputTypesCached = func(_ string, _ []string, _, out, _ string, _ bool, _ *store.FileStore, opts oracle.InvocationOptions) (string, error) {
		*cached = append(*cached, opts)
		return out, nil
	}
	invokeOutputTypesFull = func(_ string, _ []string, out, _ string, _ bool, opts oracle.InvocationOptions) (string, error) {
		*full = append(*full, opts)
		return out, nil
	}
	return cached, full
}

func TestRunOutputTypesToPassesCompileContext(t *testing.T) {
	root := outputTypesProject(t)
	cached, full := captureOutputTypesInvocations(t)
	compile := OracleCompileContext{
		Classpath: []string{"/deps/a.jar"},
		JvmTarget: "17",
		Kotlin:    gradlemodel.KotlinSettings{LanguageVersion: "2.1"},
	}

	for _, noCache := range []bool{false, true} {
		code := RunOutputTypesTo(io.Discard, RunOutputTypesOpts{
			OutputPath:         filepath.Join(t.TempDir(), "types.json"),
			NoCacheOracle:      noCache,
			Paths:              []string{root},
			Compile:            compile,
			NoPersistentDaemon: true,
		})
		if code != 0 {
			t.Fatalf("RunOutputTypesTo(noCache=%v) = %d, want 0", noCache, code)
		}
	}
	if len(*cached) != 1 || len(*full) != 1 {
		t.Fatalf("invocations: cached=%d full=%d, want one each", len(*cached), len(*full))
	}
	for _, opts := range [][]oracle.InvocationOptions{*cached, *full} {
		got := opts[0]
		if !slices.Equal(got.Classpath, compile.Classpath) || got.JvmTarget != "17" || got.Kotlin.LanguageVersion != "2.1" {
			t.Errorf("compile context not passed to the oracle: %+v", got)
		}
		if !got.NoPersistentDaemon {
			t.Error("NoPersistentDaemon not passed to the oracle")
		}
		if got.Backend != oracle.BackendFIR {
			t.Errorf("Backend = %q, want krit-fir", got.Backend)
		}
	}
}

func TestRunOutputTypesToUsesModelSourceDirs(t *testing.T) {
	// No source root under the scan path: the Gradle model's roots alone
	// must be enough to dump types.
	root := t.TempDir()
	modelSrc := filepath.Join(outputTypesProject(t), "src", "main", "kotlin")
	cached, _ := captureOutputTypesInvocations(t)

	code := RunOutputTypesTo(io.Discard, RunOutputTypesOpts{
		OutputPath: filepath.Join(t.TempDir(), "types.json"),
		Paths:      []string{root},
		Compile:    OracleCompileContext{SourceDirs: []string{modelSrc}},
	})
	if code != 0 || len(*cached) != 1 {
		t.Fatalf("RunOutputTypesTo = %d with %d invocations, want 0 and one invocation", code, len(*cached))
	}
}

func TestOutputTypesWithoutPersistentDaemon(t *testing.T) {
	cases := []struct {
		noDaemon, noFirDaemon, want bool
	}{
		{false, false, false},
		{true, false, true},
		{false, true, true},
	}
	for _, c := range cases {
		f := freshScanFlags(t)
		*f.NoDaemon, *f.NoFirDaemon = c.noDaemon, c.noFirDaemon
		if got := outputTypesWithoutPersistentDaemon(f); got != c.want {
			t.Errorf("--no-daemon=%v --no-fir-daemon=%v: got %v, want %v", c.noDaemon, c.noFirDaemon, got, c.want)
		}
	}
}
