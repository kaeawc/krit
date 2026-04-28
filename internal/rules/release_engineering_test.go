package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestBuildConfigDebugInLibrary(t *testing.T) {
	rule := buildRuleIndex()["BuildConfigDebugInLibrary"]
	if rule == nil {
		t.Fatal("BuildConfigDebugInLibrary rule not registered")
	}

	t.Run("library module triggers", func(t *testing.T) {
		moduleDir := filepath.Join(t.TempDir(), "lib")
		sourcePath := filepath.Join(moduleDir, "src", "main", "java", "com", "example", "BuildConfigDebugInLibrary.kt")
		writeModuleFile(t, filepath.Join(moduleDir, "build.gradle.kts"), `plugins {
    id("com.android.library")
    id("org.jetbrains.kotlin.android")
}`)
		writeModuleFile(t, sourcePath, `package com.example

fun logOnlyInDebug() {
    if (BuildConfig.DEBUG) {
        println("debug")
    }
}
`)

		file, err := scanner.ParseFile(sourcePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", sourcePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("application module is clean", func(t *testing.T) {
		moduleDir := filepath.Join(t.TempDir(), "app")
		sourcePath := filepath.Join(moduleDir, "src", "main", "java", "com", "example", "BuildConfigDebugInLibrary.kt")
		writeModuleFile(t, filepath.Join(moduleDir, "build.gradle.kts"), `plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
}

android {
    defaultConfig {
        applicationId = "com.example.app"
    }
}`)
		writeModuleFile(t, sourcePath, `package com.example

fun logOnlyInDebug() {
    if (BuildConfig.DEBUG) {
        println("debug")
    }
}
`)

		file, err := scanner.ParseFile(sourcePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", sourcePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestBuildConfigDebugInverted(t *testing.T) {
	rule := buildRuleIndex()["BuildConfigDebugInverted"]
	if rule == nil {
		t.Fatal("BuildConfigDebugInverted rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "release-engineering", "BuildConfigDebugInverted.kt")
	negativePath := filepath.Join(root, "negative", "release-engineering", "BuildConfigDebugInverted.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestAllProjectsBlock(t *testing.T) {
	rule := buildRuleIndex()["AllProjectsBlock"]
	if rule == nil {
		t.Fatal("AllProjectsBlock rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "release-engineering", "all-projects-block", "build.gradle.kts")
	negativePath := filepath.Join(root, "negative", "release-engineering", "all-projects-block", "build.gradle.kts")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestHardcodedEnvironmentName(t *testing.T) {
	rule := buildRuleIndex()["HardcodedEnvironmentName"]
	if rule == nil {
		t.Fatal("HardcodedEnvironmentName rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "release-engineering", "HardcodedEnvironmentName.kt")
	negativePath := filepath.Join(root, "negative", "release-engineering", "HardcodedEnvironmentName.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestConventionPluginDeadCode(t *testing.T) {
	registered := buildRuleIndex()["ConventionPluginDeadCode"]
	if registered == nil {
		t.Fatal("ConventionPluginDeadCode rule not registered")
	}

	root := fixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "release-engineering", "convention-plugin-dead-code")
	negativeDir := filepath.Join(root, "negative", "release-engineering", "convention-plugin-dead-code")

	t.Run("positive fixture triggers", func(t *testing.T) {
		findings := runConventionPluginDeadCodeRule(t, positiveDir)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "kotlin-library-conventions") {
			t.Fatalf("expected finding to mention plugin id, got %q", findings[0].Message)
		}
		if !strings.HasSuffix(filepath.ToSlash(findings[0].File), "/build-logic/src/main/kotlin/kotlin-library-conventions.gradle.kts") {
			t.Fatalf("expected finding to point at convention plugin file, got %q", findings[0].File)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		findings := runConventionPluginDeadCodeRule(t, negativeDir)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestVisibleForTestingCallerInNonTest(t *testing.T) {
	t.Run("same owner call is clean", func(t *testing.T) {
		findings := runVisibleForTestingCallerRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

class Feature {
    @VisibleForTesting
    fun rebuildForTests() = Unit

    fun render() {
        rebuildForTests()
    }
}
`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("qualified owner call triggers across files", func(t *testing.T) {
		findings := runVisibleForTestingCallerRule(t, map[string]string{
			"app/src/main/java/com/example/TestHooks.kt": `package com.example

object TestHooks {
    @VisibleForTesting
    fun resetForTests() = Unit
}
`,
			"app/src/main/java/com/example/Production.kt": `package com.example

fun production() {
    TestHooks.resetForTests()
}
`,
		})
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("unqualified cross-file call is skipped as unresolved", func(t *testing.T) {
		findings := runVisibleForTestingCallerRule(t, map[string]string{
			"app/src/main/java/com/example/TestHooks.kt": `package com.example

@VisibleForTesting
fun resetForTests() = Unit
`,
			"app/src/main/java/com/example/Production.kt": `package com.example

fun production() {
    resetForTests()
}
`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("strings and comments do not trigger", func(t *testing.T) {
		findings := runVisibleForTestingCallerRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

class Feature {
    @VisibleForTesting
    fun rebuildForTests() = Unit

    fun render() {
        val text = "rebuildForTests()"
        // rebuildForTests()
        println(text)
    }
}
`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("overload with incompatible arity is skipped", func(t *testing.T) {
		findings := runVisibleForTestingCallerRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

class Feature {
    fun deleteMessage(id: Long): Boolean = deleteMessage(id)

    @VisibleForTesting
    fun deleteMessage(id: Long, threadId: Long, notify: Boolean = true): Boolean = true
}
`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("different same-file owner is skipped", func(t *testing.T) {
		findings := runVisibleForTestingCallerRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

class TestHooks {
    @VisibleForTesting
    fun reset() = Unit
}

class Production {
    fun reset() = Unit

    fun run() {
        reset()
    }
}
`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("top-level same-file call is clean", func(t *testing.T) {
		findings := runVisibleForTestingCallerRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

@VisibleForTesting
fun resetForTests() = Unit

fun production() {
    resetForTests()
}
`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("test sources are skipped", func(t *testing.T) {
		findings := runVisibleForTestingCallerRule(t, map[string]string{
			"app/src/main/java/com/example/TestHooks.kt": `package com.example

object TestHooks {
    @VisibleForTesting
    fun resetForTests() = Unit
}
`,
			"app/src/test/java/com/example/ProductionTest.kt": `package com.example

fun productionTest() {
    TestHooks.resetForTests()
}
`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func runVisibleForTestingCallerRule(t *testing.T, files map[string]string) []scanner.Finding {
	t.Helper()
	registered := buildRuleIndex()["VisibleForTestingCallerInNonTest"]
	if registered == nil {
		t.Fatal("VisibleForTestingCallerInNonTest rule not registered")
	}

	root := t.TempDir()
	parsed := make([]*scanner.File, 0, len(files))
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		writeModuleFile(t, path, content)
		file, err := scanner.ParseFile(path)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", path, err)
		}
		parsed = append(parsed, file)
	}

	index := scanner.BuildIndex(parsed, 1)
	ctx := &v2.Context{
		Rule:      registered,
		CodeIndex: index,
		Collector: scanner.NewFindingCollector(0),
	}
	registered.Check(ctx)
	return v2.ContextFindings(ctx)
}

func TestTimberTreeNotPlanted(t *testing.T) {
	t.Run("flags first production Timber usage when no startup plant exists", func(t *testing.T) {
		findings := runTimberTreeNotPlantedRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

import timber.log.Timber

fun logIt() {
    Timber.d("hello")
}
`,
		}, nil)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 6 {
			t.Fatalf("finding line = %d, want first Timber call line 6", findings[0].Line)
		}
	})

	t.Run("accepts Application onCreate planting", func(t *testing.T) {
		findings := runTimberTreeNotPlantedRule(t, map[string]string{
			"app/src/main/java/com/example/App.kt": `package com.example

import android.app.Application
import timber.log.Timber

class App : Application() {
    override fun onCreate() {
        super.onCreate()
        Timber.plant(Timber.DebugTree())
    }
}
`,
			"app/src/main/java/com/example/Feature.kt": `package com.example

import timber.log.Timber

fun logIt() {
    Timber.e("hello")
}
`,
		}, nil)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("supports multiline qualified Timber calls", func(t *testing.T) {
		findings := runTimberTreeNotPlantedRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

import timber.log.Timber

fun logIt() {
    Timber
        .d("hello")
}
`,
		}, nil)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("supports fully qualified Timber receiver", func(t *testing.T) {
		findings := runTimberTreeNotPlantedRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

fun logIt() {
    timber.log.Timber.d("hello")
}
`,
		}, nil)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("skips unimported unresolved Timber receiver", func(t *testing.T) {
		findings := runTimberTreeNotPlantedRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

fun logIt() {
    Timber.d("ambiguous")
}
`,
		}, nil)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("ignores local Timber object", func(t *testing.T) {
		findings := runTimberTreeNotPlantedRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

object Timber {
    fun d(msg: String) = Unit
    fun plant(tree: Any) = Unit
}

fun logIt() {
    Timber.d("not timber.log.Timber")
}
`,
		}, nil)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("ignores comments strings and unrelated methods", func(t *testing.T) {
		findings := runTimberTreeNotPlantedRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

class Logger {
    fun d(msg: String) = Unit
}

fun logIt(logger: Logger) {
    val text = "Timber.d(\"hello\")"
    // Timber.e("hello")
    logger.d(text)
}
`,
		}, nil)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non Application plant does not satisfy production usage", func(t *testing.T) {
		findings := runTimberTreeNotPlantedRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

import timber.log.Timber

class Feature {
    fun start() {
        Timber.plant(Timber.DebugTree())
        Timber.i("hello")
    }
}
`,
		}, nil)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("test source plant does not satisfy production usage", func(t *testing.T) {
		findings := runTimberTreeNotPlantedRule(t, map[string]string{
			"app/src/test/java/com/example/TestApp.kt": `package com.example

import android.app.Application
import timber.log.Timber

class TestApp : Application() {
    override fun onCreate() {
        Timber.plant(Timber.DebugTree())
    }
}
`,
			"app/src/main/java/com/example/Feature.kt": `package com.example

import timber.log.Timber

fun logIt() {
    Timber.w("hello")
}
`,
		}, nil)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("supports alias and member imports", func(t *testing.T) {
		findings := runTimberTreeNotPlantedRule(t, map[string]string{
			"app/src/main/java/com/example/Alias.kt": `package com.example

import timber.log.Timber as T

fun aliasLog() {
    T.w("hello")
}
`,
			"app/src/main/java/com/example/Member.kt": `package com.example

import timber.log.Timber.d

fun memberLog() {
    d("hello")
}
`,
		}, nil)
		if len(findings) != 1 {
			t.Fatalf("expected one project-level finding, got %d", len(findings))
		}
	})

	t.Run("resolved unrelated Timber target is skipped", func(t *testing.T) {
		findings := runTimberTreeNotPlantedRule(t, map[string]string{
			"app/src/main/java/com/example/Feature.kt": `package com.example

import timber.log.Timber

fun logIt() {
    Timber.d("hello")
}
`,
		}, map[string]map[string]string{
			"app/src/main/java/com/example/Feature.kt": {
				"Timber.d": "com.example.Timber.d",
			},
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func runTimberTreeNotPlantedRule(t *testing.T, files map[string]string, fakeTargets map[string]map[string]string) []scanner.Finding {
	t.Helper()
	registered := buildRuleIndex()["TimberTreeNotPlanted"]
	if registered == nil {
		t.Fatal("TimberTreeNotPlanted rule not registered")
	}

	root := t.TempDir()
	parsed := make([]*scanner.File, 0, len(files))
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		writeModuleFile(t, path, content)
		file, err := scanner.ParseFile(path)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", path, err)
		}
		parsed = append(parsed, file)
	}

	index := scanner.BuildIndex(parsed, 1)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel(parsed, 1)
	var typedResolver typeinfer.TypeResolver = resolver
	if fakeTargets != nil {
		fake := oracle.NewFakeOracle()
		for _, file := range parsed {
			targets := fakeTargets[relativeToRoot(t, root, file.Path)]
			if len(targets) == 0 {
				continue
			}
			fake.CallTargets[file.Path] = make(map[string]string)
			file.FlatWalkNodes(0, "call_expression", func(call uint32) {
				text := file.FlatNodeText(call)
				for needle, target := range targets {
					if strings.Contains(text, needle) {
						key := fmt.Sprintf("%d:%d", file.FlatRow(call)+1, file.FlatCol(call)+1)
						fake.CallTargets[file.Path][key] = target
					}
				}
			})
		}
		typedResolver = oracle.NewCompositeResolver(fake, resolver)
	}

	ctx := &v2.Context{
		Rule:      registered,
		CodeIndex: index,
		Resolver:  typedResolver,
		Collector: scanner.NewFindingCollector(0),
	}
	registered.Check(ctx)
	return v2.ContextFindings(ctx)
}

func relativeToRoot(t *testing.T, root, path string) string {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("filepath.Rel(%s, %s): %v", root, path, err)
	}
	return filepath.ToSlash(rel)
}

func TestOpenForTestingCallerInNonTest(t *testing.T) {
	root := fixtureRoot(t)

	t.Run("positive fixture triggers", func(t *testing.T) {
		findings := runOpenForTestingCallerRule(t, map[string]string{
			"app/src/main/java/com/example/OpenForTestingCallerInNonTest.kt": mustReadFixture(t, filepath.Join(root, "positive", "release-engineering", "OpenForTestingCallerInNonTest.kt")),
		})
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		findings := runOpenForTestingCallerRule(t, map[string]string{
			"app/src/main/java/com/example/OpenForTestingCallerInNonTest.kt": mustReadFixture(t, filepath.Join(root, "negative", "release-engineering", "OpenForTestingCallerInNonTest.kt")),
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("cross file explicit import triggers", func(t *testing.T) {
		findings := runOpenForTestingCallerRule(t, map[string]string{
			"core/src/main/java/com/example/testing/BaseForTests.kt": `package com.example.testing

@OpenForTesting
open class BaseForTests
`,
			"app/src/main/java/com/example/app/ProductionSubclass.kt": `package com.example.app

import com.example.testing.BaseForTests

class ProductionSubclass : BaseForTests()
`,
		})
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 5 {
			t.Fatalf("expected finding on supertype line 5, got %d", findings[0].Line)
		}
	})

	t.Run("cross file same package without import triggers", func(t *testing.T) {
		findings := runOpenForTestingCallerRule(t, map[string]string{
			"core/src/main/java/com/example/BaseForTests.kt": `package com.example

@OpenForTesting
open class BaseForTests
`,
			"app/src/main/java/com/example/ProductionSubclass.kt": `package com.example

class ProductionSubclass : BaseForTests()
`,
		})
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("import alias to nested class triggers", func(t *testing.T) {
		findings := runOpenForTestingCallerRule(t, map[string]string{
			"core/src/main/java/com/example/testing/Outer.kt": `package com.example.testing

class Outer {
    @OpenForTesting
    open class NestedBase
}
`,
			"app/src/main/java/com/example/app/ProductionSubclass.kt": `package com.example.app

import com.example.testing.Outer.NestedBase as TestBase

class ProductionSubclass : TestBase()
`,
		})
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("test sources are skipped", func(t *testing.T) {
		findings := runOpenForTestingCallerRule(t, map[string]string{
			"app/src/main/java/com/example/BaseForTests.kt": `package com.example

@OpenForTesting
open class BaseForTests
`,
			"app/src/test/java/com/example/ProductionSubclassTest.kt": `package com.example

import com.example.BaseForTests

class ProductionSubclassTest : BaseForTests()
`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("unrelated same simple name is skipped", func(t *testing.T) {
		findings := runOpenForTestingCallerRule(t, map[string]string{
			"core/src/main/java/com/example/testing/BaseForTests.kt": `package com.example.testing

@OpenForTesting
open class BaseForTests
`,
			"app/src/main/java/com/example/app/BaseForTests.kt": `package com.example.app

open class BaseForTests
`,
			"app/src/main/java/com/example/app/ProductionSubclass.kt": `package com.example.app

class ProductionSubclass : BaseForTests()
`,
		})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func runOpenForTestingCallerRule(t *testing.T, files map[string]string) []scanner.Finding {
	t.Helper()
	registered := buildRuleIndex()["OpenForTestingCallerInNonTest"]
	if registered == nil {
		t.Fatal("OpenForTestingCallerInNonTest rule not registered")
	}

	root := t.TempDir()
	parsed := make([]*scanner.File, 0, len(files))
	for rel, content := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		writeModuleFile(t, path, content)
		file, err := scanner.ParseFile(path)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", path, err)
		}
		parsed = append(parsed, file)
	}

	index := scanner.BuildIndex(parsed, 1)
	ctx := &v2.Context{
		Rule:      registered,
		CodeIndex: index,
		Collector: scanner.NewFindingCollector(0),
	}
	registered.Check(ctx)
	return v2.ContextFindings(ctx)
}

func mustReadFixture(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return string(content)
}

func TestTestFixtureAccessedFromProduction(t *testing.T) {
	registered := buildRuleIndex()["TestFixtureAccessedFromProduction"]
	if registered == nil {
		t.Fatal("TestFixtureAccessedFromProduction rule not registered")
	}

	root := fixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "release-engineering", "test-fixture-accessed-from-production")
	negativeDir := filepath.Join(root, "negative", "release-engineering", "test-fixture-accessed-from-production")

	t.Run("positive fixture triggers on imports and references", func(t *testing.T) {
		findings := runTestFixtureAccessedFromProductionRule(t, registered, positiveDir)
		if len(findings) < 5 {
			t.Fatalf("expected at least 5 findings, got %d: %#v", len(findings), findings)
		}
		if !hasFindingAtPathSuffix(findings, "app/src/main/kotlin/com/example/prod/Prod.kt", 3) {
			t.Fatalf("expected import finding in Prod.kt, got %#v", findings)
		}
		if !hasFindingAtPathSuffix(findings, "app/src/main/java/com/example/prod/JavaProd.java", 3) {
			t.Fatalf("expected Java import finding in JavaProd.java, got %#v", findings)
		}
		if !hasFindingAtPathSuffix(findings, "app/src/main/kotlin/com/example/SamePackageProd.kt", 4) {
			t.Fatalf("expected same-package reference finding, got %#v", findings)
		}
		var sawExact, sawFallback bool
		for _, finding := range findings {
			if finding.Confidence >= 0.95 {
				sawExact = true
			}
			if finding.Confidence == 0.80 {
				sawFallback = true
			}
		}
		if !sawExact || !sawFallback {
			t.Fatalf("expected exact and package fallback confidence findings, got %#v", findings)
		}
	})

	t.Run("negative fixture avoids comments strings tests aliases generated and unresolved names", func(t *testing.T) {
		findings := runTestFixtureAccessedFromProductionRule(t, registered, negativeDir)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %#v", len(findings), findings)
		}
	})
}

func runTestFixtureAccessedFromProductionRule(t *testing.T, registered *v2.Rule, projectDir string) []scanner.Finding {
	t.Helper()
	var kotlinFiles []*scanner.File
	var javaFiles []*scanner.File
	var parsedFiles []*scanner.File

	err := filepath.Walk(projectDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info == nil || info.IsDir() {
			return err
		}
		switch {
		case strings.HasSuffix(path, ".kt"), strings.HasSuffix(path, ".kts"):
			file, parseErr := scanner.ParseFile(path)
			if parseErr != nil {
				return parseErr
			}
			kotlinFiles = append(kotlinFiles, file)
			parsedFiles = append(parsedFiles, file)
		case strings.HasSuffix(path, ".java"):
			file, parseErr := scanner.ParseJavaFile(path)
			if parseErr != nil {
				return parseErr
			}
			javaFiles = append(javaFiles, file)
			parsedFiles = append(parsedFiles, file)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", projectDir, err)
	}

	index := scanner.BuildIndex(kotlinFiles, 1, javaFiles...)
	ctx := &v2.Context{
		Rule:        registered,
		CodeIndex:   index,
		ParsedFiles: parsedFiles,
		Collector:   scanner.NewFindingCollector(0),
	}
	registered.Check(ctx)
	return v2.ContextFindings(ctx)
}

func hasFindingAtPathSuffix(findings []scanner.Finding, suffix string, line int) bool {
	for _, finding := range findings {
		if strings.HasSuffix(filepath.ToSlash(finding.File), suffix) && finding.Line == line {
			return true
		}
	}
	return false
}

func TestCommentedOutCodeBlock(t *testing.T) {
	rule := buildRuleIndex()["CommentedOutCodeBlock"]
	if rule == nil {
		t.Fatal("CommentedOutCodeBlock rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "release-engineering", "CommentedOutCodeBlock.kt")
	negativePath := filepath.Join(root, "negative", "release-engineering", "CommentedOutCodeBlock.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestGradleBuildContainsTodo(t *testing.T) {
	rule := buildRuleIndex()["GradleBuildContainsTodo"]
	if rule == nil {
		t.Fatal("GradleBuildContainsTodo rule not registered")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "release-engineering", "gradle-build-contains-todo", "build.gradle.kts")
	negativePath := filepath.Join(root, "negative", "release-engineering", "gradle-build-contains-todo", "build.gradle.kts")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func runConventionPluginDeadCodeRule(t *testing.T, projectDir string) []scanner.Finding {
	t.Helper()

	graph, err := module.DiscoverModules(projectDir)
	if err != nil {
		t.Fatalf("DiscoverModules(%s): %v", projectDir, err)
	}
	if graph == nil {
		t.Fatalf("expected modules to be discovered in %s", projectDir)
	}

	registered := buildRuleIndex()["ConventionPluginDeadCode"]
	if registered == nil {
		t.Fatal("ConventionPluginDeadCode rule not registered")
	}
	pmi := &module.PerModuleIndex{Graph: graph}
	ctx := &v2.Context{ModuleIndex: pmi, Collector: scanner.NewFindingCollector(0)}
	registered.Check(ctx)
	_ = rules.ConventionPluginDeadCodeRule{} // keep import used
	return v2.ContextFindings(ctx)
}

func writeModuleFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("WriteFile(%s): %v", path, err)
	}
}

func TestCommentedOutImport(t *testing.T) {
	rule := buildRuleIndex()["CommentedOutImport"]
	if rule == nil {
		t.Fatal("CommentedOutImport rule not registered")
	}

	root := fixtureRoot(t)

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "positive", "release-engineering", "CommentedOutImport.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "negative", "release-engineering", "CommentedOutImport.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestDebugToastInProduction(t *testing.T) {
	rule := buildRuleIndex()["DebugToastInProduction"]
	if rule == nil {
		t.Fatal("DebugToastInProduction rule not registered")
	}

	root := fixtureRoot(t)

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "positive", "release-engineering", "DebugToastInProduction.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "negative", "release-engineering", "DebugToastInProduction.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestMergeConflictMarkerLeftover(t *testing.T) {
	rule := buildRuleIndex()["MergeConflictMarkerLeftover"]
	if rule == nil {
		t.Fatal("MergeConflictMarkerLeftover rule not registered")
	}

	root := fixtureRoot(t)

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "positive", "release-engineering", "MergeConflictMarkerLeftover.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) < 1 {
			t.Fatalf("expected at least 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "negative", "release-engineering", "MergeConflictMarkerLeftover.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestPrintlnInProduction(t *testing.T) {
	rule := buildRuleIndex()["PrintlnInProduction"]
	if rule == nil {
		t.Fatal("PrintlnInProduction rule not registered")
	}

	root := fixtureRoot(t)

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "positive", "release-engineering", "PrintlnInProduction.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "negative", "release-engineering", "PrintlnInProduction.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestPrintStackTraceInProduction(t *testing.T) {
	rule := buildRuleIndex()["PrintStackTraceInProduction"]
	if rule == nil {
		t.Fatal("PrintStackTraceInProduction rule not registered")
	}

	root := fixtureRoot(t)

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "positive", "release-engineering", "PrintStackTraceInProduction.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "negative", "release-engineering", "PrintStackTraceInProduction.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestHardcodedLocalhostUrl(t *testing.T) {
	rule := buildRuleIndex()["HardcodedLocalhostUrl"]
	if rule == nil {
		t.Fatal("HardcodedLocalhostUrl rule not registered")
	}

	root := fixtureRoot(t)

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "positive", "release-engineering", "HardcodedLocalhostUrl.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "negative", "release-engineering", "HardcodedLocalhostUrl.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestTestOnlyImportInProduction(t *testing.T) {
	rule := buildRuleIndex()["TestOnlyImportInProduction"]
	if rule == nil {
		t.Fatal("TestOnlyImportInProduction rule not registered")
	}

	root := fixtureRoot(t)

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "positive", "release-engineering", "TestOnlyImportInProduction.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "negative", "release-engineering", "TestOnlyImportInProduction.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestNonAsciiIdentifier(t *testing.T) {
	rule := buildRuleIndex()["NonAsciiIdentifier"]
	if rule == nil {
		t.Fatal("NonAsciiIdentifier rule not registered")
	}

	root := fixtureRoot(t)

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "positive", "release-engineering", "NonAsciiIdentifier.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "negative", "release-engineering", "NonAsciiIdentifier.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestHardcodedLogTag(t *testing.T) {
	rule := buildRuleIndex()["HardcodedLogTag"]
	if rule == nil {
		t.Fatal("HardcodedLogTag rule not registered")
	}

	root := fixtureRoot(t)

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "positive", "release-engineering", "HardcodedLogTag.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(filepath.Join(root, "negative", "release-engineering", "HardcodedLogTag.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRule(t, rule, file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}
