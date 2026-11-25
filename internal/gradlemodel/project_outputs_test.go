package gradlemodel

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// A project dependency with built outputs is not compiled from source: only the
// scanned project's own sources, and those of unbuilt dependencies, are context.
func TestCompileSourceContextBuiltSkipsBuiltDependencies(t *testing.T) {
	root := t.TempDir()
	dir := func(module string) string { return filepath.Join(root, module, "src", "main", "kotlin") }
	model := &Model{Projects: []Project{
		{Path: ":app", Dir: filepath.Join(root, "app"), SourceSets: []SourceSet{
			{Name: "main", Kind: "main", Platform: "jvm", SourceDirs: []string{dir("app")}, ProjectDeps: []string{":lib", ":util"}},
		}},
		{Path: ":lib", Dir: filepath.Join(root, "lib"), SourceSets: []SourceSet{
			{Name: "main", Kind: "main", Platform: "jvm", SourceDirs: []string{dir("lib")}, ProjectDeps: []string{":core"}},
		}},
		{Path: ":core", Dir: filepath.Join(root, "core"), SourceSets: []SourceSet{
			{Name: "main", Kind: "main", Platform: "jvm", SourceDirs: []string{dir("core")}},
		}},
		{Path: ":util", Dir: filepath.Join(root, "util"), SourceSets: []SourceSet{
			{Name: "main", Kind: "main", Platform: "jvm", SourceDirs: []string{dir("util")}},
		}},
	}}
	for _, module := range []string{"app", "lib", "core", "util"} {
		write(t, filepath.Join(dir(module), "A.kt"), "package "+module+"\nclass A\n")
	}

	all, _ := model.CompileSourceContext([]string{dir("app")})
	if want := []string{dir("app"), dir("core"), dir("lib"), dir("util")}; !sameSet(all, want) {
		t.Fatalf("without outputs: got %v, want %v", all, want)
	}
	// :lib is built, so neither it nor :core (reached only through :lib) is context.
	built, fallback := model.CompileSourceContextBuilt([]string{dir("app")}, map[string]bool{":lib": true})
	if fallback {
		t.Fatal("unexpected fallback to every model source root")
	}
	if want := []string{dir("app"), dir("util")}; !sameSet(built, want) {
		t.Fatalf("with :lib built: got %v, want %v", built, want)
	}
}

func TestLoadProjectOutputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "outputs.json")
	write(t, path, `{"schema": 1, "projects": {":lib": ["/r/lib/classes"], ":core": ["/r/core.jar", "/r/core-res"]}}`)
	outputs, err := LoadProjectOutputs(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := ProjectOutputEntries(outputs), []string{"/r/core.jar", "/r/core-res", "/r/lib/classes"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("entries: got %v, want %v", got, want)
	}

	for name, content := range map[string]string{
		"schema":    `{"schema": 2, "projects": {}}`,
		"malformed": `{"schema": 1, "projects": [`,
	} {
		bad := filepath.Join(t.TempDir(), name+".json")
		write(t, bad, content)
		if _, err := LoadProjectOutputs(bad); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
	if _, err := LoadProjectOutputs(filepath.Join(t.TempDir(), "missing.json")); !os.IsNotExist(unwrapAll(err)) {
		t.Errorf("missing file: got %v", err)
	}
}

func sameSet(got, want []string) bool {
	seen := map[string]int{}
	for _, value := range got {
		seen[value]++
	}
	for _, value := range want {
		seen[value]--
	}
	for _, count := range seen {
		if count != 0 {
			return false
		}
	}
	return len(got) == len(want)
}

func unwrapAll(err error) error {
	for {
		next, ok := err.(interface{ Unwrap() error })
		if !ok || next.Unwrap() == nil {
			return err
		}
		err = next.Unwrap()
	}
}
