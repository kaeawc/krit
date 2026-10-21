package rules

import (
	"bytes"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const diHygieneRuleSet = "di-hygiene"

var (
	anvilContributesToToken      = []byte("ContributesTo")
	anvilContributesBindingToken = []byte("ContributesBinding")
	anvilMergeComponentToken     = []byte("MergeComponent")
)

func anvilMergeComponentMayMatch(content []byte) bool {
	return bytes.Contains(content, anvilContributesToToken) ||
		bytes.Contains(content, anvilContributesBindingToken) ||
		bytes.Contains(content, anvilMergeComponentToken)
}

func anvilScopeDeclarationCandidates(file *scanner.File) []uint32 {
	var matches []uint32
	if file == nil || file.FlatTree == nil {
		return matches
	}
	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatType(idx)
		if nodeType != "class_declaration" && nodeType != "object_declaration" {
			return
		}
		if anvilModifiersMayMatchFlat(file, idx) {
			matches = append(matches, idx)
		}
	})
	return matches
}

func anvilModifiersMayMatchFlat(file *scanner.File, idx uint32) bool {
	if file == nil || idx == 0 {
		return false
	}
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	segment := file.Content[int(file.FlatStartByte(mods)):int(file.FlatEndByte(mods))]
	return bytes.Contains(segment, anvilContributesToToken) ||
		bytes.Contains(segment, anvilContributesBindingToken) ||
		bytes.Contains(segment, anvilMergeComponentToken)
}

// BindsMismatchedArityRule detects @Binds functions that do not declare exactly
// one parameter, which Dagger rejects during code generation.
type BindsMismatchedArityRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. DI hygiene rule. Detection uses annotation and import patterns for
// Dagger/Hilt/Anvil; project-specific DI aliases are not followed.
// Classified per roadmap/17.
func (r *BindsMismatchedArityRule) Confidence() float64 { return api.ConfidenceMedium }

func anvilContributedInterfaceScopesFlat(file *scanner.File) map[string]string {
	scopes := make(map[string]string)
	if file == nil || file.FlatTree == nil {
		return scopes
	}
	file.FlatWalkNodes(0, "class_declaration", func(idx uint32) {
		if _, ok := file.FlatFindChild(idx, "interface"); !ok {
			return
		}
		scope := anvilAnnotationScopeFlat(file, idx, "ContributesTo")
		if scope == "" {
			return
		}
		name := extractIdentifierFlat(file, idx)
		if name == "" {
			return
		}
		scopes[name] = scope
	})
	return scopes
}

func anvilAnnotationScopeFlat(file *scanner.File, idx uint32, annotationName string) string {
	annotationText := findAnnotationTextFlat(file, idx, annotationName)
	if annotationText == "" {
		return ""
	}

	openIdx := strings.Index(annotationText, "(")
	closeIdx := strings.LastIndex(annotationText, ")")
	if openIdx < 0 || closeIdx <= openIdx {
		return ""
	}

	scopeExpr := strings.TrimSpace(annotationText[openIdx+1 : closeIdx])
	if eqIdx := strings.Index(scopeExpr, "="); eqIdx >= 0 {
		scopeExpr = strings.TrimSpace(scopeExpr[eqIdx+1:])
	}
	if commaIdx := strings.Index(scopeExpr, ","); commaIdx >= 0 {
		scopeExpr = scopeExpr[:commaIdx]
	}
	if classIdx := strings.Index(scopeExpr, "::class"); classIdx >= 0 {
		scopeExpr = scopeExpr[:classIdx]
	}
	scopeExpr = strings.TrimSpace(scopeExpr)
	if dotIdx := strings.LastIndex(scopeExpr, "."); dotIdx >= 0 {
		scopeExpr = scopeExpr[dotIdx+1:]
	}
	return scopeExpr
}

func findAnnotationTextFlat(file *scanner.File, idx uint32, annotationName string) string {
	if file == nil || idx == 0 {
		return ""
	}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		if text := findAnnotationTextInFlatParent(file, mods, annotationName); text != "" {
			return text
		}
	}
	return findAnnotationTextInFlatParent(file, idx, annotationName)
}

func findAnnotationTextInFlatParent(file *scanner.File, parent uint32, annotationName string) string {
	target := "@" + annotationName
	for i := 0; i < file.FlatChildCount(parent); i++ {
		child := file.FlatChild(parent, i)
		switch file.FlatType(child) {
		case "annotation", "modifier":
			text := strings.TrimSpace(file.FlatNodeText(child))
			name := strings.TrimPrefix(text, "@")
			if parenIdx := strings.Index(name, "("); parenIdx >= 0 {
				name = name[:parenIdx]
			}
			if colonIdx := strings.Index(name, ":"); colonIdx >= 0 {
				name = name[:colonIdx]
			}
			if name == annotationName || strings.HasPrefix(name, annotationName+".") || strings.Contains(text, target) {
				return text
			}
		}
	}
	return ""
}

// firstMapKeyAnnotationLiteralFlat returns the literal argument of the first
// `@*Key(...)` annotation on idx (e.g. `"foo"` for `@StringKey("foo")`).
// Returns ("", false) if no key annotation or no parenthesized argument.
func firstMapKeyAnnotationLiteralFlat(file *scanner.File, idx uint32) (string, bool) {
	if file == nil || idx == 0 {
		return "", false
	}
	parents := []uint32{idx}
	if mods, ok := file.FlatFindChild(idx, "modifiers"); ok {
		parents = append(parents, mods)
	}
	for _, parent := range parents {
		for i := 0; i < file.FlatChildCount(parent); i++ {
			child := file.FlatChild(parent, i)
			t := file.FlatType(child)
			if t != "annotation" && t != "modifier" {
				continue
			}
			text := strings.TrimSpace(file.FlatNodeText(child))
			name := strings.TrimPrefix(text, "@")
			parenIdx := strings.Index(name, "(")
			if parenIdx < 0 {
				continue
			}
			argText := name[parenIdx+1:]
			closeIdx := strings.LastIndex(argText, ")")
			if closeIdx < 0 {
				continue
			}
			argText = strings.TrimSpace(argText[:closeIdx])
			name = name[:parenIdx]
			if colonIdx := strings.Index(name, ":"); colonIdx >= 0 {
				name = name[:colonIdx]
			}
			if dotIdx := strings.LastIndex(name, "."); dotIdx >= 0 {
				name = name[dotIdx+1:]
			}
			name = strings.TrimSpace(name)
			if name == "" || name == "Key" || name == "MapKey" {
				continue
			}
			if !strings.HasSuffix(name, "Key") {
				continue
			}
			if eq := strings.Index(argText, "="); eq >= 0 {
				argText = strings.TrimSpace(argText[eq+1:])
			}
			if argText == "" {
				continue
			}
			return argText, true
		}
	}
	return "", false
}

// enclosingClassChainFlat returns the dot-joined name of the chain of class /
// object declarations enclosing idx (outermost first). Returns "" if the
// enclosing scope is the file (top-level).
func enclosingClassChainFlat(file *scanner.File, idx uint32) string {
	if file == nil || idx == 0 {
		return ""
	}
	var names []string
	for parent, ok := file.FlatParent(idx); ok; parent, ok = file.FlatParent(parent) {
		switch file.FlatType(parent) {
		case "class_declaration", "object_declaration":
			if name := extractIdentifierFlat(file, parent); name != "" {
				names = append([]string{name}, names...)
			}
		}
	}
	return strings.Join(names, ".")
}

func anvilImplementedTypesFlat(file *scanner.File, idx uint32) []string {
	var names []string
	if file == nil || idx == 0 {
		return names
	}
	file.FlatForEachChild(idx, func(child uint32) {
		if file.FlatType(child) != "delegation_specifier" {
			return
		}
		if name := extractSupertypeNameFlat(file, child); name != "" {
			names = append(names, name)
		}
	})
	return names
}
