package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/rules"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
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
