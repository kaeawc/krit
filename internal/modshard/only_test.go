package modshard

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/firchecks"
)

// coveringWorker writes a java stand-in that answers each module of its request with facts for the
// files listed for its ID and an empty check response, and keeps a copy of the module request beside
// itself as modules.json.
func coveringWorker(t *testing.T, files map[string][]string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake worker is a shell script")
	}
	path := filepath.Join(t.TempDir(), "java")
	var script strings.Builder
	script.WriteString("#!/bin/sh\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = --facts-dir ]; then dir=$2; fi\n")
	script.WriteString("  if [ \"$1\" = --module-facts-file ]; then cp \"$2\" '" + filepath.Join(filepath.Dir(path), "modules.json") + "'; fi\n")
	script.WriteString("  shift\ndone\n")
	for id, paths := range files {
		facts := map[string]any{"files": map[string]any{}}
		for _, p := range paths {
			facts["files"].(map[string]any)[p] = map[string]any{"package": "p"}
		}
		data, _ := json.Marshal(facts)
		name := strings.TrimSuffix(factsFileName(id), ".json")
		script.WriteString("printf '%s' '{\"succeeded\":1,\"findings\":[],\"rules\":[\"R\"],\"errorFiles\":{}}' > \"$dir/" + name + ".check.json\"\n")
		script.WriteString("printf '%s' '" + string(data) + "' > \"$dir/" + name + ".json\"\n")
	}
	if err := os.WriteFile(path, []byte(script.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

// PR mode (#1660): a run limited to changed files compiles only the modules that hold them.
func TestRunOnlyAnalyzesTheModulesHoldingTheChangedFiles(t *testing.T) {
	t.Setenv("KRIT_WORKER_AOT", "0")
	model, root := fixtureModel(t)
	writeModel(t, root, model)
	app := filepath.Join(root, "app", "src", "main", "kotlin", "A.kt")
	lib := filepath.Join(root, "lib", "src", "main", "kotlin", "L.kt")
	java := coveringWorker(t, map[string][]string{":app:release": {app}, ":lib:release": {lib}, ":lib:releaseUnitTest": {}})
	request, _ := json.Marshal(map[string]any{"id": 1, "files": []map[string]string{{"path": app}}, "rules": []string{"R"}})
	res, err := Run(context.Background(), Input{
		ScanPaths: []string{root}, JarPath: filepath.Join(root, "krit-fir.jar"), JavaPath: java, Workers: 1,
		CacheDir: filepath.Join(t.TempDir(), "cache"), OutputPath: filepath.Join(t.TempDir(), "types.json"),
		Required: []string{app}, Only: []string{app}, CheckRequest: request,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Applied || res.Units != 1 || res.Covered != 1 || len(res.Check) == 0 {
		t.Fatalf("result = %+v", res)
	}
	var sent firchecks.AnalyzeModulesRequest
	if data, err := os.ReadFile(filepath.Join(filepath.Dir(java), "modules.json")); err != nil || json.Unmarshal(data, &sent) != nil {
		t.Fatalf("worker module request: %v", err)
	}
	var ids []string
	for _, m := range sent.Modules {
		ids = append(ids, m.ID)
	}
	if !slices.Equal(ids, []string{":app:release"}) {
		t.Fatalf("compiled %v, want only the changed file's module", ids)
	}
}

// A module that would compile without some of its dependencies gives worse verdicts than the
// whole-repository compile, so PR mode does not apply.
func TestRunOnlyDeclinesAModuleWithoutItsFullContext(t *testing.T) {
	model, root := fixtureModel(t)
	writeModel(t, root, model)
	if err := os.Remove(filepath.Join(root, "lib/build/intermediates/compile_library_classes_jar/release/bundleLibCompileToJarRelease/classes.jar")); err != nil {
		t.Fatal(err)
	}
	units, _ := Plan(model, root, []string{root})
	for i := range units {
		units[i].ContextDropped = units[i].Spec.ID == ":app:release"
	}
	if id := incompleteContext(unitsHolding(units, []string{filepath.Join(root, "app", "src", "main", "kotlin", "A.kt")})); id != ":app:release" {
		t.Fatalf("incompleteContext = %q", id)
	}
	res, err := Run(context.Background(), Input{
		ScanPaths: []string{root}, JarPath: filepath.Join(root, "krit-fir.jar"), JavaPath: "/bin/false",
		CacheDir: t.TempDir(), OutputPath: filepath.Join(t.TempDir(), "types.json"),
		Only: []string{filepath.Join(root, "elsewhere", "X.kt")},
	})
	if err != nil || res.Applied || res.Reason != "no module holds the changed files" {
		t.Fatalf("a change outside every module: %+v %v", res, err)
	}
}

// A dependency whose sources hold a changed file has a built jar that predates the change: a module
// depending on it compiles its sources as context instead (#1660).
func TestRunOnlyCompilesAChangedDependencyFromSource(t *testing.T) {
	model, root := fixtureModel(t)
	lib := filepath.Join(root, "lib", "src", "main", "kotlin", "L.kt")
	app := filepath.Join(root, "app", "src", "main", "kotlin", "A.kt")
	jar := filepath.Join(root, "lib/build/intermediates/compile_library_classes_jar/release/bundleLibCompileToJarRelease/classes.jar")
	appUnit := func(units []Unit) Unit {
		for _, u := range units {
			if u.Spec.ID == ":app:release" {
				return u
			}
		}
		t.Fatal("no :app:release unit")
		return Unit{}
	}
	unchanged, _ := planWith(model, root, []string{root}, projectsHolding(model, []string{app}))
	before := appUnit(unchanged)
	if !slices.Contains(before.Spec.Classpath, jar) || len(before.Spec.ContextSourceRoots) != 0 {
		t.Fatalf("an unchanged dependency uses its built jar: %+v", before.Spec)
	}
	if stale := projectsHolding(model, []string{app, lib}); !stale[":lib"] || !stale[":app"] || len(stale) != 2 {
		t.Fatalf("projectsHolding = %v", stale)
	}
	changed, _ := planWith(model, root, []string{root}, projectsHolding(model, []string{app, lib}))
	after := appUnit(changed)
	if slices.Contains(after.Spec.Classpath, jar) || !slices.Contains(after.Spec.ContextSourceRoots, filepath.Dir(lib)) {
		t.Fatalf("a changed dependency should compile from source: classpath %v context %v", after.Spec.Classpath, after.Spec.ContextSourceRoots)
	}
}
