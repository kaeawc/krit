package rules

import (
	"strings"
	"unicode"

	"github.com/kaeawc/krit/internal/scanner"
)

// numberFormatMaxAliasDepth bounds how many local variable hops a formatted value is followed through.
const numberFormatMaxAliasDepth = 3

// numberFormatPassThroughMembers keep the formatted text intact, so the value keeps flowing.
var numberFormatPassThroughMembers = map[string]bool{
	"format": true, "trim": true, "replace": true, "replaceFirst": true, "toString": true,
	"lowercase": true, "uppercase": true, "padStart": true, "padEnd": true, "substring": true,
	"removePrefix": true, "removeSuffix": true, "plus": true,
}

// numberFormatParseMembers read the formatted text back as a value, so a locale mismatch breaks them.
var numberFormatParseMembers = map[string]bool{
	"toDouble": true, "toDoubleOrNull": true, "toInt": true, "toIntOrNull": true,
	"toLong": true, "toLongOrNull": true, "toFloat": true, "toFloatOrNull": true,
	"toBigDecimal": true, "toBigDecimalOrNull": true, "toUri": true, "toUrl": true,
	"toHttpUrl": true, "toHttpUrlOrNull": true, "toFile": true,
}

// numberFormatSinkCallees take a string that is parsed, sent, stored, or used as a key, URL, or file name.
var numberFormatSinkCallees = map[string]bool{
	"parse": true, "parseDouble": true, "parseInt": true, "parseLong": true, "parseFloat": true,
	"BigDecimal": true, "BigInteger": true, "URL": true, "URI": true, "File": true,
	"appendQueryParameter": true, "addQueryParameter": true, "addPathSegment": true,
	"encode": true, "addProperty": true, "writeText": true, "writeUtf8": true,
}

// numberFormatSinkNameWords mark a variable or function that holds a key, URL, path, or payload.
var numberFormatSinkNameWords = map[string]bool{
	"key": true, "keys": true, "url": true, "uri": true, "path": true, "json": true,
	"payload": true, "query": true, "querystring": true, "filename": true,
	"serialize": true, "serialized": true, "serialization": true, "cachekey": true,
}

// numberFormatResultFlowsToSink reports whether text produced by the formatter call reaches a
// serialization, parse, cache-key, URL, or file-name sink in the same function. Display formatting
// is locale-correct by design, so any other use stays silent.
func numberFormatResultFlowsToSink(file *scanner.File, formatter uint32) bool {
	scope, ok := flatEnclosingAncestor(file, formatter, "function_declaration")
	if !ok {
		scope, ok = flatEnclosingAncestor(file, formatter, "property_declaration")
	}
	if !ok {
		return false
	}
	return numberFormatValueFlowsToSink(file, scope, formatter, 0, map[string]bool{})
}

func numberFormatValueFlowsToSink(file *scanner.File, scope, value uint32, depth int, seenVars map[string]bool) bool {
	cur := value
	for {
		parent, ok := file.FlatParent(cur)
		if !ok {
			return false
		}
		switch file.FlatType(parent) {
		case "parenthesized_expression", "additive_expression", "interpolated_expression", "string_literal",
			"elvis_expression", "check_expression":
			cur = parent
		case "navigation_expression":
			// Only the receiver position carries the value; the suffix is a member name.
			if file.FlatNamedChild(parent, 0) != cur {
				return false
			}
			member := flatNavigationExpressionLastIdentifier(file, parent)
			if numberFormatParseMembers[member] {
				return true
			}
			if !numberFormatPassThroughMembers[member] {
				return false
			}
			cur = parent
			if call, ok := file.FlatParent(parent); ok && file.FlatType(call) == "call_expression" {
				cur = call
			}
		case "value_argument":
			call, ok := flatEnclosingAncestor(file, parent, "call_expression")
			return ok && numberFormatCallIsSink(file, call)
		case "property_declaration":
			return numberFormatVariableCarriesToSink(file, scope, nameOfProperty(file, parent), depth, seenVars)
		case "assignment":
			return numberFormatVariableCarriesToSink(file, scope, numberFormatAssignmentTarget(file, parent), depth, seenVars)
		case "jump_expression":
			if !strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(parent)), "return") {
				return false
			}
			fn, ok := flatEnclosingAncestor(file, parent, "function_declaration")
			return ok && numberFormatNameIsSink(extractIdentifierFlat(file, fn))
		case "function_body":
			fn, ok := flatEnclosingAncestor(file, parent, "function_declaration")
			return ok && numberFormatNameIsSink(extractIdentifierFlat(file, fn))
		default:
			return false
		}
	}
}

func nameOfProperty(file *scanner.File, prop uint32) string {
	return swallowedPropertyDeclarationName(file, prop)
}

// numberFormatAssignmentTarget returns the assigned variable or property name.
func numberFormatAssignmentTarget(file *scanner.File, assign uint32) string {
	target := file.FlatNamedChild(assign, 0)
	if target == 0 {
		return ""
	}
	if file.FlatType(target) == "directly_assignable_expression" {
		if inner := file.FlatNamedChild(target, 0); inner != 0 && file.FlatNamedChildCount(target) == 1 {
			target = inner
		}
	}
	if file.FlatType(target) == "simple_identifier" {
		return file.FlatNodeString(target, nil)
	}
	return flatNavigationExpressionLastIdentifier(file, target)
}

// numberFormatVariableCarriesToSink checks a sink-named destination, then follows later reads of the variable.
func numberFormatVariableCarriesToSink(file *scanner.File, scope uint32, name string, depth int, seenVars map[string]bool) bool {
	if name == "" {
		return false
	}
	if numberFormatNameIsSink(name) {
		return true
	}
	if depth >= numberFormatMaxAliasDepth || seenVars[name] {
		return false
	}
	seenVars[name] = true
	found := false
	file.FlatWalkNodes(scope, "simple_identifier", func(ident uint32) {
		if found || file.FlatNodeString(ident, nil) != name {
			return
		}
		parent, ok := file.FlatParent(ident)
		if !ok {
			return
		}
		switch file.FlatType(parent) {
		case "variable_declaration", "navigation_suffix", "directly_assignable_expression", "parameter":
			return
		}
		if numberFormatValueFlowsToSink(file, scope, ident, depth+1, seenVars) {
			found = true
		}
	})
	return found
}

// numberFormatCallIsSink matches calls that parse, send, store, or address using their string argument.
func numberFormatCallIsSink(file *scanner.File, call uint32) bool {
	name := flatCallExpressionName(file, call)
	if numberFormatSinkCallees[name] {
		return true
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	if navExpr == 0 {
		return false
	}
	receiver := strings.TrimSpace(file.FlatNodeText(file.FlatNamedChild(navExpr, 0)))
	return (receiver == "Paths" && name == "get") || (receiver == "Path" && name == "of")
}

// numberFormatNameIsSink splits camelCase and snake_case words and looks for key, URL, path, or payload words.
func numberFormatNameIsSink(name string) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "cachekey") || strings.Contains(lower, "filename") {
		return true
	}
	var word []rune
	flush := func() bool {
		defer func() { word = word[:0] }()
		return numberFormatSinkNameWords[strings.ToLower(string(word))]
	}
	for _, r := range name {
		switch {
		case r == '_' || r == '-':
			if flush() {
				return true
			}
		case unicode.IsUpper(r):
			if flush() {
				return true
			}
			word = append(word, r)
		default:
			word = append(word, r)
		}
	}
	return flush()
}
