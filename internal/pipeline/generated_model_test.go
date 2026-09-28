package pipeline

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestModelGeneratedDirsSuppressGoRulesUnlessIncluded(t *testing.T) {
	root := t.TempDir()
	modelGenerated := filepath.Join(root, "build", "parser")
	conventional := filepath.Join(root, "custom", "generated")
	modelFile := filepath.Join(modelGenerated, "Model.kt")
	conventionalFile := filepath.Join(conventional, "Conventional.kt")
	productFile := filepath.Join(root, "src", "Product.kt")
	for _, path := range []string{modelFile, conventionalFile, productFile} {
		writeKt(t, path, "class Example {}\n")
	}
	rule := api.FakeRule("GeneratedGoRule", api.WithNodeTypes("class_declaration"),
		api.WithCheck(func(ctx *api.Context) { ctx.EmitAt(1, 1, "class declared") }))
	for _, tc := range []struct {
		include bool
		want    int
	}{{false, 1}, {true, 3}} {
		parsed, err := (ParsePhase{}).Run(context.Background(), ParseInput{
			KotlinPaths: []string{modelFile, conventionalFile, productFile},
			ActiveRules: []*api.Rule{rule}, GeneratedSourceDirs: []string{modelGenerated},
			IncludeGenerated: tc.include,
		})
		if err != nil {
			t.Fatal(err)
		}
		out, err := (DispatchPhase{}).Run(context.Background(), IndexResult{ParseResult: parsed})
		if err != nil {
			t.Fatal(err)
		}
		if got := out.Findings.Len(); got != tc.want {
			t.Errorf("includeGenerated=%t: Go findings=%d, want %d", tc.include, got, tc.want)
		}
	}
	if !IsGeneratedSourcePath(modelFile, []string{modelGenerated}) || IsGeneratedSourcePath(filepath.Join(root, "build", "parser2", "File.kt"), []string{modelGenerated}) {
		t.Fatal("model generated directory boundary not respected")
	}
}

func TestFIRBuildLogicExclusionDoesNotSuppressGoRulePass(t *testing.T) {
	root := t.TempDir()
	writeKt(t, filepath.Join(root, ".git", "HEAD"), "ref: refs/heads/main")
	writeKt(t, filepath.Join(root, "settings.gradle.kts"), `pluginManagement { includeBuild("gradle/plugins") }`)
	dirs := []string{"app", "buildSrc", "build-logic", "gradle/plugins", "sample-app"}
	var paths, sourceDirs []string
	for _, dir := range dirs {
		if dir != "app" {
			writeKt(t, filepath.Join(root, dir, "settings.gradle.kts"), "")
		}
		path := filepath.Join(root, dir, "src", "Example.kt")
		writeKt(t, path, "class Example {}\n")
		paths = append(paths, path)
		sourceDirs = append(sourceDirs, filepath.Dir(path))
	}
	if got := oracle.FilterFIRSourceDirs(sourceDirs); len(got) != 2 {
		t.Fatalf("FIR source dirs = %v, want app and sample-app", got)
	}
	rule := api.FakeRule("BuildLogicGoRule", api.WithNodeTypes("class_declaration"),
		api.WithCheck(func(ctx *api.Context) { ctx.EmitAt(1, 1, "class declared") }))
	parsed, err := (ParsePhase{}).Run(context.Background(), ParseInput{KotlinPaths: paths, ActiveRules: []*api.Rule{rule}})
	if err != nil {
		t.Fatal(err)
	}
	result, err := (DispatchPhase{}).Run(context.Background(), IndexResult{ParseResult: parsed})
	if err != nil {
		t.Fatal(err)
	}
	if result.Findings.Len() != len(dirs) {
		t.Fatalf("Go rule findings = %d, want %d", result.Findings.Len(), len(dirs))
	}
}
