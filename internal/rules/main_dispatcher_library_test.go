package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func runMainDispatcherRule(t *testing.T, root string, modules map[string]*module.Module, files []*scanner.File) []scanner.Finding {
	t.Helper()
	graph := buildGraph(root, modules)
	pmi := module.BuildPerModuleIndex(graph, files, 1)
	rule := &MainDispatcherInLibraryCodeRule{
		BaseRule: BaseRule{RuleName: "MainDispatcherInLibraryCode", RuleSetName: "coroutines", Sev: "warning"},
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel(files, 1)
	ctx := &api.Context{ModuleIndex: pmi, Resolver: resolver, Collector: scanner.NewFindingCollector(0)}
	rule.check(ctx)
	return api.ContextFindings(ctx)
}

func TestMainDispatcherInLibraryCode_CatalogAndTransitiveDependenciesCount(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("bare/build.gradle.kts", `plugins { id("com.android.library") }`)
	write("catalog/build.gradle.kts", "plugins { alias(libs.plugins.android.library) }\ndependencies { implementation(libs.kotlinx.coroutines.android) }")
	write("viaModule/build.gradle.kts", "plugins { id(\"com.android.library\") }\ndependencies { implementation(project(\":catalog\")) }")
	src := `package p
import kotlinx.coroutines.Dispatchers
fun f() = Dispatchers.Main
`
	var files []*scanner.File
	modules := map[string]*module.Module{}
	for _, name := range []string{"bare", "catalog", "viaModule"} {
		dir := filepath.Join(root, name)
		files = append(files, writeAndParse(t, filepath.Join(dir, "src", "main", "kotlin"), "F.kt", src))
		m := &module.Module{Path: ":" + name, Dir: dir}
		if name == "viaModule" {
			m.Dependencies = []module.Dependency{{ModulePath: ":catalog", Configuration: "implementation"}}
		}
		modules[":"+name] = m
	}
	findings := runMainDispatcherRule(t, root, modules, files)
	if len(findings) != 1 {
		t.Fatalf("expected only the bare module to be flagged, got %v", findings)
	}
}
