package parity_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// writeJvmTargetSplitProject writes a project whose source sets share their
// compiler arguments and differ only in JVM target: :app targets 17 (the
// merged session's maximum) and :legacy targets 11 and declares a @JvmRecord
// class, which needs JVM target 16. Its real build reports that error; the
// merged session, compiled at 17, does not.
func writeJvmTargetSplitProject(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"settings.gradle.kts":                    "rootProject.name = \"fixture\"\ninclude(\":app\", \":legacy\")\n",
		"krit.yml":                               "oracle:\n  classpath: []\n",
		"app/src/main/kotlin/app/Point.kt":       "package app\n\n@JvmRecord\ndata class Point(val x: Int, val y: Int)\n",
		"legacy/src/main/kotlin/legacy/Pair.kt":  "package legacy\n\n@JvmRecord\ndata class Pair(val a: Int, val b: Int)\n",
		"legacy/src/main/kotlin/legacy/Plain.kt": "package legacy\n\nfun plain(): Int = 1\n",
	}
	for name, body := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	set := func(project, target string) string {
		return fmt.Sprintf(`{"path":":%[1]s","dir":%[2]q,"sourceSets":[{"name":"main","kind":"main","platform":"jvm","sourceDirs":[%[3]q],"jvmTarget":%[4]q,"kotlin":{"optIn":[],"freeCompilerArgs":[]}}]}`,
			project, filepath.Join(dir, project), filepath.Join(dir, project, "src", "main", "kotlin"), target)
	}
	modelDir := filepath.Join(dir, ".krit", "gradle-model")
	if err := os.MkdirAll(modelDir, 0o755); err != nil {
		t.Fatal(err)
	}
	model := `{"schema":2,"projects":[` + set("app", "17") + "," + set("legacy", "11") + `]}`
	if err := os.WriteFile(filepath.Join(modelDir, "root.json"), []byte(model), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}
