package rules

import (
	"fmt"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// SampleAnnotationFreshnessRule detects KDoc @sample tags whose target function is missing.
type SampleAnnotationFreshnessRule struct {
	BaseRule
}

// Confidence reflects exact FQN lookup through the cross-file symbol index.
func (r *SampleAnnotationFreshnessRule) Confidence() float64 { return api.ConfidenceHigher }

var kdocSampleTagRe = regexp.MustCompile(`@sample\s+([A-Za-z_][A-Za-z0-9_]*(?:\.[A-Za-z_][A-Za-z0-9_]*)+)`)

func (r *SampleAnnotationFreshnessRule) check(ctx *api.Context) {
	if ctx.CodeIndex == nil || len(ctx.ParsedFiles) == 0 {
		return
	}
	for _, file := range ctx.ParsedFiles {
		if file == nil || file.Language != scanner.LangKotlin {
			continue
		}
		file.FlatWalkNodes(0, "multiline_comment", func(idx uint32) {
			if !flatIsKDoc(file, idx) {
				return
			}
			text := file.FlatNodeText(idx)
			matches := kdocSampleTagRe.FindAllStringSubmatchIndex(text, -1)
			if len(matches) == 0 {
				return
			}
			declLine := kdocDocumentedDeclarationLine(file, idx)
			if declLine > 0 && file.Suppression != nil &&
				file.Suppression.IsSuppressed(r.RuleName, r.RuleSetName, declLine) {
				return
			}
			for _, match := range matches {
				if len(match) < 4 || match[2] < 0 || match[3] < 0 {
					continue
				}
				fqn := text[match[2]:match[3]]
				if sampleAnnotationFunctionExists(ctx.CodeIndex, fqn) {
					continue
				}
				line := file.FlatRow(idx) + 1 + strings.Count(text[:match[0]], "\n")
				ctx.Emit(scanner.Finding{
					File:    file.Path,
					Line:    line,
					Col:     1,
					Message: fmt.Sprintf("KDoc @sample target %q does not resolve to a function in analysed sources.", fqn),
				})
			}
		})
	}
}

func sampleAnnotationFunctionExists(index *scanner.CodeIndex, fqn string) bool {
	sym, ok := index.SymbolByFQN(fqn)
	return ok && sym.Kind == "function"
}

func kdocDocumentedDeclarationLine(file *scanner.File, kdoc uint32) int {
	if file == nil || kdoc == 0 {
		return 0
	}
	end := file.FlatEndByte(kdoc)
	var best uint32
	for _, typ := range []string{"function_declaration", "class_declaration", "property_declaration", "object_declaration"} {
		file.FlatWalkNodes(0, typ, func(idx uint32) {
			if file.FlatStartByte(idx) < end {
				return
			}
			if best == 0 || file.FlatStartByte(idx) < file.FlatStartByte(best) {
				best = idx
			}
		})
	}
	if best != 0 {
		return file.FlatRow(best) + 1
	}
	return 0
}
