package gradlemodel

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCompileSourceDirs(t *testing.T) {
	root := t.TempDir()
	sourceRoot := func(module, sourceSet, language string) string {
		return filepath.Join(root, module, "src", sourceSet, language)
	}
	appMain := sourceRoot("app", "main", "kotlin")
	appJava := sourceRoot("app", "main", "java")
	appTest := sourceRoot("app", "test", "kotlin")
	appAndroidTest := sourceRoot("app", "androidTest", "kotlin")
	appGenerated := filepath.Join(root, "app", "build", "generated", "main")
	libMain := sourceRoot("lib", "main", "kotlin")
	libGenerated := filepath.Join(root, "lib", "build", "generated", "main")
	coreMain := sourceRoot("core", "main", "kotlin")
	testLibMain := sourceRoot("testlib", "main", "kotlin")
	projects := []Project{
		{Path: ":app", Dir: filepath.Join(root, "app"), SourceSets: []SourceSet{
			{Name: "main", Kind: "main", Platform: "jvm", SourceDirs: []string{appMain, appJava}, GeneratedSourceDirs: []string{appGenerated}, ProjectDeps: []string{":lib", ":missing"}},
			{Name: "test", Kind: "test", Platform: "jvm", SourceDirs: []string{appTest}, ProjectDeps: []string{":testlib"}},
			{Name: "androidTest", Kind: "androidTest", Platform: "android", SourceDirs: []string{appAndroidTest}},
		}},
		{Path: ":lib", Dir: filepath.Join(root, "lib"), SourceSets: []SourceSet{
			{Name: "main", Kind: "main", Platform: "jvm", SourceDirs: []string{libMain, libMain, filepath.Join(root, "absent")}, GeneratedSourceDirs: []string{libGenerated}, ProjectDeps: []string{":core", ":native"}},
			{Name: "test", Kind: "test", SourceDirs: []string{sourceRoot("lib", "test", "kotlin")}},
			{Name: "androidTest", Kind: "androidTest", SourceDirs: []string{sourceRoot("lib", "androidTest", "kotlin")}},
		}},
		{Path: ":core", Dir: filepath.Join(root, "core"), SourceSets: []SourceSet{
			{Name: "main", Kind: "main", Platform: "android", SourceDirs: []string{coreMain}, ProjectDeps: []string{":lib"}},
		}},
		{Path: ":testlib", Dir: filepath.Join(root, "testlib"), SourceSets: []SourceSet{
			{Name: "main", Kind: "main", SourceDirs: []string{testLibMain}},
		}},
		{Path: ":native", Dir: filepath.Join(root, "native"), SourceSets: []SourceSet{
			{Name: "nativeMain", Kind: "main", Platform: "native", SourceDirs: []string{sourceRoot("native", "nativeMain", "kotlin")}},
		}},
		{Path: ":unrelated", Dir: filepath.Join(root, "unrelated"), SourceSets: []SourceSet{
			{Name: "main", Kind: "main", SourceDirs: []string{sourceRoot("unrelated", "main", "kotlin")}},
		}},
	}
	for _, project := range projects {
		for _, set := range project.SourceSets {
			for _, dir := range append(append([]string(nil), set.SourceDirs...), set.GeneratedSourceDirs...) {
				if dir == filepath.Join(root, "absent") {
					continue
				}
				write(t, filepath.Join(dir, "Shared.kt"), "package "+filepath.Base(project.Dir)+"\nclass Shared\n")
			}
		}
	}
	modelDir := filepath.Join(root, ".krit", "gradle-model")
	body, err := json.Marshal(struct {
		Schema   int       `json:"schema"`
		Projects []Project `json:"projects"`
	}{Schema: 2, Projects: projects})
	if err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(modelDir, "model.json"), string(body))
	model, warnings, err := Load(modelDir)
	if err != nil || len(warnings) != 0 {
		t.Fatalf("load fixture: warnings=%v err=%v", warnings, err)
	}
	before := append([]string(nil), model.SourceDirs()...)
	wantMain := []string{appMain, appJava, appGenerated, coreMain, libMain, libGenerated}
	wantTest := []string{appMain, appJava, appGenerated, appTest, coreMain, libMain, libGenerated, testLibMain}
	tests := []struct {
		name  string
		paths []string
		want  []string
	}{
		{name: "outside dependency main roots only", paths: []string{appMain}, want: wantMain},
		{name: "single checked file", paths: []string{filepath.Join(appMain, "Shared.kt")}, want: wantMain},
		{name: "generated source scan", paths: []string{appGenerated}, want: wantMain},
		{name: "overlapping scan paths deduplicated", paths: []string{appMain, appJava, appMain, libMain}, want: wantMain},
		{name: "test excludes androidTest context", paths: []string{appTest}, want: wantTest},
		{name: "androidTest excludes test context", paths: []string{appAndroidTest}, want: []string{appMain, appJava, appGenerated, appAndroidTest, coreMain, libMain, libGenerated}},
		{name: "explicitly scanned tests retained", paths: []string{appTest, appAndroidTest}, want: []string{appMain, appJava, appGenerated, appTest, appAndroidTest, coreMain, libMain, libGenerated, testLibMain}},
		{name: "module scan retains every scanned source set", paths: []string{filepath.Join(root, "app")}, want: []string{appMain, appJava, appGenerated, appTest, appAndroidTest, coreMain, libMain, libGenerated, testLibMain}},
		{name: "whole repository retains every scanned source set", paths: []string{root}, want: before},
		{name: "unmatched paths", paths: []string{appMain + "-other"}, want: before},
		{name: "empty paths", want: before},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := model.CompileSourceDirs(test.paths); !reflect.DeepEqual(got, test.want) {
				t.Fatalf("source roots = %v, want %v", got, test.want)
			}
		})
	}
	if got := model.SourceDirs(); !reflect.DeepEqual(got, before) {
		t.Fatal("compilation context mutated the model")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	relative, err := filepath.Rel(cwd, appMain)
	if err != nil {
		t.Fatal(err)
	}
	if got := model.CompileSourceDirs([]string{relative}); !reflect.DeepEqual(got, wantMain) {
		t.Fatalf("relative source roots = %v, want %v", got, wantMain)
	}
	if got := (*Model)(nil).CompileSourceDirs([]string{appMain}); got != nil {
		t.Fatalf("nil model source roots = %v", got)
	}
}

func TestCompileSourceDirsTestFixtures(t *testing.T) {
	root := t.TempDir()
	source := func(module, kind string) SourceSet {
		dir := filepath.Join(root, module, "src", kind, "kotlin")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		return SourceSet{Name: kind, Kind: kind, Platform: "jvm", SourceDirs: []string{dir}}
	}
	appMain, appTest, appAndroidTest := source("app", "main"), source("app", "test"), source("app", "androidTest")
	appFixtures := source("app", "testFixtures")
	libMain, libTest, libFixtures := source("lib", "main"), source("lib", "test"), source("lib", "testFixtures")
	helperMain := source("helper", "main")
	appMain.ProjectDeps = []string{":lib"}
	appTest.ProjectDeps = []string{":lib"}
	appAndroidTest.ProjectDeps = []string{":lib"}
	appFixtures.ProjectDeps = []string{":lib"}
	libFixtures.ProjectDeps = []string{":helper", ":app"}
	model := &Model{Projects: []Project{
		{Path: ":app", SourceSets: []SourceSet{appMain, appTest, appAndroidTest, appFixtures}},
		{Path: ":lib", SourceSets: []SourceSet{libMain, libTest, libFixtures}},
		{Path: ":helper", SourceSets: []SourceSet{helperMain}},
	}}
	for _, test := range []struct {
		name string
		sets []SourceSet
		want []SourceSet
	}{
		{name: "main excludes fixtures", sets: []SourceSet{appMain}, want: []SourceSet{appMain, libMain}},
		{name: "test includes local and dependency fixtures", sets: []SourceSet{appTest}, want: []SourceSet{appMain, appTest, appFixtures, libMain, libFixtures, helperMain}},
		{name: "androidTest includes fixtures", sets: []SourceSet{appAndroidTest}, want: []SourceSet{appMain, appAndroidTest, appFixtures, libMain, libFixtures, helperMain}},
		{name: "testFixtures includes dependency fixtures", sets: []SourceSet{appFixtures}, want: []SourceSet{appMain, appFixtures, libMain, libFixtures, helperMain}},
		{name: "main selected before test still expands fixtures", sets: []SourceSet{appMain, appTest}, want: []SourceSet{appMain, appTest, appFixtures, libMain, libFixtures, helperMain}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var paths []string
			for _, set := range test.sets {
				paths = append(paths, set.SourceDirs...)
			}
			wantModel := &Model{Projects: []Project{{SourceSets: test.want}}}
			if got, want := model.CompileSourceDirs(paths), wantModel.SourceDirs(); !reflect.DeepEqual(got, want) {
				t.Fatalf("roots = %v, want %v", got, want)
			}
		})
	}
}

func TestSourcePathsOverlapSymlinks(t *testing.T) {
	selection := newCompileSourceSelection(&Model{})
	root := t.TempDir()
	realRoot := filepath.Join(root, "real")
	if err := os.MkdirAll(realRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(realRoot, "Example.kt")
	write(t, file, "class Example")
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(realRoot, alias); err != nil {
		t.Fatal(err)
	}
	for _, pair := range [][2]string{
		{realRoot, alias}, {alias, realRoot},
		{realRoot, filepath.Join(alias, "Example.kt")}, {alias, file},
		{filepath.Join(root, "missing"), filepath.Join(root, "missing", "Example.kt")},
	} {
		if !sourcePathsOverlap(selection.resolvePath(pair[0]), selection.resolvePath(pair[1])) {
			t.Errorf("paths do not overlap: %v", pair)
		}
	}
	resolvedRoot := selection.resolvePath(realRoot)
	if sourcePathsOverlap(resolvedRoot, selection.resolvePath(realRoot+"-other")) || sourcePathsOverlap("", resolvedRoot) {
		t.Fatal("unrelated or empty paths overlap")
	}
	model := &Model{Projects: []Project{{SourceSets: []SourceSet{{Kind: "main", SourceDirs: []string{realRoot}}}}}}
	if dirs, fallback := model.CompileSourceContext([]string{alias}); fallback || !reflect.DeepEqual(dirs, []string{realRoot}) {
		t.Fatalf("symlink scan roots=%v fallback=%v", dirs, fallback)
	}
}

func TestCompileSourceSelectionResolvesEachPathOnce(t *testing.T) {
	root := t.TempDir()
	model := &Model{}
	wantPaths := map[string]bool{}
	for index := 0; index < 48; index++ {
		path := filepath.Join(root, "roots", fmt.Sprint(index))
		wantPaths[path] = true
		model.Projects = append(model.Projects, Project{SourceSets: []SourceSet{
			{Kind: "main", SourceDirs: []string{path, path}, GeneratedSourceDirs: []string{path}},
		}})
	}
	var paths []string
	for index := 0; index < 40; index++ {
		path := filepath.Join(root, "scans", fmt.Sprint(index))
		wantPaths[path] = true
		paths = append(paths, path, path)
	}
	selection := newCompileSourceSelection(model)
	calls := map[string]int{}
	selection.resolver = func(path string) (string, error) {
		calls[path]++
		return path, nil
	}
	selection.match(paths)
	selection.match(paths)
	if len(selection.queue) != 0 {
		t.Fatal("disjoint paths selected source sets")
	}
	if len(calls) != len(wantPaths) {
		t.Fatalf("resolved %d distinct paths, want %d", len(calls), len(wantPaths))
	}
	for path := range wantPaths {
		if calls[path] != 1 {
			t.Errorf("resolved %q %d times, want once", path, calls[path])
		}
	}
}

func TestCompileSourceSelectionCachesResolutionFailure(t *testing.T) {
	selection := newCompileSourceSelection(&Model{Projects: []Project{{SourceSets: []SourceSet{
		{Kind: "main", SourceDirs: []string{"unresolvable", "unresolvable", ""}},
	}}}})
	calls := 0
	selection.resolver = func(string) (string, error) {
		calls++
		return "", errors.New("cannot resolve path")
	}
	selection.match([]string{"unresolvable", "unresolvable", ""})
	if calls != 1 || len(selection.queue) != 0 {
		t.Fatalf("resolution calls=%d selected=%d, want 1 and 0", calls, len(selection.queue))
	}
}
