package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/pipeline"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestLoadGradleCompileContextScopesDependencySources(t *testing.T) {
	root := t.TempDir()
	modelDir := filepath.Join(root, ".krit", "gradle-model")
	appMain := filepath.Join(root, "app", "src", "main", "kotlin")
	appTest := filepath.Join(root, "app", "src", "test", "kotlin")
	libMain := filepath.Join(root, "lib", "src", "main", "kotlin")
	libTest := filepath.Join(root, "lib", "src", "test", "kotlin")
	generated := filepath.Join(root, "lib", "build", "generated", "main")
	for _, dir := range []string{modelDir, appMain, appTest, libMain, libTest, generated} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "settings.gradle.kts"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(struct {
		Schema   int                   `json:"schema"`
		Projects []gradlemodel.Project `json:"projects"`
	}{Schema: 2, Projects: []gradlemodel.Project{
		{Path: ":app", Dir: filepath.Join(root, "app"), SourceSets: []gradlemodel.SourceSet{
			{Name: "main", Kind: "main", SourceDirs: []string{appMain}, ProjectDeps: []string{":lib"}, JvmTarget: "17"},
			{Name: "test", Kind: "test", SourceDirs: []string{appTest}},
		}},
		{Path: ":lib", Dir: filepath.Join(root, "lib"), SourceSets: []gradlemodel.SourceSet{
			{Name: "main", Kind: "main", SourceDirs: []string{libMain}, GeneratedSourceDirs: []string{generated}, JvmTarget: "21"},
			{Name: "test", Kind: "test", SourceDirs: []string{libTest}},
		}},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modelDir, "model.json"), body, 0o644); err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name     string
		explicit string
		disabled bool
	}{
		{name: "discovered"},
		{name: "explicit", explicit: modelDir},
		{name: "explicit overrides disabled discovery", explicit: modelDir, disabled: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dirs, generatedDirs, target := loadGradleCompileContext([]string{appMain}, test.explicit, test.disabled)
			if want := []string{appMain, libMain, generated}; !reflect.DeepEqual(dirs, want) {
				t.Fatalf("model roots = %v, want %v", dirs, want)
			}
			if !reflect.DeepEqual(generatedDirs, []string{generated}) || target != "21" {
				t.Fatalf("generated=%v target=%q", generatedDirs, target)
			}
			compileDirs := oracle.FilterFIRSourceDirs(oracle.UnionSourceDirs(oracle.FindSourceDirs([]string{appMain}), dirs))
			if !reflect.DeepEqual(compileDirs, dirs) {
				t.Fatalf("merged compilation roots = %v, want %v", compileDirs, dirs)
			}
			flags := freshScanFlags(t)
			flags.modelSourceDirs, flags.modelGeneratedSourceDirs, flags.modelJvmTarget = dirs, generatedDirs, target
			args := buildDaemonAnalyzeArgs(flags, []string{appMain})
			if !reflect.DeepEqual(args.Paths, []string{appMain}) || !reflect.DeepEqual(args.OracleSourceDirs, dirs) {
				t.Fatalf("daemon scan paths = %v, context roots = %v", args.Paths, args.OracleSourceDirs)
			}
		})
	}
	if dirs, generatedDirs, target := loadGradleCompileContext([]string{appMain}, "", true); len(dirs) != 0 || len(generatedDirs) != 0 || target != "" {
		t.Fatalf("disabled context: dirs=%v generated=%v target=%q", dirs, generatedDirs, target)
	}
	var verbose bytes.Buffer
	dirs, _, _ := loadGradleCompileContext([]string{filepath.Join(root, "unmodeled")}, modelDir, false, &verbose)
	if want := []string{appMain, appTest, libMain, generated, libTest}; !reflect.DeepEqual(dirs, want) {
		t.Fatalf("fallback roots = %v, want %v", dirs, want)
	}
	if !strings.Contains(verbose.String(), "no source sets match scan paths; using all model source roots") {
		t.Fatalf("missing fallback diagnostic: %q", verbose.String())
	}
}

func TestCompileContextOnlyFileNeverYieldsFindings(t *testing.T) {
	root := t.TempDir()
	appRoot := filepath.Join(root, "app", "src", "main", "kotlin")
	libRoot := filepath.Join(root, "lib", "src", "main", "kotlin")
	for _, dir := range []string{appRoot, libRoot} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		writeContractFile(t, filepath.Join(dir, "Example.kt"), "class Example\n")
	}
	model := &gradlemodel.Model{Projects: []gradlemodel.Project{
		{Path: ":app", SourceSets: []gradlemodel.SourceSet{{Kind: "main", SourceDirs: []string{appRoot}, ProjectDeps: []string{":lib"}}}},
		{Path: ":lib", SourceSets: []gradlemodel.SourceSet{{Kind: "main", SourceDirs: []string{libRoot}}}},
	}}
	compileDirs := model.CompileSourceDirs([]string{appRoot})
	if !reflect.DeepEqual(compileDirs, []string{appRoot, libRoot}) {
		t.Fatalf("compilation roots = %v", compileDirs)
	}
	rule := api.FakeRule("CompileContextClass", api.WithNodeTypes("class_declaration"),
		api.WithCheck(func(ctx *api.Context) { ctx.EmitAt(1, 1, "class declared") }))
	for _, scanRoot := range []string{appRoot, libRoot} {
		result, err := pipeline.RunProject(context.Background(), pipeline.ProjectInput{Args: pipeline.ProjectArgs{
			Config: config.NewConfig(), Paths: []string{scanRoot}, OracleSourceDirs: compileDirs,
			ActiveRules: []*api.Rule{rule}, Format: "json",
		}})
		if err != nil {
			t.Fatal(err)
		}
		if result.FilesScanned != 1 || result.FindingsCount != 1 || !strings.Contains(string(result.JSON), filepath.Join(scanRoot, "Example.kt")) {
			t.Fatalf("context leaked into findings: files=%d findings=%d json=%s", result.FilesScanned, result.FindingsCount, result.JSON)
		}
	}
}
