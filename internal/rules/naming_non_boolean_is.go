package rules

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kaeawc/krit/internal/scanner"
)

// nonBooleanIsBooleanTypes are the declared type names that count as Boolean
// for NonBooleanPropertyPrefixedWithIs: detekt's kotlin.Boolean,
// java.lang.Boolean, and java.util.concurrent.atomic.AtomicBoolean, plus the
// Kotlin and atomicfu AtomicBoolean classes.
var nonBooleanIsBooleanTypes = map[string]bool{
	"Boolean":           true,
	"kotlin.Boolean":    true,
	"java.lang.Boolean": true,
	"AtomicBoolean":     true,
	"java.util.concurrent.atomic.AtomicBoolean": true,
	"kotlin.concurrent.atomics.AtomicBoolean":   true,
	"kotlinx.atomicfu.AtomicBoolean":            true,
}

// nonBooleanIsFunctionInterface matches the function interfaces written as
// user types (Function1<A, R>, KFunction0<R>, KSuspendFunction1<A, R>).
var nonBooleanIsFunctionInterface = regexp.MustCompile(`^K?(Suspend)?Function[0-9]+$`)

// nonBooleanIsAliasDepth bounds type alias expansion, so alias cycles end.
const nonBooleanIsAliasDepth = 4

// hasIsPrefix reports a name with detekt's `is` prefix: "is" followed by an
// uppercase letter, so `issues` and `is_open` do not count. detekt tests one
// UTF-16 unit with Character.isUpperCase, which never holds for a letter
// outside the BMP.
func hasIsPrefix(name string) bool {
	if len(name) <= 2 || !strings.HasPrefix(name, "is") {
		return false
	}
	r, _ := utf8.DecodeRuneInString(name[2:])
	if r > 0xFFFF {
		return false
	}
	return unicode.IsUpper(r) || unicode.Is(unicode.Other_Uppercase, r)
}

// nonBooleanIsDeclaredType returns the type node written after the property's
// colon, or 0 with the recovered text when the grammar wraps the type in an
// ERROR node (an explicit backing field); both are empty without a declared type.
func nonBooleanIsDeclaredType(file *scanner.File, idx uint32) (uint32, string) {
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "variable_declaration" {
			continue
		}
		colonSeen := false
		for typeChild := file.FlatFirstChild(child); typeChild != 0; typeChild = file.FlatNextSib(typeChild) {
			switch file.FlatType(typeChild) {
			case ":":
				colonSeen = true
			case "ERROR":
				if declared := recoveredFieldPropertyType(file.FlatNodeText(typeChild)); declared != "" {
					return 0, declared
				}
			case "user_type", "nullable_type", "function_type", "parenthesized_type", "not_nullable_type":
				if colonSeen {
					return typeChild, file.FlatNodeText(typeChild)
				}
			}
		}
	}
	return 0, ""
}

// nonBooleanIsSkipsType reports a declared type the rule does not report, like
// detekt: a Boolean or AtomicBoolean type, a function type returning one, or a
// type parameter, whose type is unknown. allowFunction is false in a function
// type's return position, where only a Boolean class counts.
func nonBooleanIsSkipsType(file *scanner.File, prop, typeNode uint32) bool {
	return nonBooleanIsBooleanLike(file, prop, typeNode, true, 0)
}

func nonBooleanIsBooleanLike(file *scanner.File, prop, node uint32, allowFunction bool, depth int) bool {
	switch file.FlatType(node) {
	case "nullable_type", "parenthesized_type":
		inner := nonBooleanIsFirstType(file, node)
		return inner != 0 && nonBooleanIsBooleanLike(file, prop, inner, allowFunction, depth)
	case "not_nullable_type":
		// `T & Any` is only legal on a type parameter. detekt skips a type
		// parameter as the property's type, not as a function's return type.
		return allowFunction
	case "function_type":
		if !allowFunction {
			return false
		}
		ret := nonBooleanIsFunctionReturn(file, node)
		return ret != 0 && nonBooleanIsBooleanLike(file, prop, ret, false, depth)
	case "user_type":
		return nonBooleanIsUserTypeBooleanLike(file, prop, node, allowFunction, depth)
	}
	return false
}

func nonBooleanIsUserTypeBooleanLike(file *scanner.File, prop, node uint32, allowFunction bool, depth int) bool {
	var segments []string
	args := uint32(0)
	for c := file.FlatFirstChild(node); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "type_identifier":
			segments = append(segments, file.FlatNodeText(c))
		case "type_arguments":
			args = c
		}
	}
	if len(segments) == 0 {
		return false
	}
	if len(segments) == 1 {
		// A same-file alias wins over the default import of kotlin.Boolean.
		if target := nonBooleanIsSameFileAlias(file, segments[0]); target != 0 {
			return depth < nonBooleanIsAliasDepth && nonBooleanIsBooleanLike(file, prop, target, allowFunction, depth+1)
		}
	}
	if args != 0 {
		if !allowFunction || !nonBooleanIsFunctionInterface.MatchString(segments[len(segments)-1]) {
			return false
		}
		ret := uint32(0)
		for c := file.FlatFirstChild(args); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "type_projection" {
				ret = nonBooleanIsFirstType(file, c)
			}
		}
		return ret != 0 && nonBooleanIsBooleanLike(file, prop, ret, false, depth)
	}
	if len(segments) == 1 {
		name := segments[0]
		if nonBooleanIsTypeParameterInScope(file, prop, name) {
			return allowFunction
		}
		if path := nonBooleanIsImportAliasPath(file, name); path != "" {
			return nonBooleanIsBooleanTypes[path]
		}
	}
	return nonBooleanIsBooleanTypes[strings.Join(segments, ".")]
}

// nonBooleanIsFirstType returns node's first named child that is a type.
func nonBooleanIsFirstType(file *scanner.File, node uint32) uint32 {
	for c := file.FlatFirstChild(node); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "user_type", "nullable_type", "function_type", "parenthesized_type", "not_nullable_type":
			return c
		}
	}
	return 0
}

// nonBooleanIsFunctionReturn returns the return type of a function_type: the
// type after `->`.
func nonBooleanIsFunctionReturn(file *scanner.File, node uint32) uint32 {
	arrow := false
	for c := file.FlatFirstChild(node); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "->" {
			arrow = true
			continue
		}
		if arrow {
			switch file.FlatType(c) {
			case "user_type", "nullable_type", "function_type", "parenthesized_type", "not_nullable_type":
				return c
			}
		}
	}
	return 0
}

// nonBooleanIsSameFileAlias returns the target type node of the top-level
// typealias named name declared in the file, or 0.
func nonBooleanIsSameFileAlias(file *scanner.File, name string) uint32 {
	for decl := file.FlatFirstChild(0); decl != 0; decl = file.FlatNextSib(decl) {
		if file.FlatType(decl) != "type_alias" {
			continue
		}
		alias, ok := file.FlatFindChild(decl, "type_identifier")
		if !ok || file.FlatNodeText(alias) != name {
			continue
		}
		eq := false
		for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) == "=" {
				eq = true
				continue
			}
			if eq {
				switch file.FlatType(c) {
				case "user_type", "nullable_type", "function_type", "parenthesized_type", "not_nullable_type":
					return c
				}
			}
		}
	}
	return 0
}

// nonBooleanIsTypeParameterInScope reports a type parameter named name on the
// property or on an enclosing class or function.
func nonBooleanIsTypeParameterInScope(file *scanner.File, prop uint32, name string) bool {
	for p, ok := prop, true; ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "property_declaration", "class_declaration", "function_declaration":
		default:
			continue
		}
		params, has := file.FlatFindChild(p, "type_parameters")
		if !has {
			continue
		}
		for tp := file.FlatFirstChild(params); tp != 0; tp = file.FlatNextSib(tp) {
			if file.FlatType(tp) != "type_parameter" {
				continue
			}
			if id, ok := file.FlatFindChild(tp, "type_identifier"); ok && file.FlatNodeText(id) == name {
				return true
			}
		}
	}
	return false
}

// nonBooleanIsImportAliasPath returns the imported path of `import path as
// name`, or "".
func nonBooleanIsImportAliasPath(file *scanner.File, name string) string {
	list, ok := file.FlatFindChild(0, "import_list")
	if !ok {
		return ""
	}
	for header := file.FlatFirstChild(list); header != 0; header = file.FlatNextSib(header) {
		if file.FlatType(header) != "import_header" || !file.FlatHasChildOfType(header, "import_alias") {
			continue
		}
		if path, alias, _ := getInstanceImportHeaderParts(file, header); alias == name {
			return path
		}
	}
	return ""
}

// nonBooleanIsRecoveredBoolean reports recovered declared-type text that names
// a Boolean type, without parentheses or a trailing `?`.
func nonBooleanIsRecoveredBoolean(text string) bool {
	text = strings.TrimSpace(text)
	for {
		trimmed := strings.TrimSpace(strings.TrimSuffix(text, "?"))
		if strings.HasPrefix(trimmed, "(") && strings.HasSuffix(trimmed, ")") {
			trimmed = strings.TrimSpace(trimmed[1 : len(trimmed)-1])
		}
		if trimmed == text {
			break
		}
		text = trimmed
	}
	return nonBooleanIsBooleanTypes[text]
}
