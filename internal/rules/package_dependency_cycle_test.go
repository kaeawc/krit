package rules

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/module"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestPackageDependencyCycleRule_FlagsCycleWithinModule(t *testing.T) {
	root := t.TempDir()
	appSrc := filepath.Join(root, "app", "src", "main", "kotlin")

	fileA := writeAndParse(t, filepath.Join(appSrc, "com", "example", "a"), "A.kt", `package com.example.a

import com.example.b.B

class A(private val dependency: B)
`)
	fileB := writeAndParse(t, filepath.Join(appSrc, "com", "example", "b"), "B.kt", `package com.example.b

import com.example.a.A

class B(private val dependency: A)
`)

	graph := buildGraph(root, map[string]*module.Module{
		":app": {Path: ":app", Dir: filepath.Join(root, "app")},
	})
	pmi := module.BuildPerModuleIndex(graph, []*scanner.File{fileA, fileB}, 1)

	rule := &PackageDependencyCycleRule{
		BaseRule: BaseRule{RuleName: "PackageDependencyCycle", RuleSetName: "architecture", Sev: "info"},
	}
	rule.SetModuleIndex(pmi)
	findings := rule.CheckModuleAware()

	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "com.example.a") || !strings.Contains(findings[0].Message, "com.example.b") {
		t.Fatalf("expected finding to mention both packages, got %q", findings[0].Message)
	}
	if !strings.Contains(findings[0].Message, ":app") {
		t.Fatalf("expected finding to mention module path, got %q", findings[0].Message)
	}
}

func TestPackageDependencyCycleRule_IgnoresDagWithinModule(t *testing.T) {
	root := t.TempDir()
	appSrc := filepath.Join(root, "app", "src", "main", "kotlin")

	fileA := writeAndParse(t, filepath.Join(appSrc, "com", "example", "a"), "A.kt", `package com.example.a

import com.example.b.B

class A(private val dependency: B)
`)
	fileB := writeAndParse(t, filepath.Join(appSrc, "com", "example", "b"), "B.kt", `package com.example.b

import com.example.c.C

class B(private val dependency: C)
`)
	fileC := writeAndParse(t, filepath.Join(appSrc, "com", "example", "c"), "C.kt", `package com.example.c

class C
`)

	graph := buildGraph(root, map[string]*module.Module{
		":app": {Path: ":app", Dir: filepath.Join(root, "app")},
	})
	pmi := module.BuildPerModuleIndex(graph, []*scanner.File{fileA, fileB, fileC}, 1)

	rule := &PackageDependencyCycleRule{
		BaseRule: BaseRule{RuleName: "PackageDependencyCycle", RuleSetName: "architecture", Sev: "info"},
	}
	rule.SetModuleIndex(pmi)
	findings := rule.CheckModuleAware()

	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}
