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

// An older krit-fir jar (for example via KRIT_FIR_JAR) rejects --kotlin-arg
// as an unknown argument, which would disable the oracle. Krit must detect it
// and leave the flags off instead.
func TestKotlinLaunchArgsSkipJarsThatPredateTheFlag(t *testing.T) {
	dir := t.TempDir()
	kt := gradlemodel.KotlinSettings{LanguageVersion: "2.1", APIVersion: "2.1", OptIns: []string{"a.A"}}
	want := []string{"--kotlin-arg", "-language-version=2.1", "--kotlin-arg", "-api-version=2.1", "--kotlin-arg", "-opt-in=a.A"}

	modernFir := writeJar(t, dir, "fir.jar", "dev/jasonpearson/krit/fir/Main.class", "dev/jasonpearson/krit/fir/runner/KotlinCompilerSettings.class")
	old := writeJar(t, dir, "old-fir.jar", "dev/jasonpearson/krit/fir/Main.class")
	if !JarSupportsKotlinSettings(modernFir) || !reflect.DeepEqual(KotlinLaunchArgs(modernFir, kt), want) {
		t.Errorf("%s supports the flags but got %v", filepath.Base(modernFir), KotlinLaunchArgs(modernFir, kt))
	}
	if JarSupportsKotlinSettings(old) {
		t.Fatal("a jar without KotlinCompilerSettings was reported as supporting the flags")
	}
	if got := KotlinLaunchArgs(old, kt); got != nil {
		t.Fatalf("an old jar must not be sent --kotlin-arg: %v", got)
	}
	if KotlinLaunchArgs(modernFir, gradlemodel.KotlinSettings{}) != nil {
		t.Fatal("no settings, no flags")
	}
	// Uninspectable jars are assumed capable: the launch then fails for its own reason.
	if !JarSupportsKotlinSettings(filepath.Join(dir, "missing.jar")) {
		t.Fatal("a missing jar must not be reported as downgraded")
	}
	// And the oracle command line honors it.
	opts := InvocationOptions{Backend: BackendFIR, Kotlin: kt}
	if got := compileContextArgs(old, opts); len(got) != 0 {
		t.Fatalf("one-shot args for an old jar: %v", got)
	}
	if got := compileContextArgs(modernFir, opts); !reflect.DeepEqual(got, want) {
		t.Fatalf("one-shot args for a current jar: %v", got)
	}
	if got := appendDaemonJarArgsKT(nil, old, nil, nil, kt, "--daemon"); reflect.DeepEqual(got[len(got)-2:], want[:2]) || len(got) != 3 {
		t.Fatalf("daemon launch args for an old jar: %v", got)
	}
}

// Compiler plugins go to a krit-fir jar that understands them as flags of
// their own; a krit-fir jar that predates them gets the settings without the plugins rather than failing to
// start on an unknown flag.
func TestKotlinLaunchArgsRenderCompilerPluginsForKritFir(t *testing.T) {
	dir := t.TempDir()
	kt := gradlemodel.KotlinSettings{LanguageVersion: "2.3", Plugins: gradlemodel.CompilerPlugins{
		Classpath: []string{"/plugins/serialization.jar"}, Options: []string{"plugin:s:k=v"},
	}}
	settings := []string{"--kotlin-arg", "-language-version=2.3"}
	plugins := []string{"--compiler-plugin", "/plugins/serialization.jar", "--compiler-plugin-option", "plugin:s:k=v"}

	fir := writeJar(t, dir, "fir.jar", "dev/jasonpearson/krit/fir/runner/KotlinCompilerSettings.class", "dev/jasonpearson/krit/fir/runner/CompilerPlugins.class")
	oldFir := writeJar(t, dir, "old-fir.jar", "dev/jasonpearson/krit/fir/runner/KotlinCompilerSettings.class")
	if !JarSupportsCompilerPlugins(fir) || JarSupportsCompilerPlugins(oldFir) {
		t.Fatal("plugin support detection")
	}
	if got, want := KotlinLaunchArgs(fir, kt), append(append([]string{}, settings...), plugins...); !reflect.DeepEqual(got, want) {
		t.Fatalf("krit-fir args = %v want %v", got, want)
	}
	if got := KotlinLaunchArgs(oldFir, kt); !reflect.DeepEqual(got, settings) {
		t.Fatalf("old krit-fir args = %v want %v", got, settings)
	}
	onlyPlugins := gradlemodel.KotlinSettings{Plugins: kt.Plugins}
	if got := KotlinLaunchArgs(fir, onlyPlugins); !reflect.DeepEqual(got, plugins) {
		t.Fatalf("plugins without other settings = %v", got)
	}
	opts := InvocationOptions{Backend: BackendFIR, Kotlin: kt}
	if got := compileContextArgs(fir, opts); !reflect.DeepEqual(got[len(got)-4:], plugins) {
		t.Fatalf("one-shot args = %v", got)
	}
	if got := appendDaemonJarArgsKT(nil, fir, nil, nil, kt, "--daemon"); !reflect.DeepEqual(got[len(got)-4:], plugins) {
		t.Fatalf("daemon launch args = %v", got)
	}
	// The daemon registry key follows the plugins.
	if daemonRegistryKeyKT(fir, []string{dir}, kt) == daemonRegistryKeyKT(fir, []string{dir}, gradlemodel.KotlinSettings{LanguageVersion: "2.3"}) {
		t.Fatal("a daemon launched with plugins must not answer for one without")
	}
}
