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

func writeAndParseJava(t *testing.T, dir, name, content string) *scanner.File {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseJavaFile(path)
	if err != nil {
		t.Fatalf("ParseJavaFile(%s): %v", path, err)
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

func TestModuleDeadCode_SameFileReferencesAreUsedWithinModule(t *testing.T) {
	root := t.TempDir()
	appSrc := filepath.Join(root, "app", "src", "main", "kotlin")
	appFile := writeAndParse(t, appSrc, "Screen.kt", `
package app

fun Screen() {
    Section()
}

fun Section() {}
`)
	graph := buildGraph(root, map[string]*module.Module{
		":app": {Path: ":app", Dir: filepath.Join(root, "app")},
	})
	pmi := module.BuildPerModuleIndex(graph, []*scanner.File{appFile}, 1)

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
		if contains(f.Message, "Section") {
			found = true
			if contains(f.Message, "not used by any module") {
				t.Fatalf("expected same-file reference to avoid truly-dead classification, got: %s", f.Message)
			}
		}
	}
	if !found {
		t.Fatal("expected same-file public helper to remain a module-visibility finding")
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

func TestModuleDeadCode_JavaMethodUsedByKotlinConsumer(t *testing.T) {
	root := t.TempDir()
	libSrc := filepath.Join(root, "lib", "src", "main", "java", "com", "example")
	appSrc := filepath.Join(root, "app", "src", "main", "kotlin")

	libFile := writeAndParseJava(t, libSrc, "JavaApi.java", `package com.example;

public class JavaApi {
  public void usedFromKotlin() {}
  public void trulyUnused() {}
}
`)
	appFile := writeAndParse(t, appSrc, "Main.kt", `package app

import com.example.JavaApi

fun callApi(api: JavaApi) {
    api.usedFromKotlin()
}
`)

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
		if contains(f.Message, "usedFromKotlin") {
			t.Fatalf("usedFromKotlin should be protected by Kotlin consumer reference, got: %s", f.Message)
		}
	}
	foundUnused := false
	for _, f := range findings {
		if contains(f.Message, "trulyUnused") {
			foundUnused = true
		}
	}
	if !foundUnused {
		t.Fatalf("expected trulyUnused Java method to be reported, got %+v", findings)
	}
}

func TestModuleDeadCode_KotlinPropertyUsedByJavaGetterConsumer(t *testing.T) {
	root := t.TempDir()
	libSrc := filepath.Join(root, "lib", "src", "main", "kotlin")
	appSrc := filepath.Join(root, "app", "src", "main", "java", "com", "example", "app")

	libFile := writeAndParse(t, libSrc, "KotlinProfile.kt", `package com.example

class KotlinProfile {
    val displayName: String = "Ada"
    val unusedName: String = "Lovelace"
}
`)
	appFile := writeAndParseJava(t, appSrc, "UseProfile.java", `package com.example.app;

import com.example.KotlinProfile;

public class UseProfile {
  public String render(KotlinProfile profile) {
    return profile.getDisplayName();
  }
}
`)

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
		if contains(f.Message, "displayName") {
			t.Fatalf("displayName should be protected by Java getter reference, got: %s", f.Message)
		}
	}
	foundUnused := false
	for _, f := range findings {
		if contains(f.Message, "unusedName") {
			foundUnused = true
		}
	}
	if !foundUnused {
		t.Fatalf("expected unusedName Kotlin property to be reported, got %+v", findings)
	}
}

func TestDeadCode_JavaXmlAndLifecycleEntriesAreConservative(t *testing.T) {
	root := t.TempDir()
	ktSrc := filepath.Join(root, "app", "src", "main", "kotlin")
	javaSrc := filepath.Join(root, "app", "src", "main", "java", "com", "example")
	layoutDir := filepath.Join(root, "app", "src", "main", "res", "layout")

	anchor := writeAndParse(t, ktSrc, "Anchor.kt", `package com.example
fun anchor() = Unit
`)
	viewFile := writeAndParseJava(t, javaSrc, "CustomWidget.java", `package com.example;
public class CustomWidget {
  public void onCreate(android.os.Bundle state) {}
  public void helper() {}
}
`)
	if err := os.MkdirAll(layoutDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(layoutDir, "widget.xml"), []byte(`<com.example.CustomWidget />`), 0o644); err != nil {
		t.Fatal(err)
	}

	index := scanner.BuildIndex([]*scanner.File{anchor}, 1, viewFile)
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
		if contains(f.Message, "CustomWidget") || contains(f.Message, "onCreate") {
			t.Fatalf("expected XML/lifecycle Java entries to be skipped, got: %s", f.Message)
		}
	}
	foundHelper := false
	for _, f := range findings {
		if contains(f.Message, "helper") {
			foundHelper = true
		}
	}
	if !foundHelper {
		t.Fatalf("expected ordinary Java helper method to remain reportable, got %+v", findings)
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

func TestModuleDeadCode_IgnoresGradleBuildScripts(t *testing.T) {
	root := t.TempDir()
	appDir := filepath.Join(root, "app")
	buildFile := writeAndParse(t, appDir, "build.gradle.kts", `
val gradleWorkerJvmArgs = listOf("-Xmx2g")

fun killKotlinCompileDaemon() = Unit
`)
	graph := buildGraph(root, map[string]*module.Module{
		":app": {Path: ":app", Dir: appDir},
	})
	pmi := module.BuildPerModuleIndex(graph, []*scanner.File{buildFile}, 1)

	rule := &ModuleDeadCodeRule{
		BaseRule: BaseRule{RuleName: "ModuleDeadCode", RuleSetName: "dead-code", Sev: "warning"},
	}
	ctx := &v2.Context{
		ModuleIndex: pmi,
		Collector:   scanner.NewFindingCollector(0),
	}
	rule.check(ctx)
	findings := v2.ContextFindings(ctx)
	if len(findings) != 0 {
		t.Fatalf("expected Gradle build script declarations to be ignored, got %d findings: %+v", len(findings), findings)
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
