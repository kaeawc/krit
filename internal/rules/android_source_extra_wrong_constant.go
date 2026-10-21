package rules

import (
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/analyzers/nullflow"
	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type WrongConstantRule struct {
	FlatDispatchBase
	AndroidRule
}

type wrongConstantEntry struct {
	Method      string
	Description string
	Values      map[int64]bool
	Flag        bool
}

var wrongConstantEntries = []wrongConstantEntry{
	{Method: "setVisibility", Description: "setVisibility() should use View.VISIBLE, View.INVISIBLE, or View.GONE constants.", Values: int64Set(0, 4, 8)},
	{Method: "setLayoutDirection", Description: "setLayoutDirection() should use View.LAYOUT_DIRECTION_* constants.", Values: int64Set(0, 1, 2, 3)},
	{Method: "setImportantForAccessibility", Description: "setImportantForAccessibility() should use View.IMPORTANT_FOR_ACCESSIBILITY_* constants.", Values: int64Set(0, 1, 2, 4)},
	{Method: "setGravity", Description: "setGravity() should use Gravity.* constants.", Values: int64Set(0, 1, 2, 3, 4, 5, 7, 8, 16, 17, 48, 80, 112, 119, 128, 8388611, 8388613), Flag: true},
	{Method: "setOrientation", Description: "setOrientation() should use LinearLayout.HORIZONTAL or LinearLayout.VERTICAL.", Values: int64Set(0, 1)},
}

func int64Set(values ...int64) map[int64]bool {
	out := make(map[int64]bool, len(values))
	for _, value := range values {
		out[value] = true
	}
	return out
}

func (r *WrongConstantRule) NodeTypes() []string { return []string{"call_expression"} }

// Confidence is medium-high: findings require a structural call plus either a
// resolved Android framework target or same-file constant-set annotation.
func (r *WrongConstantRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *WrongConstantRule) check(ctx *api.Context) {
	if ctx.File.FlatType(ctx.Idx) != "call_expression" {
		return
	}
	file := ctx.File
	method := flatCallExpressionName(file, ctx.Idx)
	entry, ok := wrongConstantEntryForMethod(method)
	if !ok {
		return
	}
	arg := wrongConstantFirstValueArgument(file, ctx.Idx)
	if arg == 0 {
		return
	}
	expr := wrongConstantValueArgumentExpression(file, arg)
	if expr == 0 {
		return
	}
	allowed, anchored := wrongConstantAllowedConstants(ctx, ctx.Idx, entry)
	if !anchored || len(allowed.Values) == 0 {
		return
	}
	if literal := wrongConstantFirstIntegerLiteral(file, expr); literal != 0 {
		ctx.EmitAt(file.FlatRow(literal)+1, file.FlatCol(literal)+1, entry.Description)
		return
	}
	value, ok := wrongConstantEvalIntExpr(file, expr, wrongConstantFileConstants(file))
	if !ok {
		return
	}
	if !allowed.ValueAllowed(value) {
		ctx.EmitAt(file.FlatRow(expr)+1, file.FlatCol(expr)+1, entry.Description)
	}
}

type wrongConstantAllowedSet struct {
	Values map[int64]bool
	Flag   bool
}

func (s wrongConstantAllowedSet) ValueAllowed(value int64) bool {
	if s.Values[value] {
		return true
	}
	if !s.Flag {
		return false
	}
	var allowedBits int64
	for v := range s.Values {
		allowedBits |= v
	}
	return value&^allowedBits == 0
}

func wrongConstantEntryForMethod(method string) (wrongConstantEntry, bool) {
	for _, entry := range wrongConstantEntries {
		if entry.Method == method {
			return entry, true
		}
	}
	return wrongConstantEntry{}, false
}

func wrongConstantFirstValueArgument(file *scanner.File, call uint32) uint32 {
	args := flatCallKeyArguments(file, call)
	if args == 0 {
		return 0
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" {
			return arg
		}
	}
	return 0
}

func wrongConstantValueArgumentExpression(file *scanner.File, arg uint32) uint32 {
	return flatLastNamedChild(file, arg)
}

func wrongConstantAllowedConstants(ctx *api.Context, call uint32, entry wrongConstantEntry) (wrongConstantAllowedSet, bool) {
	if target := wrongConstantOracleCallTarget(ctx, call); target != "" {
		if wrongConstantFrameworkTargetMatches(target, entry.Method) {
			return wrongConstantAllowedSet{Values: entry.Values, Flag: entry.Flag}, true
		}
	}
	if set, ok := wrongConstantSameFileAllowedConstants(ctx, call, entry.Method); ok {
		return set, true
	}
	return wrongConstantAllowedSet{}, false
}

func wrongConstantOracleCallTarget(ctx *api.Context, idx uint32) string {
	if ctx.Resolver == nil || ctx.File == nil {
		return ""
	}
	var lookup oracle.Lookup
	if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
		lookup = cr.Oracle()
	}
	if lookup == nil {
		return ""
	}
	return oracleLookupCallTargetFlat(lookup, ctx.File, idx)
}

func wrongConstantFrameworkTargetMatches(target, method string) bool {
	if target == "" || method == "" {
		return false
	}
	if !strings.HasSuffix(target, "."+method) && !strings.HasSuffix(target, "#"+method) {
		return false
	}
	switch method {
	case "setVisibility", "setLayoutDirection", "setImportantForAccessibility":
		return strings.Contains(target, "android.view.View.")
	case "setOrientation":
		return strings.Contains(target, "android.widget.LinearLayout.") ||
			strings.Contains(target, "androidx.recyclerview.widget.LinearLayoutManager.")
	case "setGravity":
		return strings.HasPrefix(target, "android.") || strings.HasPrefix(target, "androidx.")
	default:
		return false
	}
}

func wrongConstantSameFileAllowedConstants(ctx *api.Context, call uint32, method string) (wrongConstantAllowedSet, bool) {
	file := ctx.File
	receiver := wrongConstantReceiverNode(file, call)
	if receiver == 0 {
		return wrongConstantAllowedSet{}, false
	}
	className := wrongConstantReceiverClassName(ctx, receiver)
	if className == "" {
		return wrongConstantAllowedSet{}, false
	}
	classNode := flatFindSameFileClassLikeDeclaration(file, className)
	if classNode == 0 {
		return wrongConstantAllowedSet{}, false
	}
	fn := wrongConstantClassFunction(file, classNode, method)
	if fn == 0 {
		return wrongConstantAllowedSet{}, false
	}
	_, mods := wrongConstantFunctionParameter(file, fn, 0)
	if mods == 0 {
		return wrongConstantAllowedSet{}, false
	}
	return wrongConstantAllowedSetFromModifiers(file, mods, classNode)
}

func wrongConstantReceiverNode(file *scanner.File, call uint32) uint32 {
	nav, _ := flatCallExpressionParts(file, call)
	if nav == 0 || file.FlatNamedChildCount(nav) == 0 {
		return 0
	}
	return file.FlatNamedChild(nav, 0)
}

func wrongConstantReceiverClassName(ctx *api.Context, receiver uint32) string {
	file := ctx.File
	if ctx.Resolver != nil {
		if typ := ctx.Resolver.ResolveFlatNode(receiver, file); typ != nil && typ.Name != "" {
			return typ.Name
		}
	}
	if file.FlatType(receiver) != "simple_identifier" {
		return ""
	}
	decl := nullflow.ResolveSimpleReferenceDeclaration(file, receiver)
	if decl == 0 {
		return ""
	}
	return wrongConstantDeclaredTypeName(file, decl)
}

func wrongConstantDeclaredTypeName(file *scanner.File, decl uint32) string {
	var typeNode uint32
	file.FlatWalkAllNodes(decl, func(candidate uint32) {
		if typeNode != 0 || file.FlatType(candidate) != "user_type" {
			return
		}
		typeNode = candidate
	})
	if typeNode == 0 {
		return ""
	}
	return wrongConstantTypeName(file, typeNode)
}

func wrongConstantClassFunction(file *scanner.File, classNode uint32, method string) uint32 {
	var found uint32
	file.FlatWalkAllNodes(classNode, func(candidate uint32) {
		if found != 0 || file.FlatType(candidate) != "function_declaration" || extractIdentifierFlat(file, candidate) != method {
			return
		}
		owner, ok := flatEnclosingAncestor(file, candidate, "class_declaration", "object_declaration")
		if ok && owner == classNode {
			found = candidate
		}
	})
	return found
}

func wrongConstantFunctionParameter(file *scanner.File, fn uint32, index int) (uint32, uint32) {
	params, _ := file.FlatFindChild(fn, "function_value_parameters")
	if params == 0 {
		return 0, 0
	}
	seen := 0
	var pendingMods uint32
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "parameter_modifiers":
			pendingMods = child
		case "parameter":
			if seen == index {
				return child, pendingMods
			}
			seen++
			pendingMods = 0
		}
	}
	return 0, 0
}

func wrongConstantAllowedSetFromModifiers(file *scanner.File, mods, classNode uint32) (wrongConstantAllowedSet, bool) {
	var found wrongConstantAllowedSet
	ok := false
	file.FlatWalkAllNodes(mods, func(ann uint32) {
		if ok || file.FlatType(ann) != "annotation" {
			return
		}
		if set, setOK := wrongConstantAllowedSetFromAnnotation(file, ann, classNode); setOK {
			found = set
			ok = true
		}
	})
	return found, ok
}

func wrongConstantAllowedSetFromAnnotation(file *scanner.File, ann, classNode uint32) (wrongConstantAllowedSet, bool) {
	name := wrongConstantAnnotationName(file, ann)
	if name == "" {
		return wrongConstantAllowedSet{}, false
	}
	if name == "IntDef" || name == "LongDef" {
		values := wrongConstantAnnotationValues(file, ann, classNode)
		return wrongConstantAllowedSet{Values: values, Flag: wrongConstantAnnotationFlag(file, ann)}, len(values) > 0
	}
	annotationClass := flatFindSameFileClassLikeDeclaration(file, name)
	if annotationClass == 0 || !file.FlatHasModifier(annotationClass, "annotation") {
		return wrongConstantAllowedSet{}, false
	}
	if mods, ok := file.FlatFindChild(annotationClass, "modifiers"); ok {
		return wrongConstantAllowedSetFromModifiers(file, mods, classNode)
	}
	return wrongConstantAllowedSet{}, false
}

func wrongConstantAnnotationName(file *scanner.File, ann uint32) string {
	var name string
	file.FlatWalkAllNodes(ann, func(candidate uint32) {
		if name != "" || file.FlatType(candidate) != "user_type" {
			return
		}
		name = wrongConstantTypeName(file, candidate)
	})
	return name
}

func wrongConstantTypeName(file *scanner.File, idx uint32) string {
	var last string
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		switch file.FlatType(candidate) {
		case "simple_identifier", "type_identifier":
			last = file.FlatNodeText(candidate)
		}
	})
	return last
}

func wrongConstantAnnotationValues(file *scanner.File, ann, classNode uint32) map[int64]bool {
	args := wrongConstantAnnotationValueArguments(file, ann)
	if args == 0 {
		return nil
	}
	consts := wrongConstantFileConstants(file)
	out := map[int64]bool{}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		label := flatValueArgumentLabel(file, arg)
		if label != "" && label != "value" {
			continue
		}
		expr := wrongConstantValueArgumentExpression(file, arg)
		if expr == 0 {
			continue
		}
		if value, ok := wrongConstantEvalIntExpr(file, expr, consts); ok {
			out[value] = true
			continue
		}
		if file.FlatType(expr) == "simple_identifier" && classNode != 0 {
			if value, ok := consts[extractIdentifierFlat(file, classNode)+"."+file.FlatNodeText(expr)]; ok {
				out[value] = true
			}
		}
	}
	return out
}

func wrongConstantAnnotationFlag(file *scanner.File, ann uint32) bool {
	args := wrongConstantAnnotationValueArguments(file, ann)
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" || flatValueArgumentLabel(file, arg) != "flag" {
			continue
		}
		expr := wrongConstantValueArgumentExpression(file, arg)
		return expr != 0 && file.FlatNodeText(expr) == "true"
	}
	return false
}

func wrongConstantAnnotationValueArguments(file *scanner.File, ann uint32) uint32 {
	var args uint32
	file.FlatWalkAllNodes(ann, func(candidate uint32) {
		if args == 0 && file.FlatType(candidate) == "value_arguments" {
			args = candidate
		}
	})
	return args
}

func wrongConstantFileConstants(file *scanner.File) map[string]int64 {
	consts := map[string]int64{}
	file.FlatWalkAllNodes(0, func(candidate uint32) {
		if file.FlatType(candidate) != "property_declaration" || !file.FlatHasModifier(candidate, "const") {
			return
		}
		name := extractIdentifierFlat(file, candidate)
		if name == "" {
			return
		}
		expr := flatLastNamedChild(file, candidate)
		if expr == 0 || expr == candidate {
			return
		}
		value, ok := wrongConstantEvalIntExpr(file, expr, consts)
		if !ok {
			return
		}
		consts[name] = value
		if owner := wrongConstantConstantOwnerName(file, candidate); owner != "" {
			consts[owner+"."+name] = value
		}
	})
	return consts
}

func wrongConstantConstantOwnerName(file *scanner.File, idx uint32) string {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration", "object_declaration":
			if name := extractIdentifierFlat(file, cur); name != "" {
				return name
			}
		}
	}
	return ""
}

func wrongConstantEvalIntExpr(file *scanner.File, idx uint32, consts map[string]int64) (int64, bool) {
	idx = flatUnwrapParenExpr(file, idx)
	switch file.FlatType(idx) {
	case "integer_literal":
		return parseKotlinIntegerLiteral(file.FlatNodeText(idx))
	case "prefix_expression":
		lit := wrongConstantFirstIntegerLiteral(file, idx)
		if lit == 0 {
			return 0, false
		}
		value, ok := parseKotlinIntegerLiteral(file.FlatNodeText(lit))
		if !ok {
			return 0, false
		}
		if strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(idx)), "-") {
			return -value, true
		}
		return value, true
	case "simple_identifier":
		value, ok := consts[file.FlatNodeText(idx)]
		return value, ok
	case "navigation_expression":
		path, ok := nullflow.FlatReferencePathFromExpr(file, idx)
		if !ok {
			return 0, false
		}
		value, ok := consts[strings.Join(path.Parts, ".")]
		return value, ok
	case "infix_expression", "additive_expression", "multiplicative_expression":
		return wrongConstantEvalInfixExpr(file, idx, consts)
	default:
		return 0, false
	}
}

func wrongConstantEvalInfixExpr(file *scanner.File, idx uint32, consts map[string]int64) (int64, bool) {
	var values []int64
	var op string
	for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
		if !file.FlatIsNamed(child) {
			continue
		}
		if file.FlatType(child) == "simple_identifier" {
			text := file.FlatNodeText(child)
			if text == "or" || text == "and" || text == "shl" || text == "shr" {
				op = text
				continue
			}
		}
		value, ok := wrongConstantEvalIntExpr(file, child, consts)
		if !ok {
			return 0, false
		}
		values = append(values, value)
	}
	if len(values) != 2 || op == "" {
		return 0, false
	}
	switch op {
	case "or":
		return values[0] | values[1], true
	case "and":
		return values[0] & values[1], true
	case "shl":
		return values[0] << values[1], true
	case "shr":
		return values[0] >> values[1], true
	default:
		return 0, false
	}
}

func wrongConstantFirstIntegerLiteral(file *scanner.File, idx uint32) uint32 {
	var found uint32
	file.FlatWalkAllNodes(idx, func(candidate uint32) {
		if found == 0 && file.FlatType(candidate) == "integer_literal" {
			found = candidate
		}
	})
	return found
}

func parseKotlinIntegerLiteral(text string) (int64, bool) {
	text = strings.TrimSpace(strings.ReplaceAll(text, "_", ""))
	text = strings.TrimSuffix(strings.TrimSuffix(text, "L"), "l")
	if text == "" {
		return 0, false
	}
	value, err := strconv.ParseInt(text, 0, 64)
	if err != nil {
		return 0, false
	}
	return value, true
}
