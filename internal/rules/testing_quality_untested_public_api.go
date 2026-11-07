package rules

// Testing-quality rule: UntestedPublicAPI. Flags top-level Kotlin
// public declarations (functions, classes) that have no references
// from test sources.
//
// Extracted from testing_quality.go as part of the god-file split.
// testingQualityFilesByPath is shared in name only — at extraction
// time it had a single consumer (this rule) and lives here.

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type UntestedPublicAPIRule struct {
	BaseRule
}

func (r *UntestedPublicAPIRule) Confidence() float64 { return 0.55 }

func (r *UntestedPublicAPIRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil {
		return
	}
	index := ctx.CodeIndex
	filesByPath := testingQualityFilesByPath(ctx.ParsedFiles)
	if len(filesByPath) == 0 {
		filesByPath = testingQualityFilesByPath(index.Files)
	}
	for _, sym := range index.Symbols {
		if !untestedPublicAPICandidate(sym) {
			continue
		}
		file := filesByPath[sym.File]
		if untestedPublicAPISuppressed(sym, file) {
			continue
		}
		if untestedPublicAPIHasTestReference(index, sym) {
			continue
		}
		kind := sym.Kind
		if kind == "function" {
			kind = "top-level function"
		}
		line, col := symbolAnchor(file, sym)
		ctx.Emit(scanner.Finding{
			File:       sym.File,
			Line:       line,
			Col:        col,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    "Public " + kind + " '" + sym.Name + "' has no references from test sources.",
			Confidence: r.Confidence(),
		})
	}
}

func testingQualityFilesByPath(files []*scanner.File) map[string]*scanner.File {
	out := make(map[string]*scanner.File, len(files))
	for _, file := range files {
		if file != nil {
			out[file.Path] = file
		}
	}
	return out
}

func untestedPublicAPICandidate(sym scanner.Symbol) bool {
	if sym.Language != scanner.LangKotlin {
		return false
	}
	if sym.Visibility != "public" {
		return false
	}
	if sym.Kind != "class" && sym.Kind != "function" {
		return false
	}
	if sym.Kind == "function" && sym.Owner != "" {
		return false
	}
	if sym.Owner != "" && sym.Kind == "class" {
		return false
	}
	if sym.IsOverride || sym.IsTest || sym.IsMain {
		return false
	}
	return !scanner.IsTestFile(sym.File)
}

func untestedPublicAPIHasTestReference(index *scanner.CodeIndex, sym scanner.Symbol) bool {
	for _, name := range untestedPublicAPIReferenceNames(sym) {
		for path := range index.ReferenceFiles(name) {
			if path != sym.File && scanner.IsTestFile(path) {
				return true
			}
		}
	}
	return false
}

func untestedPublicAPIReferenceNames(sym scanner.Symbol) []string {
	if sym.FQN == "" || sym.FQN == sym.Name {
		return []string{sym.Name}
	}
	return []string{sym.Name, sym.FQN}
}

// untestedPublicAPISuppressed reports a declaration annotated as test-visible or generated. Only the annotations on
// the declaration itself count; an annotation on one of its members says nothing about the declaration.
func untestedPublicAPISuppressed(sym scanner.Symbol, file *scanner.File) bool {
	if file == nil {
		return false
	}
	if decl, ok := untestedPublicAPITopLevelDeclaration(file, sym.StartByte); ok {
		for _, name := range []string{"VisibleForTesting", "Generated"} {
			if flatHasAnnotationNamed(file, decl, name) {
				return true
			}
		}
		return false
	}
	return false
}

// untestedPublicAPITopLevelDeclaration returns the top-level declaration node that contains the byte offset.
func untestedPublicAPITopLevelDeclaration(file *scanner.File, offset int) (uint32, bool) {
	if file.FlatTree == nil {
		return 0, false
	}
	for child := file.FlatFirstChild(0); child != 0; child = file.FlatNextSib(child) {
		if int(file.FlatStartByte(child)) <= offset && offset < int(file.FlatEndByte(child)) {
			switch file.FlatType(child) {
			case "class_declaration", "function_declaration", "object_declaration":
				return child, true
			}
		}
	}
	return 0, false
}
