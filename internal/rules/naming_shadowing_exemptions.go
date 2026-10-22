package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// isNullNarrowingSelfShadowFlat returns true when `decl` is the canonical
// Kotlin null-narrowing self-shadow idiom, e.g.:
//
//	val foo = foo ?: return
//	val foo = foo?.bar
//	val foo = foo?.bar ?: default
//	val foo = foo?.let { ... } ?: default
//
// The detection walks the initializer AST rather than scanning text — this
// avoids false matches on byte sequences like `?:` that may appear in
// comments, string literals, or unrelated subexpressions.
func isNullNarrowingSelfShadowFlat(file *scanner.File, decl uint32, name string) bool {
	init := declInitializerFlat(file, decl)
	if init == 0 {
		return false
	}
	// Strip parens.
	for file.FlatType(init) == "parenthesized_expression" && file.FlatNamedChildCount(init) > 0 {
		init = file.FlatNamedChild(init, 0)
	}
	if isCheckNotNullSelfShadowFlat(file, init, name) {
		return true
	}
	// The initializer must use `?:` (elvis) or `?.` (safe call) somewhere
	// in its leftmost spine.
	requiresNullOp := false
	cur := init
	for {
		switch file.FlatType(cur) {
		case "parenthesized_expression":
			if file.FlatNamedChildCount(cur) == 0 {
				return false
			}
			cur = file.FlatNamedChild(cur, 0)
			continue
		case "elvis_expression":
			requiresNullOp = true
			if file.FlatNamedChildCount(cur) == 0 {
				return false
			}
			cur = file.FlatNamedChild(cur, 0)
			continue
		case "navigation_expression":
			if flatNavigationHasSafeCall(file, cur) {
				requiresNullOp = true
			}
			if file.FlatNamedChildCount(cur) == 0 {
				return false
			}
			cur = file.FlatNamedChild(cur, 0)
			continue
		case "call_expression":
			// Descend into the callee/receiver (first named child).
			if file.FlatNamedChildCount(cur) == 0 {
				return false
			}
			cur = file.FlatNamedChild(cur, 0)
			continue
		case "simple_identifier":
			return requiresNullOp && file.FlatNodeText(cur) == name
		default:
			return false
		}
	}
}

func isCheckNotNullSelfShadowFlat(file *scanner.File, init uint32, name string) bool {
	if file.FlatType(init) != "call_expression" {
		return false
	}
	callName := flatCallNameAny(file, init)
	if callName != "checkNotNull" && callName != "requireNotNull" {
		return false
	}
	_, args := flatCallExpressionParts(file, init)
	if args == 0 {
		return false
	}
	for i := 0; i < file.FlatNamedChildCount(args); i++ {
		arg := file.FlatNamedChild(args, i)
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		argText := strings.TrimSpace(file.FlatNodeText(arg))
		if eq := strings.Index(argText, "="); eq >= 0 {
			argText = strings.TrimSpace(argText[eq+1:])
		}
		return argText == name
	}
	return false
}

func isScopeFunctionSelfConfigurationShadowFlat(file *scanner.File, decl uint32, name string) bool {
	init := declInitializerFlat(file, decl)
	if init == 0 {
		return false
	}
	for file.FlatType(init) == "parenthesized_expression" && file.FlatNamedChildCount(init) > 0 {
		init = file.FlatNamedChild(init, 0)
	}
	if file.FlatType(init) != "call_expression" {
		return false
	}
	callName := flatCallNameAny(file, init)
	switch callName {
	case "apply", "also", "run", "let":
	default:
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(init))
	return strings.HasPrefix(text, name+"."+callName) ||
		strings.HasPrefix(text, name+"?."+callName)
}

func noNameShadowHasLocalSuppression(file *scanner.File, decl uint32) bool {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok || mods == 0 {
		return false
	}
	text := file.FlatNodeText(mods)
	return strings.Contains(text, "NAME_SHADOWING") ||
		strings.Contains(text, "NoNameShadowing") ||
		strings.Contains(text, "detekt:NoNameShadowing") ||
		strings.Contains(text, "detekt.NoNameShadowing")
}

func isSimpleSelfAliasShadowFlat(file *scanner.File, decl uint32, name string) bool {
	text := file.FlatNodeText(decl)
	eq := strings.Index(text, "=")
	if eq < 0 {
		return false
	}
	rhs := strings.TrimSpace(text[eq+1:])
	rhs = strings.TrimPrefix(rhs, "this.")
	return rhs == name
}

func noNameShadowIsConstructorBackedClassPropertyFlat(file *scanner.File, decl uint32, name string) bool {
	if file.FlatType(decl) != "property_declaration" {
		return false
	}
	var classBody uint32
	for p, ok := file.FlatParent(decl); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "class_body":
			classBody = p
		case "function_body", "source_file":
			return false
		}
		if classBody != 0 {
			break
		}
	}
	if classBody == 0 {
		return false
	}
	classDecl, ok := file.FlatParent(classBody)
	if !ok {
		return false
	}
	for classDecl != 0 && file.FlatType(classDecl) != "class_declaration" {
		var parentOK bool
		classDecl, parentOK = file.FlatParent(classDecl)
		if !parentOK {
			return false
		}
	}
	if !propertyInitializerContainsIdentifierFlat(file, decl, name) {
		return false
	}
	return classHeaderHasNonPropertyParameterNamedFlat(file, classDecl, name) ||
		classHeaderTextBeforeDeclarationContainsParamFlat(file, classDecl, decl, name)
}

func noNameShadowIsSetterAssignPatternFlat(file *scanner.File, decl uint32, name string) bool {
	if file.FlatType(decl) != "property_declaration" && file.FlatType(decl) != "variable_declaration" {
		return false
	}
	parent, ok := file.FlatParent(decl)
	if !ok {
		return false
	}
	next, ok := noNameShadowNextNamedSiblingInParent(file, parent, decl)
	if !ok || !noNameShadowIsThisAssignmentFromName(file, next, name) {
		return false
	}
	return !noNameShadowIdentifierUsedAfterSibling(file, parent, next, name)
}

func noNameShadowIsMutableDerivedCopyFlat(file *scanner.File, decl uint32, name string) bool {
	prop := decl
	if file.FlatType(prop) == "variable_declaration" {
		parent, ok := file.FlatParent(prop)
		if !ok || file.FlatType(parent) != "property_declaration" {
			return false
		}
		prop = parent
	}
	if file.FlatType(prop) != "property_declaration" || !propertyDeclarationIsVar(file, prop) {
		return false
	}
	init := declInitializerFlat(file, prop)
	if init == 0 {
		return false
	}
	for file.FlatType(init) == "parenthesized_expression" && file.FlatNamedChildCount(init) > 0 {
		init = file.FlatNamedChild(init, 0)
	}
	receiver := noNameShadowDerivedCopyReceiverFlat(file, init)
	if receiver == "" || receiver == name {
		return false
	}
	parent, ok := file.FlatParent(prop)
	if !ok {
		return false
	}
	return noNameShadowIdentifierAssignedAfterSibling(file, parent, prop, name)
}

func noNameShadowDerivedCopyReceiverFlat(file *scanner.File, init uint32) string {
	if file.FlatType(init) != "navigation_expression" {
		return ""
	}
	text := strings.TrimSpace(file.FlatNodeText(init))
	dot := strings.Index(text, ".")
	safeDot := strings.Index(text, "?.")
	if safeDot >= 0 && (dot < 0 || safeDot < dot) {
		dot = safeDot
	}
	if dot <= 0 {
		return ""
	}
	receiver := strings.TrimSpace(text[:dot])
	if !isSimpleIdentifierText(receiver) {
		return ""
	}
	return receiver
}

func isSimpleIdentifierText(text string) bool {
	if text == "" || !isIdentStartByte(text[0]) {
		return false
	}
	for i := 1; i < len(text); i++ {
		if !isIdentByte(text[i]) {
			return false
		}
	}
	return true
}

func noNameShadowIsSetterParamFlat(file *scanner.File, fn uint32, name string) bool {
	if fn == 0 || file.FlatType(fn) != "function_declaration" {
		return false
	}
	body, ok := file.FlatFindChild(fn, "function_body")
	if !ok || body == 0 {
		return false
	}
	stmts, _ := file.FlatFindChild(body, "statements")
	if stmts == 0 {
		stmts = body
	}
	first := noNameShadowFirstNamedChild(file, stmts)
	if first == 0 || !noNameShadowIsThisAssignmentFromName(file, first, name) {
		return false
	}
	return !noNameShadowIdentifierUsedAfterSibling(file, stmts, first, name)
}

func noNameShadowIsThisAssignmentFromName(file *scanner.File, stmt uint32, name string) bool {
	text := strings.TrimSpace(file.FlatNodeText(stmt))
	eq := strings.Index(text, "=")
	if eq < 0 {
		return false
	}
	lhs := strings.TrimSpace(text[:eq])
	rhs := strings.TrimSpace(text[eq+1:])
	return (lhs == "this."+name || noNameShadowIsLabeledThisProperty(lhs, name)) && rhs == name
}

func noNameShadowIsLabeledThisProperty(lhs string, name string) bool {
	if !strings.HasPrefix(lhs, "this@") {
		return false
	}
	dot := strings.LastIndex(lhs, ".")
	if dot < 0 || dot == len(lhs)-1 {
		return false
	}
	return lhs[dot+1:] == name
}

func noNameShadowIdentifierUsedAfterSibling(file *scanner.File, parent uint32, sibling uint32, name string) bool {
	pastSibling := false
	for i := 0; i < file.FlatNamedChildCount(parent); i++ {
		child := file.FlatNamedChild(parent, i)
		if child == sibling {
			pastSibling = true
			continue
		}
		if !pastSibling {
			continue
		}
		if containsIdentifierToken(file.FlatNodeText(child), name) {
			return true
		}
	}
	return false
}

func noNameShadowIdentifierAssignedAfterSibling(file *scanner.File, parent uint32, sibling uint32, name string) bool {
	pastSibling := false
	for i := 0; i < file.FlatNamedChildCount(parent); i++ {
		child := file.FlatNamedChild(parent, i)
		if child == sibling {
			pastSibling = true
			continue
		}
		if !pastSibling {
			continue
		}
		if noNameShadowTextAssignsBareIdentifier(file.FlatNodeText(child), name) {
			return true
		}
	}
	return false
}

func noNameShadowTextAssignsBareIdentifier(text, name string) bool {
	if name == "" {
		return false
	}
	start := 0
	for {
		idx := strings.Index(text[start:], name)
		if idx < 0 {
			return false
		}
		pos := start + idx
		after := pos + len(name)
		beforeOK := pos == 0 || !isIdentByte(text[pos-1])
		afterOK := after == len(text) || !isIdentByte(text[after])
		if beforeOK && afterOK {
			rest := strings.TrimLeft(text[after:], " \t\r\n")
			if strings.HasPrefix(rest, "=") && !strings.HasPrefix(rest, "==") && !strings.HasPrefix(rest, "=>") {
				return true
			}
		}
		start = after
		if start >= len(text) {
			return false
		}
	}
}

func noNameShadowFirstNamedChild(file *scanner.File, parent uint32) uint32 {
	for i := 0; i < file.FlatNamedChildCount(parent); i++ {
		child := file.FlatNamedChild(parent, i)
		if child != 0 {
			return child
		}
	}
	return 0
}

func noNameShadowNextNamedSiblingInParent(file *scanner.File, parent uint32, node uint32) (uint32, bool) {
	pastNode := false
	for i := 0; i < file.FlatNamedChildCount(parent); i++ {
		child := file.FlatNamedChild(parent, i)
		if child == node {
			pastNode = true
			continue
		}
		if pastNode && child != 0 {
			return child, true
		}
	}
	return 0, false
}

func classHeaderHasNonPropertyParameterNamedFlat(file *scanner.File, classDecl uint32, name string) bool {
	var found bool
	var visit func(uint32)
	visit = func(node uint32) {
		if found {
			return
		}
		for i := 0; i < file.FlatNamedChildCount(node); i++ {
			child := file.FlatNamedChild(node, i)
			childType := file.FlatType(child)
			if childType == "class_body" {
				continue
			}
			if childType == "class_parameter" {
				if !classParameterDefinesPropertyFlat(file, child) && extractIdentifierFlat(file, child) == name {
					found = true
					return
				}
				continue
			}
			visit(child)
		}
	}
	visit(classDecl)
	return found
}

func propertyInitializerContainsIdentifierFlat(file *scanner.File, decl uint32, name string) bool {
	text := file.FlatNodeText(decl)
	eq := strings.Index(text, "=")
	if eq < 0 || eq+1 >= len(text) {
		return false
	}
	return containsIdentifierToken(text[eq+1:], name)
}

func classHeaderTextBeforeDeclarationContainsParamFlat(file *scanner.File, classDecl, decl uint32, name string) bool {
	start := int(file.FlatStartByte(classDecl))
	end := int(file.FlatStartByte(decl))
	if start < 0 || end <= start || end > len(file.Content) {
		return false
	}
	header := string(file.Content[start:end])
	if !strings.Contains(header, "constructor(") && !strings.Contains(header, "class ") {
		return false
	}
	return containsIdentifierToken(header, name)
}
