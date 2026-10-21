package rules

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// DeprecationRule detects usage of deprecated elements.
// It checks:
//  1. Oracle annotations (kotlin.Deprecated / java.lang.Deprecated) on call targets
//  2. Source-level @Deprecated annotations on function/class declarations
//
// Known limitations:
//   - Cannot detect @Deprecated on transitive library APIs not in the oracle's scope
//   - Cannot detect deprecation level (WARNING/ERROR/HIDDEN) from library metadata
//
// ---------------------------------------------------------------------------
// deprecationInfo stores extracted @Deprecated annotation details.
type deprecationInfo struct {
	message     string // @Deprecated("message")
	replaceWith string // @Deprecated(replaceWith = ReplaceWith("expr"))
	level       string // WARNING, ERROR, or HIDDEN
	// ambiguous marks a name that more than one same-file @Deprecated
	// declaration uses with a different message, ReplaceWith, or level (for
	// example overloads). The index keeps the last one, so it cannot tell
	// which of them a compiler diagnostic refers to.
	ambiguous bool
}

type DeprecationRule struct {
	FlatDispatchBase
	BaseRule
	ExcludeImportStatements bool
}

// Confidence reports a tier-2 (medium) base confidence — matches on
// deprecation markers and annotations via pattern, with resolver-backed
// type checks used only when available. Classified per roadmap/17.
func (r *DeprecationRule) Confidence() float64 { return api.ConfidenceMedium }

// deprecatedDeclIndex returns the per-file index of deprecated
// declarations, computed once per file per run via the shared
// filefacts cache.
func deprecatedDeclIndex(file *scanner.File) map[string]*deprecationInfo {
	if file == nil {
		return nil
	}
	return filefacts.FileFact(fileFactsCache(), file, slotDeprecatedDecls, func() map[string]*deprecationInfo {
		out := make(map[string]*deprecationInfo)
		if file.FlatTree == nil || !bytes.Contains(file.Content, []byte("Deprecated")) {
			return out
		}
		collectDeprecatedDeclsFlat(file, out)
		return out
	})
}

func collectDeprecatedDeclsFlat(file *scanner.File, out map[string]*deprecationInfo) {
	file.FlatWalkAllNodes(0, func(idx uint32) {
		nodeType := file.FlatType(idx)
		if nodeType != "function_declaration" && nodeType != "class_declaration" && nodeType != "property_declaration" {
			return
		}
		if info := extractDeprecatedInfoFlat(file, idx); info != nil {
			name := extractIdentifierFlat(file, idx)
			if name != "" {
				if prev := out[name]; prev != nil && (prev.ambiguous || prev.message != info.message ||
					prev.replaceWith != info.replaceWith || prev.level != info.level) {
					info.ambiguous = true
				}
				out[name] = info
			}
		}
	})
}

func extractDeprecatedInfoFlat(file *scanner.File, idx uint32) *deprecationInfo {
	if file == nil || idx == 0 {
		return nil
	}
	text := deprecationAnnotationText(file, idx)
	if text == "" {
		return nil
	}
	info := &deprecationInfo{}
	info.message = extractAnnotationArg(text, "message")
	if info.message == "" {
		info.message = extractFirstPositionalArg(text)
	}
	info.level = extractDeprecationLevel(text)
	info.replaceWith = extractReplaceWith(text)
	return info
}

// deprecationAnnotationText returns the source text of the @Deprecated
// annotation among idx's modifiers, or "" when there is none. The annotation's
// type name must match exactly. An annotation that merely contains the word,
// such as @OptIn(DeprecatedForRemovalCompilerApi::class) or a project's own
// @DeprecatedApi marker, is not a deprecation. The message, level, and
// ReplaceWith arguments are then read from this annotation alone, never from a
// neighbor.
func deprecationAnnotationText(file *scanner.File, idx uint32) string {
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return ""
	}
	for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "annotation" {
			continue
		}
		text := file.FlatNodeText(child)
		switch deprecationAnnotationTypeName(text) {
		case "Deprecated", "kotlin.Deprecated", "java.lang.Deprecated":
			return text
		}
	}
	return ""
}

// deprecationAnnotationTypeName reduces annotation text to its type name:
// "@field:kotlin.Deprecated(\"x\")" becomes "kotlin.Deprecated". A use-site
// target is dropped only when its colon precedes the argument list, so a colon
// inside a message string is left alone.
func deprecationAnnotationTypeName(text string) string {
	text = strings.TrimPrefix(strings.TrimSpace(text), "@")
	paren := strings.Index(text, "(")
	if colon := strings.Index(text, ":"); colon >= 0 && (paren < 0 || colon < paren) {
		text = text[colon+1:]
		paren = strings.Index(text, "(")
	}
	if paren >= 0 {
		text = text[:paren]
	}
	return strings.Join(strings.Fields(text), "")
}

func flatDeprecationRefName(file *scanner.File, idx uint32) string {
	if file == nil {
		return ""
	}
	switch file.FlatType(idx) {
	case "call_expression":
		return flatCallExpressionName(file, idx)
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifier(file, idx)
	case "user_type":
		for i := 0; i < file.FlatNamedChildCount(idx); i++ {
			child := file.FlatNamedChild(idx, i)
			if file.FlatType(child) == "type_identifier" {
				return file.FlatNodeText(child)
			}
		}
		text := strings.TrimSpace(file.FlatNodeText(idx))
		if idx := strings.LastIndex(text, "."); idx >= 0 {
			return text[idx+1:]
		}
		return text
	default:
		return ""
	}
}

// flatDeprecationRefNameNode returns the identifier node that
// flatDeprecationRefName reads the referenced name from, or 0. The compiler
// anchors DEPRECATION on exactly that token, so matching it lets one dispatched
// node claim each diagnostic even though call_expression, navigation_expression,
// and user_type can all cover the same span: in `a.dep.m()` the call's name
// node is `m`, and the inner `a.dep` navigation owns the diagnostic on `dep`.
func flatDeprecationRefNameNode(file *scanner.File, idx uint32) uint32 {
	name := flatDeprecationRefName(file, idx)
	if name == "" {
		return 0
	}
	switch file.FlatType(idx) {
	case "call_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "simple_identifier":
				if file.FlatNodeTextEquals(child, name) {
					return child
				}
				return 0
			case "navigation_expression":
				return flatNavigationExpressionLastIdentifierNamed(file, child, name)
			}
		}
	case "navigation_expression":
		return flatNavigationExpressionLastIdentifierNamed(file, idx, name)
	case "user_type":
		// The last type_identifier outside any type_arguments: `a.b.Old<X>`
		// names Old, not X.
		var last uint32
		var walk func(uint32)
		walk = func(n uint32) {
			for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
				switch file.FlatType(child) {
				case "type_arguments":
					continue
				case "type_identifier":
					if file.FlatNodeTextEquals(child, name) {
						last = child
					}
				default:
					walk(child)
				}
			}
		}
		walk(idx)
		return last
	}
	return 0
}

// deprecationMessage formats a finding message from same-file @Deprecated
// details, shared by the compiler-projection and source-level paths.
func deprecationMessage(name string, info *deprecationInfo) string {
	switch {
	case info.message != "" && info.level != "":
		return fmt.Sprintf("'%s' is deprecated (level=%s): %s", name, info.level, info.message)
	case info.message != "":
		return fmt.Sprintf("'%s' is deprecated: %s", name, info.message)
	case info.level != "":
		return fmt.Sprintf("'%s' is deprecated (level=%s).", name, info.level)
	}
	return fmt.Sprintf("'%s' is deprecated.", name)
}

// deprecationReplaceWithFix offers the same-file ReplaceWith expression as an
// auto-fix for a call or navigation reference, or nil when none applies.
func deprecationReplaceWithFix(file *scanner.File, idx uint32, info *deprecationInfo) *scanner.Fix {
	if info == nil || info.replaceWith == "" {
		return nil
	}
	if t := file.FlatType(idx); t != "call_expression" && t != "navigation_expression" {
		return nil
	}
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(idx)),
		EndByte:     int(file.FlatEndByte(idx)),
		Replacement: info.replaceWith,
	}
}

// compilerDeprecationDetail extracts the @Deprecated message from K2's
// DEPRECATION text, "'<symbol>' is deprecated. <message>.", returning "" when
// there is none.
func compilerDeprecationDetail(message string) string {
	const marker = "' is deprecated."
	i := strings.Index(message, marker)
	if i < 0 {
		return ""
	}
	return strings.TrimSuffix(strings.TrimSpace(message[i+len(marker):]), ".")
}

// sameFileDeprecationFor returns the same-file @Deprecated entry named name only
// when it is the declaration the compiler flagged: the index is keyed by name
// alone, so another class's member, another overload, or a library symbol with
// the same name must not lend this diagnostic its message or ReplaceWith fix.
// The compiler's rendered message carries the @Deprecated text, which must match,
// and a name shared by differing same-file declarations is never trusted.
func sameFileDeprecationFor(file *scanner.File, name string, d oracle.Diagnostic) *deprecationInfo {
	info := deprecatedDeclIndex(file)[name]
	if info == nil || info.ambiguous || compilerDeprecationDetail(d.Message) != strings.TrimSuffix(info.message, ".") {
		return nil
	}
	return info
}

// deprecationProjection projects K2's DEPRECATION verdict for the reference
// named name. The compiler resolves overloads, inherited and Java-annotated
// members, typealiases, and property and type references, which the rule's
// call-target and same-file paths miss or match only by name.
func deprecationProjection(name string) DiagnosticProjection {
	return DiagnosticProjection{
		RuleID:       "Deprecation",
		FactoryNames: []string{"DEPRECATION"},
		Accept: func(ctx *api.Context, d oracle.Diagnostic) bool {
			nameNode := flatDeprecationRefNameNode(ctx.File, ctx.Idx)
			if nameNode == 0 {
				return false
			}
			start, _, ok := diagnosticAnchorBytes(ctx.File, d)
			return ok && start == ctx.File.FlatStartByte(nameNode)
		},
		Message: func(ctx *api.Context, d oracle.Diagnostic) string {
			if info := sameFileDeprecationFor(ctx.File, name, d); info != nil {
				return deprecationMessage(name, info)
			}
			if detail := compilerDeprecationDetail(d.Message); detail != "" {
				return fmt.Sprintf("'%s' is deprecated: %s", name, detail)
			}
			return fmt.Sprintf("'%s' is deprecated.", name)
		},
		Fix: func(ctx *api.Context, d oracle.Diagnostic) *scanner.Fix {
			return deprecationReplaceWithFix(ctx.File, ctx.Idx, sameFileDeprecationFor(ctx.File, name, d))
		},
		Confidence: api.ConfidenceVeryHigh,
	}
}

// extractAnnotationArg extracts a named argument from annotation text.
// e.g., extractAnnotationArg(`@Deprecated(message = "old")`, "message") → "old"
func extractAnnotationArg(annText, argName string) string {
	pattern := argName + "="
	// Also try with spaces: argName + " ="
	idx := strings.Index(annText, pattern)
	if idx < 0 {
		pattern = argName + " ="
		idx = strings.Index(annText, pattern)
	}
	if idx < 0 {
		return ""
	}
	rest := annText[idx+len(pattern):]
	rest = strings.TrimSpace(rest)
	if len(rest) == 0 {
		return ""
	}
	if rest[0] == '"' {
		// Find closing quote
		end := strings.Index(rest[1:], "\"")
		if end >= 0 {
			return rest[1 : end+1]
		}
	}
	return ""
}

// extractFirstPositionalArg extracts the first positional string arg.
// e.g., `@Deprecated("Use newMethod instead")` → "Use newMethod instead"
func extractFirstPositionalArg(annText string) string {
	parenIdx := strings.Index(annText, "(")
	if parenIdx < 0 {
		return ""
	}
	rest := strings.TrimSpace(annText[parenIdx+1:])
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	end := strings.Index(rest[1:], "\"")
	if end >= 0 {
		return rest[1 : end+1]
	}
	return ""
}

// extractDeprecationLevel extracts the level argument from @Deprecated.
// e.g., `@Deprecated(..., level = DeprecationLevel.ERROR)` → "ERROR"
func extractDeprecationLevel(annText string) string {
	idx := strings.Index(annText, "level")
	if idx < 0 {
		return ""
	}
	rest := annText[idx:]
	eqIdx := strings.Index(rest, "=")
	if eqIdx < 0 {
		return ""
	}
	rest = strings.TrimSpace(rest[eqIdx+1:])
	// Confine the scan to the level value itself; otherwise sibling
	// arguments like `replaceWith = ReplaceWith("... WARNING ...")` or
	// the deprecation message could shadow the actual level.
	if end := levelArgumentEnd(rest); end >= 0 {
		rest = rest[:end]
	}
	// Could be DeprecationLevel.WARNING or just WARNING
	for _, level := range []string{"WARNING", "ERROR", "HIDDEN"} {
		if strings.Contains(rest, level) {
			return level
		}
	}
	return ""
}

// levelArgumentEnd returns the index in s at which the level value ends —
// the first top-level `,` or `)`. The legitimate level values
// (`DeprecationLevel.WARNING|ERROR|HIDDEN`, or the unqualified variants)
// contain no parens, brackets, or string literals, so a depth/quote walk
// is enough.
func levelArgumentEnd(s string) int {
	depth := 0
	inStr := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			if c == '\\' && i+1 < len(s) {
				i++
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '(', '[', '{':
			depth++
		case ')', ']', '}':
			if depth == 0 {
				return i
			}
			depth--
		case ',':
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// extractReplaceWith extracts the replaceWith expression from @Deprecated.
// e.g., `@Deprecated(..., replaceWith = ReplaceWith("newMethod()"))` → "newMethod()"
func extractReplaceWith(annText string) string {
	idx := strings.Index(annText, "ReplaceWith")
	if idx < 0 {
		return ""
	}
	rest := annText[idx:]
	parenIdx := strings.Index(rest, "(")
	if parenIdx < 0 {
		return ""
	}
	rest = rest[parenIdx+1:]
	rest = strings.TrimSpace(rest)
	if len(rest) == 0 || rest[0] != '"' {
		return ""
	}
	end := strings.Index(rest[1:], "\"")
	if end >= 0 {
		return rest[1 : end+1]
	}
	return ""
}
