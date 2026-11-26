package rules

import (
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Port of ConstraintLayoutDslDetector from androidx.constraintlayout:constraintlayout-compose-android 1.1.2.
//
// Like the detector, the checks compare source text (`parent.start`, `Dimension.matchParent`). In UAST the last
// statement of a lambda is an implicit return, which the detector's statement scans skip; the port skips it too.

const axlConstraintLayoutPackage = "androidx.constraintlayout.compose"

// IncorrectReferencesDeclarationRule ports IncorrectReferencesDeclaration: `val (a, b) = createRefsFor(...)` whose
// argument count differs from the destructured variables.
type IncorrectReferencesDeclarationRule struct {
	FlatDispatchBase
	BaseRule
}

// IncorrectMatchParentUsageRule ports IncorrectMatchParentUsage: Dimension.matchParent next to constraints it will
// override.
type IncorrectMatchParentUsageRule struct {
	FlatDispatchBase
	BaseRule
}

// IncorrectChainMarginsUsageRule ports IncorrectChainMarginsUsage: chain-like linkTo margins on chain elements, which
// chains ignore; use withChainParams.
type IncorrectChainMarginsUsageRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *IncorrectReferencesDeclarationRule) fileMentions() []string {
	return []string{axlConstraintLayoutPackage}
}
func (r *IncorrectMatchParentUsageRule) fileMentions() []string {
	return []string{axlConstraintLayoutPackage, "matchParent"}
}
func (r *IncorrectChainMarginsUsageRule) fileMentions() []string {
	return []string{axlConstraintLayoutPackage, "Chain("}
}

func (r *IncorrectReferencesDeclarationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "IncorrectReferencesDeclaration", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"createRefsFor must be called without an explicit receiver inside a ConstraintSet or MotionScene lambda, " +
				"and every argument must be a string literal or template, as Lint requires String arguments.",
		}}
}

func (r *IncorrectMatchParentUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "IncorrectMatchParentUsage", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"The assignment must sit directly in a constrain(ref) { } lambda. Lint's quick fix is not ported.",
		}}
}

func (r *IncorrectChainMarginsUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "IncorrectChainMarginsUsage", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"Chain elements must be references created by createRefs/createRefFor/createRefsFor in the same block. " +
				"Lint's quick fix (adding withChainParams and removing the linkTo calls) is not ported.",
		}}
}

// axlCLScopeCall reports whether call is an unqualified call named name inside a ConstraintSet { } / MotionScene { }
// / ConstraintLayout(...) { } lambda in a file that uses androidx.constraintlayout.compose.
func axlCLScopeCall(file *scanner.File, call uint32, name string) bool {
	callee, _, hasReceiver := acrCallCallee(file, call)
	if callee != name || hasReceiver || len(acrFunctionDecls(file, name)) > 0 {
		return false
	}
	return acuiImportsPackage(file, axlConstraintLayoutPackage)
}

// check mirrors detectCreateRefsForUsage on `val (a, b) = createRefsFor(...)`.
func (r *IncorrectReferencesDeclarationRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if acrIsInnerCallOfTrailingLambda(file, node) || !axlCLScopeCall(file, node, "createRefsFor") {
		return
	}
	parent, ok := file.FlatParent(node)
	if !ok || file.FlatType(parent) != "property_declaration" {
		return
	}
	multi, ok := file.FlatFindChild(parent, "multi_variable_declaration")
	if !ok {
		return
	}
	vars := len(flatDirectChildrenOfType(file, multi, "variable_declaration"))
	call, ok := lintCallAt(file, node)
	if !ok || len(call.Args) == vars {
		return
	}
	for _, arg := range call.Args {
		if arg == 0 || file.FlatType(arg) != "string_literal" {
			return // a spread array or a non-String id cannot be counted
		}
	}
	_, nameNode, _ := acrCallCallee(file, node)
	ctx.EmitAtNode(nameNode, "Arguments of `createRefsFor` ("+strconv.Itoa(len(call.Args))+") do not match assigned variables ("+
		strconv.Itoa(vars)+")")
}

var (
	axlHorizontalAnchors = map[string]bool{"start": true, "end": true, "absoluteLeft": true, "absoluteRight": true}
	axlVerticalAnchors   = map[string]bool{"top": true, "bottom": true}
)

// axlLambdaStatements returns the statements of a lambda's body that UAST sees as expressions: every statement but
// the last, which is an implicit return.
func axlLambdaStatements(file *scanner.File, lambda uint32) []uint32 {
	stmts, ok := file.FlatFindChild(lambda, "statements")
	if !ok {
		return nil
	}
	var out []uint32
	for c := file.FlatFirstChild(stmts); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) {
			out = append(out, c)
		}
	}
	if len(out) > 0 {
		out = out[:len(out)-1]
	}
	return out
}

// axlConstrainLambdaOf returns the lambda of the constrain(ref) { } call whose body directly holds node.
func axlConstrainLambdaOf(file *scanner.File, node uint32) uint32 {
	stmts, ok := file.FlatParent(node)
	if !ok || file.FlatType(stmts) != "statements" {
		return 0
	}
	lambda, ok := file.FlatParent(stmts)
	if !ok || file.FlatType(lambda) != "lambda_literal" {
		return 0
	}
	arg, ok := acrLambdaArgumentOf(file, lambda)
	if !ok || !arg.Trailing || !axlCLScopeCall(file, arg.Call, "constrain") {
		return 0
	}
	return lambda
}

// check mirrors detectMatchParentUsage on `width = Dimension.matchParent` / `height = ...` in a constrain block that
// also has a linkTo or center call to something other than the matching parent anchor.
func (r *IncorrectMatchParentUsageRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	target := file.FlatFirstChild(node)
	value := flatLastNamedChild(file, node)
	if target == 0 || value == 0 || strings.Join(strings.Fields(file.FlatNodeText(value)), "") != "Dimension.matchParent" {
		return
	}
	horizontal := false
	switch strings.TrimSpace(file.FlatNodeText(target)) {
	case "width":
		horizontal = true
	case "height":
	default:
		return
	}
	lambda := axlConstrainLambdaOf(file, node)
	if lambda == 0 {
		return
	}
	errorProne := false
	for _, stmt := range axlLambdaStatements(file, lambda) {
		if axlMatchParentErrorProne(file, stmt, horizontal) {
			errorProne = true
			break
		}
	}
	if !errorProne {
		return
	}
	override := "centerVerticallyTo"
	if horizontal {
		override = "centerHorizontallyTo"
	}
	ctx.EmitAtNode(value, "`Dimension.matchParent` will override constraints to an equivalent of `"+override+
		"(parent)`.\nUse `Dimension.percent(1f)` to respect constraints.")
}

// axlMatchParentErrorProne mirrors EvaluateableExpression: `anchor.linkTo(x)` with x other than parent.anchor, or a
// center call with an argument other than parent, for the dimension's direction.
func axlMatchParentErrorProne(file *scanner.File, stmt uint32, horizontal bool) bool {
	if file.FlatType(stmt) != "call_expression" {
		return false
	}
	name, _, hasReceiver := acrCallCallee(file, stmt)
	call, ok := lintCallAt(file, stmt)
	if !ok || len(call.Args) == 0 {
		return false
	}
	argText := strings.Join(strings.Fields(file.FlatNodeText(call.Args[0])), "")
	if hasReceiver {
		if name != "linkTo" {
			return false
		}
		anchor := strings.TrimSpace(call.ReceiverText)
		if (horizontal && !axlHorizontalAnchors[anchor]) || (!horizontal && !axlVerticalAnchors[anchor]) {
			return false
		}
		return argText != "parent."+anchor
	}
	switch {
	case name == "centerTo", horizontal && name == "centerHorizontallyTo", !horizontal && name == "centerVerticallyTo":
		return argText != "parent"
	}
	return false
}

// axlOppositeAnchor mirrors getOppositeAnchorName.
func axlOppositeAnchor(anchor string) string {
	switch anchor {
	case "start":
		return "end"
	case "end":
		return "start"
	case "absoluteLeft":
		return "absoluteRight"
	case "absoluteRight":
		return "absoluteLeft"
	case "top":
		return "bottom"
	case "bottom":
		return "top"
	}
	return "start"
}

// check mirrors detectChainParamsUsage on createHorizontalChain(...) / createVerticalChain(...): each element without
// withChainParams whose constrain block links it, with a margin, to its neighbour (or the parent at the ends) along
// the chain's axis is reported at the element.
func (r *IncorrectChainMarginsUsageRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if acrIsInnerCallOfTrailingLambda(file, node) {
		return
	}
	name, _, _ := acrCallCallee(file, node)
	horizontal := name == "createHorizontalChain"
	if (!horizontal && name != "createVerticalChain") || !axlCLScopeCall(file, node, name) {
		return
	}
	stmts, ok := file.FlatParent(node)
	if !ok || file.FlatType(stmts) != "statements" {
		return
	}
	call, ok := lintCallAt(file, node)
	if !ok {
		return
	}
	type chainNode struct {
		ident      uint32
		name       string
		chainParam bool
	}
	var nodes []chainNode
	for _, arg := range call.Args {
		ident, qualified := axlChainElement(file, arg, stmts)
		if ident == 0 {
			continue
		}
		name := file.FlatNodeText(ident)
		nodes = append(nodes, chainNode{ident: ident, name: name,
			chainParam: qualified || axlWithChainParamsCalled(file, stmts, name)})
	}
	anchors := axlVerticalAnchors
	if horizontal {
		anchors = axlHorizontalAnchors
	}
	block := axlBlockStatements(file, stmts)
	for i, n := range nodes {
		if n.chainParam {
			continue
		}
		prev, next := "", ""
		if i > 0 {
			prev = nodes[i-1].name
		}
		if i+1 < len(nodes) {
			next = nodes[i+1].name
		}
		if axlHasChainLikeMargin(file, block, n.name, prev, next, anchors) {
			ctx.EmitAtNode(n.ident, "Margins for elements in a Chain should be applied with "+
				"`LayoutReference.withChainParams(...)`.")
		}
	}
}

// axlBlockStatements returns the named statements of a statements node, minus the last one when the block is a
// lambda body (an implicit return in UAST).
func axlBlockStatements(file *scanner.File, stmts uint32) []uint32 {
	var out []uint32
	for c := file.FlatFirstChild(stmts); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) {
			out = append(out, c)
		}
	}
	if p, ok := file.FlatParent(stmts); ok && file.FlatType(p) == "lambda_literal" && len(out) > 0 {
		out = out[:len(out)-1]
	}
	return out
}

// axlChainElement returns the reference identifier of a chain argument (`a` or `a.withChainParams(...)`) when it is
// a layout reference declared in the block, and whether it is qualified.
func axlChainElement(file *scanner.File, arg, stmts uint32) (uint32, bool) {
	arg = acrUnwrapParens(file, arg)
	qualified := false
	ident := arg
	if file.FlatType(arg) != "simple_identifier" {
		first := arg
		for first != 0 && file.FlatType(first) != "simple_identifier" {
			first = file.FlatFirstChild(first)
		}
		if first == 0 {
			return 0, false
		}
		ident, qualified = first, true
	}
	if !axlIsLayoutReference(file, stmts, file.FlatNodeText(ident)) {
		return 0, false
	}
	return ident, qualified
}

// axlIsLayoutReference reports whether name is declared in the block (or an enclosing one) from createRefs,
// createRefFor or createRefsFor.
func axlIsLayoutReference(file *scanner.File, stmts uint32, name string) bool {
	for s := stmts; s != 0; {
		for c := file.FlatFirstChild(s); c != 0; c = file.FlatNextSib(c) {
			if file.FlatType(c) != "property_declaration" {
				continue
			}
			declares := false
			if v, ok := file.FlatFindChild(c, "variable_declaration"); ok && file.FlatChildTextOrEmpty(v, "simple_identifier") == name {
				declares = true
			}
			if m, ok := file.FlatFindChild(c, "multi_variable_declaration"); ok {
				for _, v := range flatDirectChildrenOfType(file, m, "variable_declaration") {
					if file.FlatChildTextOrEmpty(v, "simple_identifier") == name {
						declares = true
					}
				}
			}
			if !declares {
				continue
			}
			init := flatLastNamedChild(file, c)
			if file.FlatType(init) != "call_expression" {
				return false
			}
			callee, _, _ := acrCallCallee(file, init)
			return callee == "createRefs" || callee == "createRefFor" || callee == "createRefsFor"
		}
		p, ok := file.FlatParent(s)
		for ok && file.FlatType(p) != "statements" {
			p, ok = file.FlatParent(p)
		}
		if !ok {
			return false
		}
		s = p
	}
	return false
}

// axlWithChainParamsCalled mirrors isChainParamsCalledInIdentifier: `name.withChainParams(...)` anywhere in the
// block.
func axlWithChainParamsCalled(file *scanner.File, stmts uint32, name string) bool {
	found := false
	file.FlatWalkNodes(stmts, "call_expression", func(c uint32) {
		if found {
			return
		}
		callee, _, hasReceiver := acrCallCallee(file, c)
		if callee != "withChainParams" || !hasReceiver {
			return
		}
		recv := file.FlatFirstChild(acrCalleeExpression(file, c))
		found = file.FlatType(recv) == "simple_identifier" && file.FlatNodeTextEquals(recv, name)
	})
	return found
}

// axlHasChainLikeMargin mirrors findChainLikeConstraints: a constrain(name) { } block (not the block's implicit-return
// statement) whose non-final statement `anchor.linkTo(target, margin...)` (two or more arguments) targets the
// neighbouring element's opposite anchor or the parent's same anchor.
func axlHasChainLikeMargin(file *scanner.File, block []uint32, name, prev, next string, anchors map[string]bool) bool {
	for _, stmt := range block {
		if file.FlatType(stmt) != "call_expression" || !axlCLScopeCall(file, stmt, "constrain") {
			continue
		}
		// constrain(a, b) { } parses as a call wrapping constrain(a, b); read its value arguments through it.
		targets := false
		for i := 0; ; i++ {
			a := acrCallValueArgument(file, stmt, i, "")
			if a == 0 {
				break
			}
			if strings.TrimSpace(file.FlatNodeText(a)) == name {
				targets = true
			}
		}
		if !targets {
			continue
		}
		lambda := axlTrailingLambda(file, stmt)
		if lambda == 0 {
			continue
		}
		for _, s := range axlLambdaStatements(file, lambda) {
			if file.FlatType(s) != "call_expression" {
				continue
			}
			callee, _, hasReceiver := acrCallCallee(file, s)
			link, ok := lintCallAt(file, s)
			if !hasReceiver || callee != "linkTo" || !ok || len(link.Args) < 2 {
				continue
			}
			anchor := strings.TrimSpace(link.ReceiverText)
			if !anchors[anchor] {
				continue
			}
			expectNext := "parent." + anchor
			if next != "" {
				expectNext = next + "." + axlOppositeAnchor(anchor)
			}
			expectPrev := "parent." + anchor
			if prev != "" {
				expectPrev = prev + "." + axlOppositeAnchor(anchor)
			}
			target := strings.Join(strings.Fields(file.FlatNodeText(link.Args[0])), "")
			if target == expectPrev || target == expectNext {
				return true
			}
		}
	}
	return false
}

// axlTrailingLambda returns the trailing lambda literal of a call, or 0.
func axlTrailingLambda(file *scanner.File, call uint32) uint32 {
	if suffix, ok := file.FlatFindChild(call, "call_suffix"); ok {
		if al, ok := file.FlatFindChild(suffix, "annotated_lambda"); ok {
			if l, ok := file.FlatFindChild(al, "lambda_literal"); ok {
				return l
			}
		}
	}
	return 0
}
