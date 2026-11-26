package gradlemodel

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// writeRJarModel writes a one-project model whose source sets list the given
// generatedClasspath entries, keyed by source-set name.
func writeRJarModel(t *testing.T, root string, generated map[string][]string) string {
	t.Helper()
	dir := filepath.Join(root, ".krit", "gradle-model")
	var sets []map[string]any
	for _, name := range []string{"debug", "debugUnitTest", "debugAndroidTest"} {
		if entries, ok := generated[name]; ok {
			sets = append(sets, map[string]any{"name": name, "kind": name, "generatedClasspath": entries})
		}
	}
	body, err := json.Marshal(map[string]any{
		"schema":   2,
		"projects": []any{map[string]any{"path": ":lib", "dir": root, "sourceSets": sets}},
	})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(dir, "lib.json"), string(body))
	return dir
}

func TestLoadResolvesPredictedRClassJars(t *testing.T) {
	// Glob metacharacters in the checkout path must not change the lookup.
	root := filepath.Join(t.TempDir(), "repo [1] *")
	inter := filepath.ToSlash(filepath.Join(root, "build", "intermediates"))
	jar := func(rel string) string { return inter + "/" + rel }
	// main: the predicted jar exists and is kept as is.
	mainJar := jar("compile_r_class_jar/debug/generateDebugRFile/R.jar")
	write(t, mainJar, "")
	// test: predicted for AGP 8's folder; AGP wrote it to the other runtime folder.
	testPredicted := jar("compile_and_runtime_not_namespaced_r_class_jar/debugUnitTest/generateDebugUnitTestStubRFile/R.jar")
	testActual := jar("compile_and_runtime_r_class_jar/debugUnitTest/generateDebugUnitTestStubRFile/R.jar")
	write(t, testActual, "")
	// androidTest: jars in two task directories of both runtime folders; none at the prediction.
	atPredicted := jar("compile_and_runtime_not_namespaced_r_class_jar/debugAndroidTest/processDebugAndroidTestResources/R.jar")
	atOther := jar("compile_and_runtime_r_class_jar/debugAndroidTest/customTask/R.jar")
	atSibling := jar("compile_and_runtime_not_namespaced_r_class_jar/debugAndroidTest/otherTask/R.jar")
	write(t, atOther, "")
	write(t, atSibling, "")
	// A jar of another component, or a compile-only R folder, never stands in for a runtime one.
	write(t, jar("compile_and_runtime_r_class_jar/release/processReleaseResources/R.jar"), "")
	write(t, jar("compile_r_class_jar/debugAndroidTest/generateDebugAndroidTestRFile/R.jar"), "")

	dir := writeRJarModel(t, root, map[string][]string{
		"debug":            {mainJar},
		"debugUnitTest":    {testPredicted},
		"debugAndroidTest": {atPredicted},
	})
	model, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string][]string{}
	for _, set := range model.Projects[0].SourceSets {
		got[set.Name] = set.GeneratedClasspath
	}
	want := map[string][]string{
		"debug":            {mainJar},
		"debugUnitTest":    {testActual},
		"debugAndroidTest": {atSibling, atOther},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("generatedClasspath = %v, want %v", got, want)
	}
	if present := model.GeneratedClasspath(); len(present) != 4 {
		t.Fatalf("GeneratedClasspath() = %v, want the 4 jars on disk", present)
	}
}

func TestLoadKeepsUnbuiltRClassJarPrediction(t *testing.T) {
	root := t.TempDir()
	predicted := filepath.ToSlash(filepath.Join(root, "build", "intermediates",
		"compile_and_runtime_r_class_jar", "debug", "processDebugResources", "R.jar"))
	other := filepath.ToSlash(filepath.Join(root, "build", "libs", "R.jar"))
	dir := writeRJarModel(t, root, map[string][]string{"debug": {predicted, other}})
	model, _, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := model.Projects[0].SourceSets[0].GeneratedClasspath; !reflect.DeepEqual(got, []string{predicted, other}) {
		t.Fatalf("generatedClasspath = %v, want the predictions unchanged", got)
	}
	if _, missing := model.ClasspathWithMissing(); len(missing) != 2 {
		t.Fatalf("missing = %v, want both entries", missing)
	}
	if _, err := os.Stat(filepath.Dir(predicted)); err == nil {
		t.Fatal("loading must not create build directories")
	}
}

// A model restored under another checkout path (#1663) is relocated before its
// R jar predictions are looked up, so the lookup searches the new root.
func TestLoadResolvesRClassJarsOfRelocatedModel(t *testing.T) {
	root := t.TempDir()
	oldRoot := "/agent/1/work/repo"
	rel := "/lib/build/intermediates/"
	predicted := "compile_and_runtime_not_namespaced_r_class_jar/debugUnitTest/generateDebugUnitTestStubRFile/R.jar"
	actual := filepath.ToSlash(root) + rel + "compile_and_runtime_r_class_jar/debugUnitTest/generateDebugUnitTestStubRFile/R.jar"
	write(t, actual, "")
	body, err := json.Marshal(map[string]any{
		"schema": 2, "rootDir": oldRoot,
		"projects": []any{map[string]any{"path": ":lib", "dir": oldRoot + "/lib", "sourceSets": []any{map[string]any{
			"name": "debugUnitTest", "kind": "test", "generatedClasspath": []string{oldRoot + rel + predicted},
		}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, ".krit", "gradle-model")
	write(t, filepath.Join(dir, "lib.json"), string(body))

	model, warnings, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(model.Projects) != 1 {
		t.Fatalf("projects = %v, warnings = %v", model.Projects, warnings)
	}
	if got := model.Projects[0].SourceSets[0].GeneratedClasspath; !reflect.DeepEqual(got, []string{actual}) {
		t.Fatalf("generatedClasspath = %v, want [%s]", got, actual)
	}
}
