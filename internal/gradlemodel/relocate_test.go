package gradlemodel

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
)

// writeRelocatedModel writes a model as another machine exported it: build root
// oldRoot, Gradle user home oldHome, into newRoot/.krit/gradle-model.
func writeRelocatedModel(t *testing.T, newRoot, oldRoot, oldHome string) string {
	t.Helper()
	dir := filepath.Join(newRoot, ".krit", "gradle-model")
	jar := oldHome + "/caches/modules-2/files-2.1/com.squareup/moshi/1.15.0/abc/moshi-1.15.0.jar"
	body, err := json.Marshal(map[string]any{
		"schema": 2, "rootDir": oldRoot, "gradleUserHome": oldHome,
		"projects": []any{map[string]any{
			"path": ":app", "dir": oldRoot + "/app",
			"sourceSets": []any{map[string]any{
				"name": "main", "kind": "main",
				"sourceDirs":          []string{oldRoot + "/app/src/main/kotlin"},
				"generatedSourceDirs": []string{oldRoot + "/app/build/generated/ksp/main/kotlin"},
				"classpath":           []string{jar, "/opt/sdk/platforms/android-36/android.jar"},
				"generatedClasspath":  []string{oldRoot + "/app/build/R.jar"},
				"bootClasspath":       []string{oldRoot + "-other/android.jar"},
				"kotlin":              map[string]any{"compilerPluginClasspath": []string{jar}},
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "app.json"), string(body))
	return dir
}

func TestLoadRelocatesModelRestoredUnderAnotherRoot(t *testing.T) {
	root := t.TempDir()
	home := filepath.Join(t.TempDir(), "gradle-home")
	write(t, filepath.Join(home, "caches", "keep"), "")
	write(t, filepath.Join(root, "app", "build.gradle.kts"), "")
	t.Setenv("GRADLE_USER_HOME", home)
	dir := writeRelocatedModel(t, root, "/agent/1/work/repo", "/agent/1/.gradle")

	model, warnings, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Projects) != 1 {
		t.Fatalf("projects = %v, warnings = %v: the relocated project dir must exist", model.Projects, warnings)
	}
	r := filepath.ToSlash(root)
	h := filepath.ToSlash(home)
	jar := h + "/caches/modules-2/files-2.1/com.squareup/moshi/1.15.0/abc/moshi-1.15.0.jar"
	project := model.Projects[0]
	set := project.SourceSets[0]
	got := map[string]any{
		"dir": project.Dir, "sourceDirs": set.SourceDirs, "generatedSourceDirs": set.GeneratedSourceDirs,
		"classpath": set.ClasspathEntries, "generatedClasspath": set.GeneratedClasspath,
		"bootClasspath": set.BootClasspath, "compilerPluginClasspath": set.Kotlin.CompilerPluginClasspath,
	}
	want := map[string]any{
		"dir":                     r + "/app",
		"sourceDirs":              []string{r + "/app/src/main/kotlin"},
		"generatedSourceDirs":     []string{r + "/app/build/generated/ksp/main/kotlin"},
		"classpath":               []string{jar, "/opt/sdk/platforms/android-36/android.jar"},
		"generatedClasspath":      []string{r + "/app/build/R.jar"},
		"bootClasspath":           []string{"/agent/1/work/repo-other/android.jar"},
		"compilerPluginClasspath": []string{jar},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("relocated model:\n got  %v\n want %v", got, want)
	}
}

func TestLoadKeepsExistingGradleUserHome(t *testing.T) {
	root := t.TempDir()
	recordedHome := t.TempDir() // Gradle ran with -g: the recorded home still exists
	t.Setenv("GRADLE_USER_HOME", t.TempDir())
	write(t, filepath.Join(root, "app", "build.gradle.kts"), "")
	dir := writeRelocatedModel(t, root, filepath.ToSlash(root), filepath.ToSlash(recordedHome))
	model, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.ToSlash(recordedHome) + "/caches/modules-2/files-2.1/com.squareup/moshi/1.15.0/abc/moshi-1.15.0.jar"
	if got := model.Projects[0].SourceSets[0].ClasspathEntries[0]; got != want {
		t.Fatalf("classpath[0] = %s, want %s unchanged", got, want)
	}
}

func TestLoadReadsCopiedModelDirectoryAsWritten(t *testing.T) {
	// A model directory that is not <root>/.krit/gradle-model (a copy passed with
	// --gradle-model) says nothing about where the build is now.
	oldRoot := t.TempDir()
	write(t, filepath.Join(oldRoot, "app", "build.gradle.kts"), "")
	copyRoot := t.TempDir()
	dir := writeRelocatedModel(t, copyRoot, filepath.ToSlash(oldRoot), "/missing/.gradle")
	copied := filepath.Join(t.TempDir(), "model-copy")
	write(t, filepath.Join(copied, "app.json"), string(mustRead(t, filepath.Join(dir, "app.json"))))
	model, _, err := Load(copied)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := model.Projects[0].Dir, filepath.ToSlash(oldRoot)+"/app"; got != want {
		t.Fatalf("dir = %s, want %s", got, want)
	}
}

func TestRelocationMatchesWholePathSegments(t *testing.T) {
	r := relocation{from: "/work/repo", to: "/new/repo"}
	for in, want := range map[string]string{
		"/work/repo":         "/new/repo",
		"/work/repo/app":     "/new/repo/app",
		"/work/repo-other/x": "/work/repo-other/x",
		"/elsewhere":         "/elsewhere",
	} {
		if got := r.apply(in); got != want {
			t.Errorf("apply(%s) = %s, want %s", in, got, want)
		}
	}
}
