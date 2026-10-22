package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

func mainDispatcherReferenceFlat(ctx *api.Context, idx uint32) bool {
	file := ctx.File
	if file.FlatType(idx) != "navigation_expression" {
		return false
	}
	segments := flatNavigationChainIdentifiers(file, idx)
	if len(segments) != 2 || segments[0] != "Dispatchers" || segments[1] != "Main" {
		return false
	}
	if ctx.Resolver != nil {
		if typ, ok := semantics.ExpressionType(ctx, idx); ok && semanticTypeNameMatches(typ.FQN, "kotlinx.coroutines.MainCoroutineDispatcher", "MainCoroutineDispatcher") {
			return true
		}
		if fqn := ctx.Resolver.ResolveImport("Dispatchers", file); fqn != "" {
			return fqn == "kotlinx.coroutines.Dispatchers"
		}
		return false
	}
	return false
}

func receiverDeclaredFromCall(file *scanner.File, call uint32, initializerCallee string) bool {
	receiver := semanticsReceiverNode(&api.Context{File: file}, call)
	name := semantics.ReferenceName(file, receiver)
	if name == "" {
		return false
	}
	fn, ok := flatEnclosingFunction(file, call)
	if !ok {
		return false
	}
	found := false
	file.FlatWalkAllNodes(fn, func(n uint32) {
		if found || file.FlatStartByte(n) >= file.FlatStartByte(call) {
			return
		}
		if file.FlatType(n) != "property_declaration" && file.FlatType(n) != "variable_declaration" {
			return
		}
		if extractIdentifierFlat(file, n) == name && propertyInitializerCallCalleeName(file, n) == initializerCallee {
			found = true
		}
	})
	return found
}

func navigationReceiverHasTypedRoot(file *scanner.File, nav uint32, typeName string) bool {
	segments := flatNavigationChainIdentifiers(file, nav)
	if len(segments) == 0 {
		return false
	}
	root := segments[0]
	fn, ok := flatEnclosingFunction(file, nav)
	if !ok {
		return false
	}
	return parameterHasTypeFlat(file, fn, root, typeName)
}

func parameterHasTypeFlat(file *scanner.File, owner uint32, name string, typeName string) bool {
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(owner, "parameter", func(param uint32) {
		if found || extractIdentifierFlat(file, param) != name {
			return
		}
		file.FlatWalkNodes(param, "user_type", func(typ uint32) {
			if found {
				return
			}
			if semanticTypeNameMatches(apiNodeNameFlat(file, typ), typeName) {
				found = true
			}
		})
	})
	return found
}

func navigationChainContainsSegment(file *scanner.File, nav uint32, name string) bool {
	return navigationSegmentsContain(flatNavigationChainIdentifiers(file, nav), name)
}

func navigationSegmentsContain(segments []string, name string) bool {
	for _, segment := range segments {
		if segment == name {
			return true
		}
	}
	return false
}

func receiverChainHasQualifiedRoot(file *scanner.File, receiver uint32, roots ...string) bool {
	segments := flatNavigationChainIdentifiers(file, receiver)
	if len(segments) == 0 {
		if file.FlatType(receiver) == "call_expression" {
			nav, _ := flatCallExpressionParts(file, receiver)
			segments = flatNavigationChainIdentifiers(file, nav)
		}
	}
	if len(segments) > 0 {
		for _, root := range roots {
			if segments[0] == root {
				return true
			}
		}
	}
	found := false
	file.FlatWalkAllNodes(receiver, func(n uint32) {
		if found {
			return
		}
		if file.FlatType(n) != "navigation_expression" && file.FlatType(n) != "simple_identifier" {
			return
		}
		name := semantics.ReferenceName(file, n)
		for _, root := range roots {
			if name == root {
				found = true
				return
			}
		}
	})
	return found
}

func sameFileFunctionDeclaredInOwner(file *scanner.File, call uint32, owners ...string) bool {
	name := flatCallExpressionName(file, call)
	if name == "" {
		return false
	}
	found := false
	file.FlatWalkNodes(0, "function_declaration", func(fn uint32) {
		if found || extractIdentifierFlat(file, fn) != name {
			return
		}
		for p, ok := file.FlatParent(fn); ok; p, ok = file.FlatParent(p) {
			if file.FlatType(p) != "class_declaration" && file.FlatType(p) != "object_declaration" {
				continue
			}
			if semanticTypeNameMatches(extractIdentifierFlat(file, p), owners...) {
				found = true
			}
			return
		}
	})
	return found
}

func importShortNameFlat(file *scanner.File, idx uint32) string {
	text := string(file.FlatNodeBytes(idx))
	if lineEnd := strings.IndexByte(text, '\n'); lineEnd >= 0 {
		text = text[:lineEnd]
	}
	text = strings.TrimSpace(text)
	if !strings.HasPrefix(text, "import ") {
		return ""
	}
	imp := strings.TrimSpace(strings.TrimPrefix(text, "import "))
	aliasAt, lastDot := scanImportTail(imp)
	if aliasAt >= 0 {
		return strings.Trim(strings.TrimSpace(imp[aliasAt+4:]), "`")
	}
	last := imp[lastDot+1:]
	if last == "" || last == "*" {
		return ""
	}
	return strings.Trim(last, "`")
}

// scanImportTail walks the import path once, returning the byte offset of an
// " as " alias keyword (or -1) and the byte offset of the last '.' segment
// separator (or -1). Backtick-quoted spans are opaque, so a space or '.'
// inside `foo bar` or `a.b` does not split the path.
func scanImportTail(imp string) (aliasAt, lastDot int) {
	aliasAt, lastDot = -1, -1
	inBacktick := false
	for i := 0; i < len(imp); i++ {
		switch imp[i] {
		case '`':
			inBacktick = !inBacktick
		case ' ':
			if !inBacktick && aliasAt < 0 && strings.HasPrefix(imp[i:], " as ") {
				aliasAt = i
			}
		case '.':
			if !inBacktick {
				lastDot = i
			}
		}
	}
	return aliasAt, lastDot
}

var kotlinImplicitOperatorImportNames = map[string]struct{}{
	"unaryPlus":       {},
	"unaryMinus":      {},
	"not":             {},
	"inc":             {},
	"dec":             {},
	"plus":            {},
	"minus":           {},
	"times":           {},
	"div":             {},
	"mod":             {},
	"rangeTo":         {},
	"rangeUntil":      {},
	"contains":        {},
	"get":             {},
	"set":             {},
	"invoke":          {},
	"plusAssign":      {},
	"minusAssign":     {},
	"timesAssign":     {},
	"divAssign":       {},
	"modAssign":       {},
	"equals":          {},
	"compareTo":       {},
	"iterator":        {},
	"getValue":        {},
	"setValue":        {},
	"provideDelegate": {},
}

func unusedImportImplicitlyUsedByKotlin(shortName string) bool {
	if _, ok := kotlinImplicitOperatorImportNames[shortName]; ok {
		return true
	}
	if !strings.HasPrefix(shortName, "component") || len(shortName) == len("component") {
		return false
	}
	for _, r := range shortName[len("component"):] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func fileHasReferenceNameOutsideNode(file *scanner.File, name string, exclude uint32) bool {
	if file == nil || name == "" {
		return false
	}
	summary := fileReferenceSummaryForUnusedChecks(file)
	excludeStart := file.FlatStartByte(exclude)
	excludeEnd := file.FlatEndByte(exclude)
	for _, ref := range summary.ByName[name] {
		if ref.Start >= excludeStart && ref.End <= excludeEnd {
			continue
		}
		return true
	}
	return contentHasIdentifierOutsideRange(file.Content, name, excludeStart, excludeEnd)
}

func contentHasIdentifierOutsideRange(content []byte, name string, excludeStart, excludeEnd uint32) bool {
	if len(content) == 0 || name == "" {
		return false
	}
	start, end := int(excludeStart), int(excludeEnd)
	if start < 0 || end < start || end > len(content) {
		return false
	}
	return codeTextHasIdentifier(string(content[:start]), name) ||
		codeTextHasIdentifier(string(content[end:]), name)
}

func codeTextHasIdentifier(text, name string) bool {
	for i := 0; i < len(text); {
		if strings.HasPrefix(text[i:], "//") {
			if nl := strings.IndexByte(text[i:], '\n'); nl >= 0 {
				i += nl + 1
			} else {
				return false
			}
			continue
		}
		if strings.HasPrefix(text[i:], "/*") {
			if closeIdx := strings.Index(text[i+2:], "*/"); closeIdx >= 0 {
				i += closeIdx + 4
			} else {
				return false
			}
			continue
		}
		if strings.HasPrefix(text[i:], `"""`) {
			if closeIdx := strings.Index(text[i+3:], `"""`); closeIdx >= 0 {
				i += closeIdx + 6
			} else {
				return false
			}
			continue
		}
		if text[i] == '"' {
			i++
			for i < len(text) {
				if text[i] == '\\' {
					i += 2
					continue
				}
				if text[i] == '"' {
					i++
					break
				}
				i++
			}
			continue
		}
		if text[i] == '\'' {
			i++
			for i < len(text) {
				if text[i] == '\\' {
					i += 2
					continue
				}
				if text[i] == '\'' {
					i++
					break
				}
				i++
			}
			continue
		}
		if hasIdentifierAt(text, name, i) {
			return true
		}
		i++
	}
	return false
}

func hasIdentifierAt(text, name string, i int) bool {
	if i < 0 || i+len(name) > len(text) || text[i:i+len(name)] != name {
		return false
	}
	if i > 0 && isIdentifierByte(text[i-1]) {
		return false
	}
	if i+len(name) < len(text) && isIdentifierByte(text[i+len(name)]) {
		return false
	}
	return true
}

func isIdentifierByte(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') ||
		(ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') ||
		ch == '_'
}

// fileReferenceSummaryForUnusedChecks returns a per-file index of
// identifier references for unused-symbol analysis. Backed by the
// shared filefacts cache, which excludes references inside import_header
// and package_header nodes.
func fileReferenceSummaryForUnusedChecks(file *scanner.File) *filefacts.ReferenceFacts {
	return fileFactsCache().References(file, func(file *scanner.File, idx uint32) string {
		return unusedCheckReferenceName(file, idx)
	})
}

func unusedCheckReferenceName(file *scanner.File, idx uint32) string {
	switch file.FlatType(idx) {
	case "call_expression":
		return flatCallExpressionName(file, idx)
	case "simple_identifier", "type_identifier", "navigation_expression", "user_type":
		return semantics.ReferenceName(file, idx)
	case "interpolated_identifier", "line_str_ref", "multi_line_str_ref":
		text := strings.TrimSpace(file.FlatNodeText(idx))
		text = strings.TrimPrefix(text, "$")
		text = strings.TrimPrefix(text, "{")
		text = strings.TrimSuffix(text, "}")
		return strings.Trim(text, "`")
	default:
		return ""
	}
}

func simpleCallExpressionNameFromText(text string) string {
	paren := strings.Index(text, "(")
	if paren < 0 {
		return ""
	}
	prefix := strings.TrimSpace(text[:paren])
	prefix = strings.TrimLeft(prefix, "!. \t\r\n")
	if dot := strings.LastIndex(prefix, "."); dot >= 0 {
		prefix = prefix[dot+1:]
	}
	prefix = strings.Trim(prefix, "`")
	if prefix == "" {
		return ""
	}
	for _, r := range prefix {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			r == '_' {
			continue
		}
		return ""
	}
	return prefix
}
