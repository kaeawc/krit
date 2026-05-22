package rules

// Testing-quality rule: UntestedPublicAPI. Flags top-level Kotlin
// public declarations (functions, classes) that have no references
// from test sources.
//
// Extracted from testing_quality.go as part of the god-file split.
// testingQualityFilesByPath is shared in name only — at extraction
// time it had a single consumer (this rule) and lives here.

import (
	"strings"

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
		ctx.Emit(scanner.Finding{
			File:       sym.File,
			Line:       sym.Line,
			Col:        1,
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

func untestedPublicAPISuppressed(sym scanner.Symbol, file *scanner.File) bool {
	if file == nil {
		return false
	}
	text := untestedPublicAPIDeclarationContext(sym, file)
	return strings.Contains(text, "@VisibleForTesting") ||
		strings.Contains(text, "@Generated") ||
		strings.Contains(text, `@Suppress("UntestedPublicApi")`) ||
		strings.Contains(text, `@Suppress("all")`)
}

func untestedPublicAPIDeclarationContext(sym scanner.Symbol, file *scanner.File) string {
	var b strings.Builder
	if sym.Line > 0 && len(file.Lines) > 0 {
		start := sym.Line - 4
		if start < 0 {
			start = 0
		}
		end := sym.Line + 1
		if end > len(file.Lines) {
			end = len(file.Lines)
		}
		for _, line := range file.Lines[start:end] {
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	if sym.StartByte >= 0 && sym.EndByte > sym.StartByte && sym.EndByte <= len(file.Content) {
		b.Write(file.Content[sym.StartByte:sym.EndByte])
	}
	return b.String()
}
