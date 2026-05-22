package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// KdocLinkValidationRule detects KDoc bracket links that do not resolve to source symbols.
type KdocLinkValidationRule struct {
	BaseRule
}

// Confidence reflects source-index lookup with conservative external-library skips.
func (r *KdocLinkValidationRule) Confidence() float64 { return api.ConfidenceHigher }

func (r *KdocLinkValidationRule) check(ctx *api.Context) {
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
			declLine := kdocDocumentedDeclarationLine(file, idx)
			if declLine > 0 && file.Suppression != nil &&
				file.Suppression.IsSuppressed(r.RuleName, r.RuleSetName, declLine) {
				return
			}
			iterateKdocLinks(text, func(link kdocLinkToken) {
				if kdocLinkResolves(ctx.CodeIndex, link.Target) {
					return
				}
				line := file.FlatRow(idx) + 1 + strings.Count(text[:link.Offset], "\n")
				ctx.Emit(scanner.Finding{
					File:    file.Path,
					Line:    line,
					Col:     1,
					Message: fmt.Sprintf("KDoc link target %q does not resolve to a symbol in analysed sources.", link.Target),
				})
			})
		})
	}
}

func kdocLinkResolves(index *scanner.CodeIndex, target string) bool {
	if target == "" || isKnownExternalKdocLinkTarget(target) {
		return true
	}
	target = strings.TrimPrefix(target, "#")
	if strings.Contains(target, ".") {
		if sym, ok := index.SymbolByFQN(target); ok && sym.Name != "" {
			return true
		}
		return len(index.SymbolsNamed(target)) > 0
	}
	return len(index.SymbolsNamed(target)) > 0
}

func isKnownExternalKdocLinkTarget(target string) bool {
	target = strings.TrimPrefix(target, "#")
	if strings.HasPrefix(target, "kotlin.") || strings.HasPrefix(target, "java.") || strings.HasPrefix(target, "javax.") {
		return true
	}
	switch target {
	case "Any", "Array", "Boolean", "Byte", "Char", "CharSequence", "Collection",
		"Comparable", "Double", "Enum", "Float", "HashMap", "HashSet", "Int",
		"Iterable", "List", "Long", "Map", "MutableCollection", "MutableList",
		"MutableMap", "MutableSet", "Nothing", "Number", "Pair", "Result",
		"Sequence", "Set", "Short", "String", "Throwable", "Triple", "Unit":
		return true
	}
	return false
}
