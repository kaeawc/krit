package rules

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// helper: write a .kt file, parse it, return the *scanner.File
func writeAndParse(t *testing.T, dir, name, content string) *scanner.File {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile(%s): %v", path, err)
	}
	return f
}

func buildGraph(root string, modules map[string]*module.Module) *module.ModuleGraph {
	g := module.NewModuleGraph(root)
	for k, v := range modules {
		g.Modules[k] = v
	}
	return g
}

func TestModuleDeadCode_TrulyDead(t *testing.T) {
	root := t.TempDir()
	libSrc := filepath.Join(root, "lib", "src", "main", "kotlin")

	// Symbol in :lib not used anywhere
	libFile := writeAndParse(t, libSrc, "Unused.kt", `fun unusedHelper(): Int = 42`)

	graph := buildGraph(root, map[string]*module.Module{
		":lib": {Path: ":lib", Dir: filepath.Join(root, "lib")},
	})
	// No consumers
	pmi := module.BuildPerModuleIndex(graph, []*scanner.File{libFile}, 1)

	rule := &ModuleDeadCodeRule{
		BaseRule: BaseRule{RuleName: "ModuleDeadCode", RuleSetName: "dead-code", Sev: "warning"},
	}
	ctx := &v2.Context{
		ModuleIndex: pmi,
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	findings := v2.ContextFindings(ctx)

	found := false
	for _, f := range findings {
		if f.Rule == "ModuleDeadCode" && contains(f.Message, "unusedHelper") && contains(f.Message, "truly-dead") {
			t.Logf("got expected finding: %s", f.Message)
		}
		if contains(f.Message, "unusedHelper") {
			found = true
		}
	}
	if !found {
		t.Error("expected a finding for unusedHelper but got none")
	}
}

func TestModuleDeadCode_CouldBeInternal(t *testing.T) {
	root := t.TempDir()
	libSrc := filepath.Join(root, "lib", "src", "main", "kotlin")

	// Public symbol used only within :lib (two files in same module)
	libFile1 := writeAndParse(t, libSrc, "Api.kt", `fun sharedUtil(): String = "hello"`)
	libFile2 := writeAndParse(t, libSrc, "Consumer.kt", `fun doStuff() { sharedUtil() }`)

	graph := buildGraph(root, map[string]*module.Module{
		":lib": {Path: ":lib", Dir: filepath.Join(root, "lib")},
	})
	// No consumers of :lib
	pmi := module.BuildPerModuleIndex(graph, []*scanner.File{libFile1, libFile2}, 1)

	rule := &ModuleDeadCodeRule{
		BaseRule: BaseRule{RuleName: "ModuleDeadCode", RuleSetName: "dead-code", Sev: "warning"},
	}
	ctx := &v2.Context{
		ModuleIndex: pmi,
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	findings := v2.ContextFindings(ctx)

	found := false
	for _, f := range findings {
		if contains(f.Message, "sharedUtil") && contains(f.Message, "internal") {
			found = true
		}
	}
	if !found {
		t.Error("expected a could-be-internal finding for sharedUtil")
	}
}

func TestModuleDeadCode_UsedByConsumer(t *testing.T) {
	root := t.TempDir()
	libSrc := filepath.Join(root, "lib", "src", "main", "kotlin")
	appSrc := filepath.Join(root, "app", "src", "main", "kotlin")

	// Symbol in :lib used by :app
	libFile := writeAndParse(t, libSrc, "Api.kt", `fun greet(): String = "hi"`)
	appFile := writeAndParse(t, appSrc, "Main.kt", `fun main() { greet() }`)

	graph := buildGraph(root, map[string]*module.Module{
		":lib": {Path: ":lib", Dir: filepath.Join(root, "lib")},
		":app": {Path: ":app", Dir: filepath.Join(root, "app")},
	})
	graph.Consumers[":lib"] = []string{":app"}

	pmi := module.BuildPerModuleIndex(graph, []*scanner.File{libFile, appFile}, 1)

	rule := &ModuleDeadCodeRule{
		BaseRule: BaseRule{RuleName: "ModuleDeadCode", RuleSetName: "dead-code", Sev: "warning"},
	}
	ctx := &v2.Context{
		ModuleIndex: pmi,
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	findings := v2.ContextFindings(ctx)

	for _, f := range findings {
		if contains(f.Message, "greet") {
			t.Errorf("greet should NOT be flagged (used by :app consumer), but got: %s", f.Message)
		}
	}
}

func TestModuleDeadCode_PublishedModuleSkip(t *testing.T) {
	root := t.TempDir()
	libSrc := filepath.Join(root, "lib", "src", "main", "kotlin")

	// Symbol in a published module — should be skipped entirely
	libFile := writeAndParse(t, libSrc, "Api.kt", `fun publicApi(): String = "sdk"`)

	graph := buildGraph(root, map[string]*module.Module{
		":lib": {Path: ":lib", Dir: filepath.Join(root, "lib"), IsPublished: true},
	})

	pmi := module.BuildPerModuleIndex(graph, []*scanner.File{libFile}, 1)

	rule := &ModuleDeadCodeRule{
		BaseRule: BaseRule{RuleName: "ModuleDeadCode", RuleSetName: "dead-code", Sev: "warning"},
	}
	ctx := &v2.Context{
		ModuleIndex: pmi,
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	findings := v2.ContextFindings(ctx)

	for _, f := range findings {
		if contains(f.Message, "publicApi") {
			t.Errorf("publicApi in published module should be skipped, but got: %s", f.Message)
		}
	}
}

func TestDeadCode_SkipsGeneratedDIBindings(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "app", "src", "main", "kotlin")
	file := writeAndParse(t, src, "MetroBindings.kt", `
package test

import dev.zacsweers.metro.ContributesBinding
import dev.zacsweers.metro.ContributesTo
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.IntoSet
import dev.zacsweers.metro.Provides
import dev.zacsweers.metro.Qualifier

interface Service

@ContributesBinding(AppScope::class)
@Inject
class MetroService : Service

@Qualifier
annotation class ApplicationContext

@ContributesTo(AppScope::class)
interface ApplicationModule {
    companion object {
        @Provides
        @IntoSet
        fun provideInitializer(): () -> Unit = {}
    }
}

fun unusedHelper(): Int = 42
`)
	index := scanner.BuildIndex([]*scanner.File{file}, 1)
	rule := &DeadCodeRule{
		BaseRule:                BaseRule{RuleName: "DeadCode", RuleSetName: "dead-code", Sev: "warning"},
		IgnoreCommentReferences: true,
	}
	ctx := &v2.Context{
		CodeIndex: index,
		Collector: scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	findings := v2.ContextFindings(ctx)

	for _, f := range findings {
		for _, generated := range []string{"MetroService", "ApplicationContext", "ApplicationModule", "provideInitializer"} {
			if contains(f.Message, generated) {
				t.Fatalf("expected generated DI symbol %s to be skipped, got finding: %s", generated, f.Message)
			}
		}
	}
	foundPlainUnused := false
	for _, f := range findings {
		if contains(f.Message, "unusedHelper") {
			foundPlainUnused = true
		}
	}
	if !foundPlainUnused {
		t.Fatal("expected ordinary unusedHelper to remain flagged")
	}
}

func TestModuleDeadCode_SkipsGeneratedDIBindings(t *testing.T) {
	root := t.TempDir()
	appSrc := filepath.Join(root, "app", "src", "main", "kotlin")
	file := writeAndParse(t, appSrc, "MetroBindings.kt", `
package test

import dev.zacsweers.metro.ContributesBinding
import dev.zacsweers.metro.ContributesTo
import dev.zacsweers.metro.Inject
import dev.zacsweers.metro.IntoSet
import dev.zacsweers.metro.Provides
import dev.zacsweers.metro.Qualifier

interface Service

@ContributesBinding(AppScope::class)
@Inject
class MetroService : Service

@Qualifier
annotation class ApplicationContext

@ContributesTo(AppScope::class)
interface ApplicationModule {
    companion object {
        @Provides
        @IntoSet
        fun provideInitializer(): () -> Unit = {}
    }
}

fun unusedHelper(): Int = 42
`)
	graph := buildGraph(root, map[string]*module.Module{
		":app": {Path: ":app", Dir: filepath.Join(root, "app")},
	})
	pmi := module.BuildPerModuleIndex(graph, []*scanner.File{file}, 1)

	rule := &ModuleDeadCodeRule{
		BaseRule: BaseRule{RuleName: "ModuleDeadCode", RuleSetName: "dead-code", Sev: "warning"},
	}
	ctx := &v2.Context{
		ModuleIndex: pmi,
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	findings := v2.ContextFindings(ctx)

	for _, f := range findings {
		for _, generated := range []string{"MetroService", "ApplicationContext", "ApplicationModule", "provideInitializer"} {
			if contains(f.Message, generated) {
				t.Fatalf("expected generated DI symbol %s to be skipped, got finding: %s", generated, f.Message)
			}
		}
	}
	foundPlainUnused := false
	for _, f := range findings {
		if contains(f.Message, "unusedHelper") {
			foundPlainUnused = true
		}
	}
	if !foundPlainUnused {
		t.Fatal("expected ordinary unusedHelper to remain flagged")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
