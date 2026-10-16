package oracle

import (
	"archive/zip"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
)

func writeJar(t *testing.T, dir, name string, entries ...string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, e := range entries {
		w, err := zw.Create(e)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = w.Write([]byte("x"))
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

// An older krit-types jar (for example via KRIT_TYPES_JAR) rejects --kotlin-arg
// as an unknown argument, which would disable the oracle. Krit must detect it
// and leave the flags off instead.
func TestKotlinLaunchArgsSkipJarsThatPredateTheFlag(t *testing.T) {
	dir := t.TempDir()
	kt := gradlemodel.KotlinSettings{LanguageVersion: "2.1", APIVersion: "2.1", OptIns: []string{"a.A"}}
	want := []string{"--kotlin-arg", "-language-version=2.1", "--kotlin-arg", "-api-version=2.1", "--kotlin-arg", "-opt-in=a.A"}

	modernTypes := writeJar(t, dir, "types.jar", "dev/jasonpearson/krit/types/Main.class", "dev/jasonpearson/krit/types/KotlinCompilerSettings.class")
	modernFir := writeJar(t, dir, "fir.jar", "dev/jasonpearson/krit/fir/runner/KotlinCompilerSettings.class")
	old := writeJar(t, dir, "old-types.jar", "dev/jasonpearson/krit/types/Main.class")
	for _, jar := range []string{modernTypes, modernFir} {
		if !JarSupportsKotlinSettings(jar) || !reflect.DeepEqual(KotlinLaunchArgs(jar, kt), want) {
			t.Errorf("%s supports the flags but got %v", filepath.Base(jar), KotlinLaunchArgs(jar, kt))
		}
	}
	if JarSupportsKotlinSettings(old) {
		t.Fatal("a jar without KotlinCompilerSettings was reported as supporting the flags")
	}
	if got := KotlinLaunchArgs(old, kt); got != nil {
		t.Fatalf("an old jar must not be sent --kotlin-arg: %v", got)
	}
	if KotlinLaunchArgs(modernTypes, gradlemodel.KotlinSettings{}) != nil {
		t.Fatal("no settings, no flags")
	}
	// Uninspectable jars are assumed capable: the launch then fails for its own reason.
	if !JarSupportsKotlinSettings(filepath.Join(dir, "missing.jar")) {
		t.Fatal("a missing jar must not be reported as downgraded")
	}
	// And the oracle command line honors it.
	opts := InvocationOptions{Backend: BackendKAA, Kotlin: kt}
	if got := compileContextArgs(old, opts); len(got) != 0 {
		t.Fatalf("one-shot args for an old jar: %v", got)
	}
	if got := compileContextArgs(modernTypes, opts); !reflect.DeepEqual(got, want) {
		t.Fatalf("one-shot args for a current jar: %v", got)
	}
	if got := appendDaemonJarArgsKT(nil, old, nil, nil, kt, "--daemon"); reflect.DeepEqual(got[len(got)-2:], want[:2]) || len(got) != 3 {
		t.Fatalf("daemon launch args for an old jar: %v", got)
	}
}
