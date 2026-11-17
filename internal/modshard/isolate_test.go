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
	"github.com/kaeawc/krit/internal/gradlemodel"
)

func TestIsolationUnitsKeepEveryScannedSetOfAnOwningProject(t *testing.T) {
	model, root := fixtureModel(t)
	units, _ := Plan(model, root, []string{root})
	kept, skipped := isolationUnits(units, []string{filepath.Join(root, "lib", "src", "test", "kotlin", "LTest.kt")})
	var ids []string
	for _, u := range kept {
		ids = append(ids, u.Spec.ID)
	}
	if !slices.Equal(ids, []string{":lib:release", ":lib:releaseUnitTest"}) || len(skipped) != 0 {
		t.Fatalf("both :lib sets should be rechecked, got %v (skipped %v)", ids, skipped)
	}
}

func TestIsolationUnitsRecheckASingleSetAndIgnoreUnknownFiles(t *testing.T) {
	model, root := fixtureModel(t)
	units, _ := Plan(model, root, []string{root})
	gated := []string{filepath.Join(root, "app", "src", "main", "kotlin", "A.kt"), filepath.Join(root, "elsewhere", "B.kt")}
	kept, skipped := isolationUnits(units, gated)
	if len(kept) != 1 || kept[0].Spec.ID != ":app:release" || len(skipped) != 0 {
		t.Fatalf("a single set merged with other projects' sets is rechecked alone; unknown files are ignored: %+v %v", kept, skipped)
	}
}

func TestIsolationUnitsSkipProjectsWithoutFullContext(t *testing.T) {
	model, root := fixtureModel(t)
	units, _ := Plan(model, root, []string{root})
	for i := range units {
		if units[i].Spec.ID == ":lib:releaseUnitTest" {
			units[i].ContextDropped = true
		}
	}
	kept, skipped := isolationUnits(units, []string{filepath.Join(root, "lib", "src", "main", "kotlin", "L.kt")})
	if len(kept) != 0 || !slices.Equal(skipped, []string{":lib"}) {
		t.Fatalf("a project that lost dependency context keeps its merged verdicts: %+v %v", kept, skipped)
	}
}

func TestIsolatorIsOffWhenMergedIsRequested(t *testing.T) {
	t.Setenv(ShardsEnv, "merged")
	if Isolator([]string{t.TempDir()}, true, nil) != nil {
		t.Fatal("KRIT_MODULE_SHARDS=merged should turn the recheck off")
	}
	t.Setenv(ShardsEnv, "")
	if Isolator([]string{t.TempDir()}, true, nil) == nil {
		t.Fatal("the recheck should be on by default")
	}
}

// fakeWorker writes a java stand-in that answers every module of its request with a facts file and a
// check response holding one finding per file in files.
func fakeWorker(t *testing.T, ids []string, files []string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("the fake worker is a shell script")
	}
	var findings []string
	for _, f := range files {
		findings = append(findings, `{"path":"`+f+`","line":1,"col":1,"rule":"R","severity":"warning","message":"isolated"}`)
	}
	check := `{"succeeded":` + "1" + `,"findings":[` + strings.Join(findings, ",") + `],"rules":["R"],"errorFiles":{}}`
	var script strings.Builder
	script.WriteString("#!/bin/sh\nwhile [ $# -gt 0 ]; do\n  if [ \"$1\" = --facts-dir ]; then dir=$2; fi\n  shift\ndone\n")
	for _, id := range ids {
		name := strings.TrimSuffix(factsFileName(id), ".json")
		script.WriteString("printf '%s' '" + check + "' > \"$dir/" + name + ".check.json\"\n")
		script.WriteString("printf '%s' '{\"files\":{}}' > \"$dir/" + name + ".json\"\n")
	}
	path := filepath.Join(t.TempDir(), "java")
	if err := os.WriteFile(path, []byte(script.String()), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeModel(t *testing.T, root string, model *gradlemodel.Model) {
	t.Helper()
	dir := filepath.Join(root, ".krit", "gradle-model")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]any{"schema": 2, "projects": model.Projects})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "model.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCheckIsolatedRechecksTheOwningProjectsSetsOnly(t *testing.T) {
	t.Setenv("KRIT_WORKER_AOT", "0")
	model, root := fixtureModel(t)
	writeModel(t, root, model)
	main := filepath.Join(root, "lib", "src", "main", "kotlin", "L.kt")
	test := filepath.Join(root, "lib", "src", "test", "kotlin", "LTest.kt")
	app := filepath.Join(root, "app", "src", "main", "kotlin", "A.kt")
	request, _ := json.Marshal(map[string]any{
		"id":    2,
		"files": []map[string]string{{"path": main}, {"path": test}, {"path": app}},
		"rules": []string{"R"},
	})
	java := fakeWorker(t, []string{":lib:release", ":lib:releaseUnitTest"}, []string{test})
	resp, rechecked, err := CheckIsolated(context.Background(), Input{
		ScanPaths:    []string{root},
		JarPath:      filepath.Join(root, "krit-fir.jar"),
		JavaPath:     java,
		Workers:      1,
		CacheDir:     filepath.Join(t.TempDir(), "cache"),
		CheckRequest: request,
	}, []string{test})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rechecked, []string{main, test}) {
		t.Fatalf("both :lib files should be rechecked and :app left alone, got %v", rechecked)
	}
	var got struct {
		ID         int64 `json:"id"`
		Findings   []struct{ Path string }
		ErrorFiles map[string]string `json:"errorFiles"`
		Rules      []string          `json:"rules"`
	}
	if err := json.Unmarshal(resp, &got); err != nil {
		t.Fatal(err)
	}
	if got.ID != 2 || len(got.Findings) == 0 || got.Findings[0].Path != test || len(got.ErrorFiles) != 0 || !slices.Equal(got.Rules, []string{"R"}) {
		t.Fatalf("the response should answer the request for the rechecked files only: %s", resp)
	}
}

func TestCheckIsolatedLeavesFailedUnitsToTheMergedVerdict(t *testing.T) {
	t.Setenv("KRIT_WORKER_AOT", "0")
	model, root := fixtureModel(t)
	writeModel(t, root, model)
	main := filepath.Join(root, "lib", "src", "main", "kotlin", "L.kt")
	test := filepath.Join(root, "lib", "src", "test", "kotlin", "LTest.kt")
	request, _ := json.Marshal(map[string]any{"id": 2, "files": []map[string]string{{"path": main}, {"path": test}}, "rules": []string{"R"}})
	// The worker answers only the main set; the test set has no check response.
	java := fakeWorker(t, []string{":lib:release"}, nil)
	_, rechecked, err := CheckIsolated(context.Background(), Input{
		ScanPaths: []string{root}, JarPath: filepath.Join(root, "krit-fir.jar"), JavaPath: java, Workers: 1,
		CacheDir: filepath.Join(t.TempDir(), "cache"), CheckRequest: request,
	}, []string{test})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rechecked, []string{main}) {
		t.Fatalf("only the unit that finished should replace merged verdicts, got %v", rechecked)
	}
}

func TestProjectsSharingFilesFlagsOverlappingSets(t *testing.T) {
	units := []Unit{
		{Spec: firchecks.ModuleSpec{ID: ":a:debug"}}, {Spec: firchecks.ModuleSpec{ID: ":a:release"}},
		{Spec: firchecks.ModuleSpec{ID: ":b:main"}}, {Spec: firchecks.ModuleSpec{ID: ":b:test"}},
	}
	requested := map[string][]string{
		":a:debug": {"/a/src/main/A.kt"}, ":a:release": {"/a/src/main/A.kt"},
		":b:main": {"/b/src/main/B.kt"}, ":b:test": {"/b/src/test/BTest.kt"},
	}
	if got := projectsSharingFiles(units, requested); !got[":a"] || got[":b"] {
		t.Fatalf("only :a has a file in two sets: %v", got)
	}
}
