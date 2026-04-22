package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/rules/semantics"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

func flatNonNullCheckText(file *scanner.File, idx uint32, funcName string) (argText string, lambdaText string, ok bool) {
	if file == nil || file.FlatType(idx) != "call_expression" {
		return "", "", false
	}
	if flatCallExpressionName(file, idx) != funcName {
		return "", "", false
	}
	suffix, _ := file.FlatFindChild(idx, "call_suffix")
	if suffix == 0 {
		return "", "", false
	}
	var eq uint32
	file.FlatWalkNodes(suffix, "equality_expression", func(candidate uint32) {
		if eq == 0 {
			eq = candidate
		}
	})
	if eq == 0 || file.FlatType(eq) != "equality_expression" || file.FlatChildCount(eq) < 3 {
		return "", "", false
	}
	left := file.FlatChild(eq, 0)
	op := file.FlatChild(eq, 1)
	right := file.FlatChild(eq, file.FlatChildCount(eq)-1)
	if left == 0 || op == 0 || right == 0 || !file.FlatNodeTextEquals(op, "!=") {
		return "", "", false
	}
	leftText := strings.TrimSpace(file.FlatNodeText(left))
	rightText := strings.TrimSpace(file.FlatNodeText(right))
	if leftText == "null" {
		argText = rightText
	} else if rightText == "null" {
		argText = leftText
	} else {
		return "", "", false
	}
	if argText == "" {
		return "", "", false
	}
	if lambda := flatCallSuffixLambdaNode(file, suffix); lambda != 0 {
		lambdaText = file.FlatNodeText(lambda)
	}
	return argText, lambdaText, true
}

type nullOrEmptyCheckKind uint8

const (
	nullOrEmptyCheckUnknown nullOrEmptyCheckKind = iota
	nullOrEmptyCheckIsEmpty
	nullOrEmptyCheckSize
	nullOrEmptyCheckLength
	nullOrEmptyCheckCount
	nullOrEmptyCheckEmptyString
)

type flatNullOrEmptyCheck struct {
	receiver uint32
	kind     nullOrEmptyCheckKind
}

func flatUseIsNullOrEmpty(ctx *v2.Context, base BaseRule) {
	if ctx == nil || ctx.File == nil || ctx.File.FlatType(ctx.Idx) != "disjunction_expression" {
		return
	}
	file := ctx.File
	left, right := flatBinaryExpressionOperands(file, ctx.Idx)
	if left == 0 || right == 0 {
		return
	}
	nullReceiver := flatNullOrEmptyNullCheckedReceiver(file, left)
	if nullReceiver == 0 {
		return
	}
	emptyCheck := flatNullOrEmptyEmptinessCheck(ctx, right)
	if emptyCheck.receiver == 0 || !flatSameReferencePath(file, nullReceiver, emptyCheck.receiver) {
		return
	}
	if !flatNullOrEmptyReceiverSupported(ctx, nullReceiver, emptyCheck.kind) {
		return
	}
	if flatInsideNullOrEmptyHelper(file, ctx.Idx) {
		return
	}
	receiverText := strings.TrimSpace(file.FlatNodeText(flatUnwrapParenExpr(file, nullReceiver)))
	if receiverText == "" {
		return
	}
	f := base.Finding(file, file.FlatRow(ctx.Idx)+1, file.FlatCol(ctx.Idx)+1,
		"Use 'isNullOrEmpty()' instead of 'x == null || x.isEmpty()'.")
	f.Fix = &scanner.Fix{
		ByteMode:    true,
		StartByte:   int(file.FlatStartByte(ctx.Idx)),
		EndByte:     int(file.FlatEndByte(ctx.Idx)),
		Replacement: receiverText + ".isNullOrEmpty()",
	}
	ctx.Emit(f)
}

func flatBinaryExpressionOperands(file *scanner.File, idx uint32) (uint32, uint32) {
	if file == nil || idx == 0 || file.FlatNamedChildCount(idx) != 2 {
		return 0, 0
	}
	return file.FlatNamedChild(idx, 0), file.FlatNamedChild(idx, 1)
}

func flatNullOrEmptyNullCheckedReceiver(file *scanner.File, node uint32) uint32 {
	node = flatUnwrapParenExpr(file, node)
	left, op, right := flatEqualityExpressionParts(file, node)
	if left == 0 || right == 0 || op != "==" {
		return 0
	}
	switch {
	case flatNullOrEmptyIsNullLiteral(file, right):
		return flatUnwrapParenExpr(file, left)
	case flatNullOrEmptyIsNullLiteral(file, left):
		return flatUnwrapParenExpr(file, right)
	default:
		return 0
	}
}

func flatNullOrEmptyEmptinessCheck(ctx *v2.Context, node uint32) flatNullOrEmptyCheck {
	file := ctx.File
	node = flatUnwrapParenExpr(file, node)
	switch file.FlatType(node) {
	case "call_expression":
		return flatNullOrEmptyCallCheck(ctx, node)
	case "equality_expression":
		return flatNullOrEmptyEqualityCheck(ctx, node)
	default:
		return flatNullOrEmptyCheck{}
	}
}

func flatNullOrEmptyCallCheck(ctx *v2.Context, call uint32) flatNullOrEmptyCheck {
	file := ctx.File
	if flatCallExpressionName(file, call) != "isEmpty" || !flatCallHasNoValueArgs(file, call) {
		return flatNullOrEmptyCheck{}
	}
	if !flatResolvedEmptyCallTargetAllowed(ctx, call, "isEmpty") {
		return flatNullOrEmptyCheck{}
	}
	navExpr, _ := flatCallExpressionParts(file, call)
	receiver := flatNavigationReceiver(file, navExpr)
	if receiver == 0 {
		return flatNullOrEmptyCheck{}
	}
	return flatNullOrEmptyCheck{receiver: flatUnwrapParenExpr(file, receiver), kind: nullOrEmptyCheckIsEmpty}
}

func flatNullOrEmptyEqualityCheck(ctx *v2.Context, node uint32) flatNullOrEmptyCheck {
	file := ctx.File
	left, op, right := flatEqualityExpressionParts(file, node)
	if left == 0 || right == 0 || op != "==" {
		return flatNullOrEmptyCheck{}
	}
	if flatIsEmptyStringLiteral(file, right) {
		return flatNullOrEmptyCheck{receiver: flatUnwrapParenExpr(file, left), kind: nullOrEmptyCheckEmptyString}
	}
	if flatIsEmptyStringLiteral(file, left) {
		return flatNullOrEmptyCheck{receiver: flatUnwrapParenExpr(file, right), kind: nullOrEmptyCheckEmptyString}
	}
	if flatIsZeroLiteral(file, right) {
		return flatNullOrEmptySizeLikeCheck(ctx, left)
	}
	if flatIsZeroLiteral(file, left) {
		return flatNullOrEmptySizeLikeCheck(ctx, right)
	}
	return flatNullOrEmptyCheck{}
}

func flatNullOrEmptySizeLikeCheck(ctx *v2.Context, node uint32) flatNullOrEmptyCheck {
	file := ctx.File
	node = flatUnwrapParenExpr(file, node)
	switch file.FlatType(node) {
	case "call_expression":
		if flatCallExpressionName(file, node) != "count" || !flatCallHasNoValueArgs(file, node) {
			return flatNullOrEmptyCheck{}
		}
		if !flatResolvedEmptyCallTargetAllowed(ctx, node, "count") {
			return flatNullOrEmptyCheck{}
		}
		navExpr, _ := flatCallExpressionParts(file, node)
		receiver := flatNavigationReceiver(file, navExpr)
		if receiver == 0 {
			return flatNullOrEmptyCheck{}
		}
		return flatNullOrEmptyCheck{receiver: flatUnwrapParenExpr(file, receiver), kind: nullOrEmptyCheckCount}
	case "navigation_expression":
		propName := flatNullOrEmptyNavSelector(file, node)
		switch propName {
		case "size":
			if receiver := flatNavigationReceiver(file, node); receiver != 0 {
				return flatNullOrEmptyCheck{receiver: flatUnwrapParenExpr(file, receiver), kind: nullOrEmptyCheckSize}
			}
		case "length":
			if receiver := flatNavigationReceiver(file, node); receiver != 0 {
				return flatNullOrEmptyCheck{receiver: flatUnwrapParenExpr(file, receiver), kind: nullOrEmptyCheckLength}
			}
		}
	}
	return flatNullOrEmptyCheck{}
}

func flatEqualityExpressionParts(file *scanner.File, node uint32) (uint32, string, uint32) {
	node = flatUnwrapParenExpr(file, node)
	if file == nil || node == 0 || file.FlatType(node) != "equality_expression" || file.FlatChildCount(node) < 3 {
		return 0, "", 0
	}
	left := file.FlatChild(node, 0)
	right := file.FlatChild(node, file.FlatChildCount(node)-1)
	op := ""
	for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			continue
		}
		text := strings.TrimSpace(file.FlatNodeText(child))
		if text == "==" || text == "!=" || text == "===" || text == "!==" {
			op = text
			break
		}
	}
	return left, op, right
}

func flatNavigationReceiver(file *scanner.File, nav uint32) uint32 {
	if file == nil || nav == 0 || file.FlatType(nav) != "navigation_expression" || file.FlatNamedChildCount(nav) == 0 {
		return 0
	}
	return file.FlatNamedChild(nav, 0)
}

func flatCallHasNoValueArgs(file *scanner.File, call uint32) bool {
	_, args := flatCallExpressionParts(file, call)
	return args == 0 || file.FlatNamedChildCount(args) == 0
}

func flatResolvedEmptyCallTargetAllowed(ctx *v2.Context, call uint32, name string) bool {
	target, ok := semantics.ResolveCallTarget(ctx, call)
	if !ok || target.CalleeName != name {
		return false
	}
	if !target.Resolved {
		return true
	}
	qn := target.QualifiedName
	return strings.HasPrefix(qn, "kotlin.") || strings.HasPrefix(qn, "java.")
}

func flatNullOrEmptyReceiverSupported(ctx *v2.Context, receiver uint32, kind nullOrEmptyCheckKind) bool {
	if ctx == nil || ctx.File == nil || receiver == 0 {
		return false
	}
	if ctx.Resolver != nil {
		nullable, nullableOK := semantics.IsNullableExpression(ctx, receiver)
		typ, typeOK := semantics.ExpressionType(ctx, receiver)
		if nullableOK && typeOK && nullable && flatNullOrEmptyKindSupportsFamily(kind, flatNullOrEmptyTypeFamily(typ)) {
			return true
		}
	}
	explicitType, nullable, ok := flatNullOrEmptyExplicitReceiverType(ctx.File, receiver)
	if !ok || !nullable {
		return false
	}
	return flatNullOrEmptyKindSupportsFamily(kind, flatNullOrEmptyTypeFamilyFromName(explicitType))
}

func flatNullOrEmptyKindSupportsFamily(kind nullOrEmptyCheckKind, family string) bool {
	switch kind {
	case nullOrEmptyCheckIsEmpty:
		return family == "string" || family == "collection" || family == "map" || family == "array" || family == "sequence"
	case nullOrEmptyCheckSize:
		return family == "collection" || family == "map" || family == "array"
	case nullOrEmptyCheckLength:
		return family == "string"
	case nullOrEmptyCheckCount:
		return family == "string" || family == "collection" || family == "array" || family == "sequence"
	case nullOrEmptyCheckEmptyString:
		return family == "string"
	default:
		return false
	}
}

func flatNullOrEmptyTypeFamily(typ semantics.TypeInfo) string {
	names := []string{typ.FQN, typ.Name}
	if typ.Type != nil {
		names = append(names, typ.Type.FQN, typ.Type.Name)
	}
	for _, name := range names {
		if family := flatNullOrEmptyTypeFamilyFromName(name); family != "" {
			return family
		}
	}
	return ""
}

func flatNullOrEmptyTypeFamilyFromName(name string) string {
	name = flatBaseTypeName(name)
	switch name {
	case "String", "kotlin.String", "CharSequence", "kotlin.CharSequence", "java.lang.String":
		return "string"
	case "Collection", "kotlin.collections.Collection",
		"List", "kotlin.collections.List",
		"Set", "kotlin.collections.Set",
		"MutableCollection", "kotlin.collections.MutableCollection",
		"MutableList", "kotlin.collections.MutableList",
		"MutableSet", "kotlin.collections.MutableSet",
		"java.util.Collection", "java.util.List", "java.util.Set":
		return "collection"
	case "Map", "kotlin.collections.Map", "MutableMap", "kotlin.collections.MutableMap", "java.util.Map":
		return "map"
	case "Sequence", "kotlin.sequences.Sequence":
		return "sequence"
	}
	if name == "Array" || name == "kotlin.Array" || strings.HasSuffix(name, "Array") {
		return "array"
	}
	return ""
}

func flatBaseTypeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, "?")
	if i := strings.IndexByte(name, '<'); i >= 0 {
		name = name[:i]
	}
	return strings.TrimSpace(name)
}

func flatNullOrEmptyExplicitReceiverType(file *scanner.File, receiver uint32) (string, bool, bool) {
	path := flatNullOrEmptyReferencePath(file, receiver)
	if len(path) == 0 {
		return "", false, false
	}
	explicitThis := path[0] == "this"
	if len(path) > 1 && !explicitThis {
		return "", false, false
	}
	name := path[len(path)-1]
	if !explicitThis {
		if fn, ok := flatEnclosingFunction(file, receiver); ok {
			if typ, nullable, ok := flatNullOrEmptyFunctionParamType(file, fn, name); ok {
				return typ, nullable, true
			}
		}
	}
	if classNode, ok := flatEnclosingAncestor(file, receiver, "class_declaration", "object_declaration", "interface_declaration"); ok {
		if typ, nullable, ok := flatNullOrEmptyClassMemberType(file, classNode, name); ok {
			return typ, nullable, true
		}
	}
	return "", false, false
}

func flatNullOrEmptyFunctionParamType(file *scanner.File, fn uint32, name string) (string, bool, bool) {
	params, _ := file.FlatFindChild(fn, "function_value_parameters")
	if params == 0 {
		return "", false, false
	}
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" || flatNullOrEmptyDeclarationName(file, child) != name {
			continue
		}
		return flatNullOrEmptyDeclarationType(file, child)
	}
	return "", false, false
}

func flatNullOrEmptyClassMemberType(file *scanner.File, classNode uint32, name string) (string, bool, bool) {
	var typ string
	var nullable bool
	var ok bool
	file.FlatWalkAllNodes(classNode, func(candidate uint32) {
		if ok {
			return
		}
		switch file.FlatType(candidate) {
		case "function_declaration":
			return
		case "class_parameter", "parameter", "property_declaration":
			if flatNullOrEmptyDeclarationName(file, candidate) != name || !flatNullOrEmptyBelongsDirectlyToClass(file, candidate, classNode) {
				return
			}
			typ, nullable, ok = flatNullOrEmptyDeclarationType(file, candidate)
		}
	})
	return typ, nullable, ok
}

func flatNullOrEmptyBelongsDirectlyToClass(file *scanner.File, node uint32, classNode uint32) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		if p == classNode {
			return true
		}
		switch file.FlatType(p) {
		case "function_declaration", "class_declaration", "object_declaration", "interface_declaration":
			return false
		}
	}
	return false
}

func flatNullOrEmptyDeclarationName(file *scanner.File, decl uint32) string {
	if name := semantics.DeclarationName(file, decl); name != "" {
		return name
	}
	if file == nil || decl == 0 {
		return ""
	}
	for child := file.FlatFirstChild(decl); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "simple_identifier" {
			return file.FlatNodeString(child, nil)
		}
	}
	return ""
}

func flatNullOrEmptyDeclarationType(file *scanner.File, decl uint32) (string, bool, bool) {
	var typeNode uint32
	var priority int
	file.FlatWalkAllNodes(decl, func(candidate uint32) {
		p := 0
		switch file.FlatType(candidate) {
		case "nullable_type":
			p = 3
		case "user_type":
			p = 2
		case "type_identifier":
			p = 1
		}
		if p > priority {
			priority = p
			typeNode = candidate
		}
	})
	if typeNode == 0 {
		return "", false, false
	}
	text := strings.TrimSpace(file.FlatNodeText(typeNode))
	return text, strings.Contains(text, "?"), true
}

func flatSameReferencePath(file *scanner.File, left uint32, right uint32) bool {
	leftPath := flatNullOrEmptyReferencePath(file, left)
	rightPath := flatNullOrEmptyReferencePath(file, right)
	if len(leftPath) == 0 || len(leftPath) != len(rightPath) {
		return false
	}
	for i := range leftPath {
		if leftPath[i] != rightPath[i] {
			return false
		}
	}
	return true
}

func flatNullOrEmptyReferencePath(file *scanner.File, node uint32) []string {
	node = flatUnwrapParenExpr(file, node)
	if file == nil || node == 0 {
		return nil
	}
	switch file.FlatType(node) {
	case "simple_identifier", "type_identifier":
		return []string{file.FlatNodeString(node, nil)}
	case "this_expression":
		return []string{"this"}
	case "navigation_expression":
		var parts []string
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if !file.FlatIsNamed(child) {
				continue
			}
			switch file.FlatType(child) {
			case "simple_identifier", "type_identifier":
				parts = append(parts, file.FlatNodeString(child, nil))
			case "this_expression":
				parts = append(parts, "this")
			case "navigation_suffix":
				name := ""
				for gc := file.FlatFirstChild(child); gc != 0; gc = file.FlatNextSib(gc) {
					if file.FlatIsNamed(gc) && file.FlatType(gc) == "simple_identifier" {
						name = file.FlatNodeString(gc, nil)
					}
				}
				if name == "" {
					return nil
				}
				parts = append(parts, name)
			case "navigation_expression":
				sub := flatNullOrEmptyReferencePath(file, child)
				if len(sub) == 0 {
					return nil
				}
				parts = append(parts, sub...)
			case "parenthesized_expression":
				sub := flatNullOrEmptyReferencePath(file, child)
				if len(sub) == 0 {
					return nil
				}
				parts = append(parts, sub...)
			default:
				return nil
			}
		}
		return parts
	default:
		return nil
	}
}

func flatNullOrEmptyIsNullLiteral(file *scanner.File, node uint32) bool {
	node = flatUnwrapParenExpr(file, node)
	return file != nil && node != 0 && strings.TrimSpace(file.FlatNodeText(node)) == "null"
}

func flatIsZeroLiteral(file *scanner.File, node uint32) bool {
	node = flatUnwrapParenExpr(file, node)
	return file != nil && node != 0 && strings.TrimSpace(file.FlatNodeText(node)) == "0"
}

func flatIsEmptyStringLiteral(file *scanner.File, node uint32) bool {
	node = flatUnwrapParenExpr(file, node)
	if file == nil || node == 0 {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(node))
	return text == `""` || text == `""""""`
}

func flatInsideNullOrEmptyHelper(file *scanner.File, idx uint32) bool {
	for p, ok := file.FlatParent(idx); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) != "function_declaration" {
			continue
		}
		switch extractIdentifierFlat(file, p) {
		case "isNullOrEmpty", "isEmpty", "isNullOrBlank", "isBlank":
			return true
		default:
			return false
		}
	}
	return false
}

func flatThrowPattern(ctx *v2.Context, nodeType, nodeText string, exceptionType, replacement string, base BaseRule) {
	idx, file := ctx.Idx, ctx.File
	if file == nil || nodeType != "if_expression" {
		return
	}
	if strings.Contains(nodeText, "else") && file.FlatHasChildOfType(idx, "else") {
		return
	}
	var condNode, bodyNode uint32
	for i := 0; i < file.FlatChildCount(idx); i++ {
		child := file.FlatChild(idx, i)
		switch file.FlatType(child) {
		case "if", "(", ")", "{", "}":
			continue
		default:
			if condNode == 0 {
				condNode = child
			} else if bodyNode == 0 {
				bodyNode = child
			}
		}
	}
	if condNode == 0 || bodyNode == 0 {
		return
	}
	condText := strings.TrimSpace(file.FlatNodeText(condNode))
	isNegated := false
	innerCond := condText
	if strings.HasPrefix(condText, "!") {
		isNegated = true
		innerCond = strings.TrimSpace(condText[1:])
		if strings.HasPrefix(innerCond, "(") && strings.HasSuffix(innerCond, ")") {
			innerCond = innerCond[1 : len(innerCond)-1]
		}
	} else if file.FlatType(condNode) == "prefix_expression" && file.FlatChildCount(condNode) >= 2 {
		opNode := file.FlatChild(condNode, 0)
		if opNode != 0 && file.FlatNodeTextEquals(opNode, "!") {
			isNegated = true
			if argNode := file.FlatChild(condNode, 1); argNode != 0 {
				innerCond = file.FlatNodeText(argNode)
			}
		}
	}
	if !isNegated {
		return
	}
	bodyText := strings.TrimSpace(file.FlatNodeText(bodyNode))
	if strings.HasPrefix(bodyText, "{") && strings.HasSuffix(bodyText, "}") {
		bodyText = strings.TrimSpace(bodyText[1 : len(bodyText)-1])
	}
	if !strings.HasPrefix(bodyText, "throw ") {
		return
	}
	throwTarget := strings.TrimSpace(bodyText[6:])
	if !strings.HasPrefix(throwTarget, exceptionType) {
		return
	}
	f := base.Finding(file, file.FlatRow(idx)+1, 1, fmt.Sprintf("Use '%s()' instead of 'if (...) throw %s'.", replacement, exceptionType))
	if argStart := strings.Index(throwTarget, "("); argStart >= 0 {
		if argEnd := strings.LastIndex(throwTarget, ")"); argEnd > argStart {
			arg := strings.TrimSpace(throwTarget[argStart+1 : argEnd])
			if strings.HasPrefix(arg, "\"") && strings.HasSuffix(arg, "\"") {
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)), Replacement: fmt.Sprintf("%s(%s) { %s }", replacement, innerCond, arg)}
			}
		}
	}
	ctx.Emit(f)
}

func flatNullOrEmptyNavSelector(file *scanner.File, nav uint32) string {
	if file == nil || nav == 0 || file.FlatNamedChildCount(nav) < 2 {
		return ""
	}
	lastChild := file.FlatNamedChild(nav, file.FlatNamedChildCount(nav)-1)
	if lastChild == 0 {
		return ""
	}
	if file.FlatType(lastChild) == "navigation_suffix" {
		for i := 0; i < file.FlatNamedChildCount(lastChild); i++ {
			ident := file.FlatNamedChild(lastChild, i)
			if file.FlatType(ident) == "simple_identifier" {
				return file.FlatNodeText(ident)
			}
		}
	}
	return file.FlatNodeText(lastChild)
}

func flatIsEmptyRHS(file *scanner.File, node uint32) bool {
	if file == nil || node == 0 {
		return false
	}
	text := strings.TrimSpace(file.FlatNodeText(node))
	if text == `""` || text == `""""""` {
		return true
	}
	switch file.FlatType(node) {
	case "call_expression":
		name := flatCallExpressionName(file, node)
		if useOrEmptyFunctions[name] {
			return true
		}
		if useOrEmptyFactoryFunctions[name] {
			_, args := flatCallExpressionParts(file, node)
			return args == 0 || file.FlatNamedChildCount(args) == 0
		}
	}
	return false
}

// UseCheckNotNullRule detects check(x != null) and suggests checkNotNull(x).
// Uses AST dispatch on call_expression for precise detection, handling both
// `x != null` and `null != x` argument order, nested expressions, and
// optional message lambdas like `check(x != null) { "msg" }`.
type UseCheckNotNullRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — suggests
// checkNotNull over `if (x == null) throw`; pattern-based with resolver
// used to confirm nullability when available. Classified per roadmap/17.
func (r *UseCheckNotNullRule) Confidence() float64 { return 0.75 }

// UseRequireNotNullRule detects require(x != null) and suggests requireNotNull(x).
// Uses AST dispatch on call_expression for precise detection, handling both
// `x != null` and `null != x` argument order, nested expressions, and
// optional message lambdas like `require(x != null) { "msg" }`.
type UseRequireNotNullRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — suggests
// requireNotNull over `if (x == null) throw IAE`; pattern-based with
// resolver confirmation when available. Classified per roadmap/17.
func (r *UseRequireNotNullRule) Confidence() float64 { return 0.75 }

// UseCheckOrErrorRule detects `if (!x) throw IllegalStateException`.
type UseCheckOrErrorRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseCheckOrErrorRule) Confidence() float64 { return 0.75 }

// UseRequireRule detects `if (!x) throw IllegalArgumentException`.
type UseRequireRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseRequireRule) Confidence() float64 { return 0.75 }

// UseIsNullOrEmptyRule detects `x == null || x.isEmpty()` and related patterns
// such as `x == null || x.count() == 0`, `x == null || x.size == 0`,
// `x == null || x.length == 0`, and `x == null || x == ""`.
// Uses tree-sitter DispatchRule on disjunction_expression for structural accuracy.
type UseIsNullOrEmptyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence — suggests
// isNullOrEmpty() for `x == null || x.isEmpty()`; needs resolver to confirm
// x is String/Collection, falls back to name heuristic. Classified per
// roadmap/17.
func (r *UseIsNullOrEmptyRule) Confidence() float64 { return 0.75 }

// UseOrEmptyRule detects `x ?: emptyList()` and similar patterns that can use .orEmpty().
// Handles: emptyList/Set/Map/Array/Sequence(), listOf/setOf/mapOf/sequenceOf/arrayOf() with
// no arguments, and empty string literals ("" / """""").
// Uses tree-sitter DispatchRule on elvis_expression for structural accuracy.
type UseOrEmptyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseOrEmptyRule) Confidence() float64 { return 0.75 }

// useOrEmptyFunctions maps callee names that represent empty collections/sequences.
var useOrEmptyFunctions = map[string]bool{
	"emptyList":     true,
	"emptySet":      true,
	"emptyMap":      true,
	"emptyArray":    true,
	"emptySequence": true,
}

// useOrEmptyFactoryFunctions maps zero-arg factory calls that produce empty collections.
var useOrEmptyFactoryFunctions = map[string]bool{
	"listOf":     true,
	"setOf":      true,
	"mapOf":      true,
	"arrayOf":    true,
	"sequenceOf": true,
}

func lhsNeedsParensFlat(file *scanner.File, idx uint32) bool {
	switch file.FlatType(idx) {
	case "simple_identifier", "navigation_expression", "call_expression",
		"indexing_expression", "parenthesized_expression":
		return false
	default:
		return true
	}
}

// UseAnyOrNoneInsteadOfFindRule detects `.find {} != null` and `.find {} == null`
// (and also firstOrNull / lastOrNull variants).
// Uses AST dispatch on equality_expression for precise detection.
type UseAnyOrNoneInsteadOfFindRule struct {
	FlatDispatchBase
	BaseRule
}

// anyOrNoneFindFuncs lists the function names that can be replaced.
var anyOrNoneFindFuncs = map[string]bool{
	"find":        true,
	"firstOrNull": true,
	"lastOrNull":  true,
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseAnyOrNoneInsteadOfFindRule) Confidence() float64 { return 0.75 }

// UseEmptyCounterpartRule detects `listOf()` etc. with no arguments.
// Uses AST dispatch on call_expression for precise detection, matching
// listOf(), setOf(), mapOf(), arrayOf(), sequenceOf(), and listOfNotNull()
// with zero arguments, and suggesting emptyList(), emptySet(), etc.
type UseEmptyCounterpartRule struct {
	FlatDispatchBase
	BaseRule
}

var emptyCounterparts = map[string]string{
	"listOf":        "emptyList",
	"listOfNotNull": "emptyList",
	"setOf":         "emptySet",
	"mapOf":         "emptyMap",
	"arrayOf":       "emptyArray",
	"sequenceOf":    "emptySequence",
}

// Confidence reports a tier-2 (medium) base confidence. Style/idiomatic rule. Detection pattern-matches the anti-idiom (if/throw
// blocks, null checks, explicit loops) but whether the suggested
// replacement is actually clearer is context-dependent. Classified per
// roadmap/17.
func (r *UseEmptyCounterpartRule) Confidence() float64 { return 0.75 }
