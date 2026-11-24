package parity_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
)

// variantFixtures is an Android-shaped project whose model exports the freeDebug variant (main, free,
// debug). Each build type declares its own BuildFlags and each flavor its own Branding, as the
// Android build-variant docs recommend; the real build never compiles two copies together.
var variantFixtures = map[string]string{
	"settings.gradle.kts":                  "rootProject.name = \"fixture\"\n",
	"src/main/kotlin/app/Api.kt":           "package app\n\nfun greeting(): String = \"hi\"\n",
	"src/debug/kotlin/app/BuildFlags.kt":   "package app\n\nobject BuildFlags {\n    val name: String = \"debug\" + greeting()\n}\n",
	"src/release/kotlin/app/BuildFlags.kt": "package app\n\nobject BuildFlags {\n    val name: String = \"release\" + greeting()\n}\n",
	"src/free/kotlin/app/Branding.kt":      "package app\n\nobject Branding {\n    val label: String = \"free \" + BuildFlags.name\n}\n",
	"src/paid/kotlin/app/Branding.kt":      "package app\n\nobject Branding {\n    val label: String = \"paid \" + BuildFlags.name\n}\n",
}

func writeVariantProject(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for name, body := range variantFixtures {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	var sourceDirs []string
	for _, set := range []string{"main", "free", "debug"} {
		sourceDirs = append(sourceDirs, filepath.Join(dir, "src", set, "kotlin"))
	}
	model, err := json.Marshal(map[string]any{"schema": 2, "projects": []map[string]any{{
		"path": ":", "dir": dir, "sourceSets": []map[string]any{{
			"name": "freeDebug", "kind": "main", "platform": "jvm", "variant": "freeDebug", "sourceDirs": sourceDirs,
		}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	modelDir := filepath.Join(dir, ".krit", "gradle-model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "root.json"), model, 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// Variant directories the model did not export are compiled in the merged --fir compile, where the
// per-build-type and per-flavor copies are redeclarations. The default scan rechecks each of them
// with the parts of its own variant (#1023): src/release with src/main and never src/debug. A flavor directory alone (src/paid) is compiled with src/main only, so
// its reference to a build-type class stays unresolved and the file stays gated: a known limit.
func TestFirRechecksVariantDirectoriesOutsideTheModelEndToEnd(t *testing.T) {
	root := repoRoot(t)
	jar := firchecks.FindFirJar([]string{root})
	if jar == "" || !isExecutableJar(jar) {
		t.Skip("krit-fir executable jar not found; run `cd tools/krit-fir && ./gradlew shadowJar`")
	}
	bin := buildVerdictKrit(t, root)
	project := writeVariantProject(t)
	file := func(rel string) string { return filepath.Join(project, "src", filepath.FromSlash(rel)) }

	merged := runSourceSetKrit(t, bin, jar, project, "merged")
	for _, rel := range []string{"debug/kotlin/app/BuildFlags.kt", "release/kotlin/app/BuildFlags.kt", "free/kotlin/app/Branding.kt", "paid/kotlin/app/Branding.kt"} {
		if line := "verbose: FIR gated: " + file(rel) + ": Redeclaration:"; !strings.Contains(merged, line) {
			t.Fatalf("merged compile: missing %q; stderr:\n%s", line, merged)
		}
	}

	isolated := runSourceSetKrit(t, bin, jar, project, "")
	for _, line := range []string{
		"(2 variant source sets outside the Gradle model)",
		"verbose: FIR source sets: 4 files gated by duplicate declarations; rechecked ",
		", 0 still gated by one",
		"verbose: FIR verdict: 4 authoritative files, 1 gated (compiler error or crash)",
		"verbose: FIR gated: " + file("paid/kotlin/app/Branding.kt") + ": Unresolved reference",
	} {
		if !strings.Contains(isolated, line) {
			t.Fatalf("default scan: missing %q; stderr:\n%s", line, isolated)
		}
	}
}
