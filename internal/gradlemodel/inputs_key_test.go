package gradlemodel

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mustRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// writeBuild lays out a small multi-project build with one file of every input
// kind, plus files that must not count.
func writeBuild(t *testing.T, root string) {
	t.Helper()
	for path, body := range map[string]string{
		"settings.gradle.kts":                      `include(":app")`,
		"build.gradle.kts":                         "",
		"gradle.properties":                        "org.gradle.jvmargs=-Xmx2g",
		"local.properties":                         "sdk.dir=/opt/sdk",
		"gradle/libs.versions.toml":                "[versions]\nkotlin = \"2.2.0\"",
		"gradle/verification-metadata.xml":         "<verification-metadata/>",
		"gradle/wrapper/gradle-wrapper.properties": "distributionUrl=gradle-9.0-bin.zip",
		"gradle/init.gradle":                       "",
		"app/build.gradle":                         "",
		"app/gradle.lockfile":                      "com.squareup:moshi:1.15.0=compileClasspath",
		"buildSrc/build.gradle.kts":                "",
		"buildSrc/src/main/kotlin/Conventions.kt":  "object Conventions",
		"build-logic/src/main/kotlin/A.kt":         "",
		// Not inputs: sources, outputs, caches, VCS, other Gradle files.
		"app/src/main/kotlin/Main.kt":                    "fun main() {}",
		"app/build/tmp/build.gradle.kts":                 "",
		".gradle/8.0/gc.properties":                      "",
		".krit/gradle-model/app.json":                    "{}",
		".git/config":                                    "",
		"buildSrc/build/generated/Gen.kt":                "",
		"buildSrc/src/test/kotlin/ConventionsTest.kt":    "",
		"gradle/wrapper/gradle-wrapper.jar":              "",
		"app/src/main/resources/gradle.properties.dummy": "",
	} {
		write(t, filepath.Join(root, filepath.FromSlash(path)), body)
	}
}

func TestModelInputsListsBuildConfigurationOnly(t *testing.T) {
	root := t.TempDir()
	writeBuild(t, root)
	got, err := ModelInputs(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"app/build.gradle",
		"app/gradle.lockfile",
		"build-logic/src/main/kotlin/A.kt",
		"build.gradle.kts",
		"buildSrc/build.gradle.kts",
		"buildSrc/src/main/kotlin/Conventions.kt",
		"gradle.properties",
		"gradle/init.gradle",
		"gradle/libs.versions.toml",
		"gradle/verification-metadata.xml",
		"gradle/wrapper/gradle-wrapper.properties",
		"local.properties",
		"settings.gradle.kts",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ModelInputs =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestInputsKeyIsRelocatableAndContentSensitive(t *testing.T) {
	first, second := t.TempDir(), filepath.Join(t.TempDir(), "elsewhere")
	writeBuild(t, first)
	writeBuild(t, second)
	a, err := InputsKey(first)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(a, inputsKeyVersion+"-") {
		t.Fatalf("key %s lacks the version prefix", a)
	}
	b, err := InputsKey(second)
	if err != nil {
		t.Fatal(err)
	}
	if a != b {
		t.Fatalf("the same build at another path has key %s, want %s", b, a)
	}
	// Sources and outputs do not change the key.
	write(t, filepath.Join(second, "app/src/main/kotlin/Main.kt"), "fun main() { println() }")
	write(t, filepath.Join(second, "app/build/outputs/x.txt"), "x")
	if b, _ = InputsKey(second); a != b {
		t.Fatal("a source or output edit changed the key")
	}
	for _, edit := range []string{"gradle/libs.versions.toml", "app/gradle.lockfile", "buildSrc/src/main/kotlin/Conventions.kt"} {
		write(t, filepath.Join(second, filepath.FromSlash(edit)), "changed")
		changed, err := InputsKey(second)
		if err != nil {
			t.Fatal(err)
		}
		if changed == b {
			t.Fatalf("editing %s left the key at %s", edit, b)
		}
		b = changed
	}
	// A new input file changes the key even when empty.
	write(t, filepath.Join(first, "feature/build.gradle.kts"), "")
	if c, _ := InputsKey(first); c == a {
		t.Fatal("adding a project's build script left the key unchanged")
	}
}

func TestInputsKeyStampRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if got := ReadInputsKey(dir); got != "" {
		t.Fatalf("no stamp read as %q", got)
	}
	if err := WriteInputsKey(dir, "gm1-abc"); err != nil {
		t.Fatal(err)
	}
	if got := ReadInputsKey(dir); got != "gm1-abc" {
		t.Fatalf("stamp = %q", got)
	}
	// The stamp is not a model file.
	if _, _, err := Load(dir); err != nil {
		t.Fatal(err)
	}
}

func TestModelRoot(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "settings.gradle.kts"), "")
	write(t, filepath.Join(root, ".git", "HEAD"), "")
	if got := ModelRoot(filepath.Join(root, ".krit", "gradle-model"), nil); got != root {
		t.Fatalf("ModelRoot(<root>/.krit/gradle-model) = %s, want %s", got, root)
	}
	sub := filepath.Join(root, "app", "src")
	write(t, filepath.Join(sub, "A.kt"), "")
	if got := ModelRoot(t.TempDir(), []string{sub}); got != root {
		t.Fatalf("ModelRoot(copy, [%s]) = %s, want %s", sub, got, root)
	}
}
