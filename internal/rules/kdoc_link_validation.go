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
			scope := kdocScopeNames(file, kdocDocumentedDeclaration(file, idx))
			imports := fileFactsCache().Imports(file).Aliases
			iterateKdocLinks(text, func(link kdocLinkToken) {
				if kdocLinkResolves(ctx.CodeIndex, link.Target, scope, imports) {
					return
				}
				line, col := commentOffsetPosition(file, idx, link.Offset)
				ctx.Emit(scanner.Finding{
					File:    file.Path,
					Line:    line,
					Col:     col,
					Message: fmt.Sprintf("KDoc link target %q does not resolve to a symbol in analysed sources.", link.Target),
				})
			})
		})
	}
}

// kdocExternalRoots are package roots the source index does not cover, so a qualified link under them is not checked.
var kdocExternalRoots = []string{"android.", "androidx.", "kotlinx.", "com.", "org.", "io.", "dagger.", "javax.", "okhttp3.", "retrofit2."}

func kdocLinkResolves(index *scanner.CodeIndex, target string, scope map[string]bool, imports map[string]string) bool {
	target = strings.TrimPrefix(target, "#")
	if target == "" || isKnownExternalKdocLinkTarget(target) {
		return true
	}
	segments := strings.Split(target, ".")
	first, last := segments[0], segments[len(segments)-1]
	// Parameters, type parameters and constructor properties of the documented declaration.
	if scope[first] {
		return true
	}
	// An imported name can come from a library, and its members cannot be checked.
	if _, ok := imports[first]; ok {
		return true
	}
	if len(segments) == 1 {
		return len(index.SymbolsNamed(target)) > 0
	}
	if sym, ok := index.SymbolByFQN(target); ok && sym.Name != "" {
		return true
	}
	for _, root := range kdocExternalRoots {
		if strings.HasPrefix(target, root) {
			return true
		}
	}
	// `Class.member` resolves when both ends name a known symbol.
	if first != "" && first[0] >= 'a' && first[0] <= 'z' {
		return len(index.SymbolsNamed(last)) > 0
	}
	return len(index.SymbolsNamed(first)) > 0 && len(index.SymbolsNamed(last)) > 0
}

// kdocScopeNames returns the names a KDoc link may use without a declaration in the index: parameters, type
// parameters and primary-constructor properties of the documented declaration and its enclosing classes.
func kdocScopeNames(file *scanner.File, decl uint32) map[string]bool {
	names := map[string]bool{}
	if decl == 0 {
		return names
	}
	collect := func(node uint32) {
		file.FlatWalkAllNodes(node, func(n uint32) {
			switch file.FlatType(n) {
			case "parameter", "class_parameter":
				if name := extractIdentifierFlat(file, n); name != "" {
					names[name] = true
				}
			case "type_parameter":
				if ident, ok := file.FlatFindChild(n, "type_identifier"); ok {
					names[file.FlatNodeText(ident)] = true
				}
			}
		})
	}
	// Only the signature counts, not the body.
	for cur, ok := decl, true; ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "function_declaration", "class_declaration":
			for _, part := range []string{"type_parameters", "function_value_parameters", "primary_constructor"} {
				if n, found := file.FlatFindChild(cur, part); found {
					collect(n)
				}
			}
		}
	}
	return names
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
