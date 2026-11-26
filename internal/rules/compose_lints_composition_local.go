package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Messages of compose-lint-checks 1.6.1's CompositionLocalUsageDetector, as Lint reports them with the
// explanation's paragraphs joined by a space.
const (
	composeCompositionLocalUsageMessage = "`CompositionLocal`s are implicit dependencies and creating new ones should be " +
		"avoided. See https://slackhq.github.io/compose-lints/rules/#compositionlocals for more information."
	composeCompositionLocalGetterMessage = "`CompositionLocal`s should be singletons and not use getters. Otherwise a new " +
		"instance will be returned every call."
)

// composeLintsCompositionLocalFactories are the callees compose-lints treats as declaring a CompositionLocal.
var composeLintsCompositionLocalFactories = []string{"staticCompositionLocalOf", "compositionLocalOf"}

// ComposeCompositionLocalUsageRule ports compose-lints' ComposeCompositionLocalUsage: a `val` property (top-level or
// member) initialized with compositionLocalOf/staticCompositionLocalOf declares a new CompositionLocal, which is an
// implicit dependency, unless its name is allowlisted.
type ComposeCompositionLocalUsageRule struct {
	FlatDispatchBase
	BaseRule
	// AllowedCompositionLocals lists property names that may declare a CompositionLocal (Lint's
	// `allowed-composition-locals` option).
	AllowedCompositionLocals []string
}

// ComposeCompositionLocalGetterRule ports compose-lints' ComposeCompositionLocalGetter: a property getter that
// returns compositionLocalOf/staticCompositionLocalOf creates a new CompositionLocal on every read.
type ComposeCompositionLocalGetterRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *ComposeCompositionLocalUsageRule) check(ctx *api.Context) {
	file, prop := ctx.File, ctx.Idx
	if acrIsLocalDeclaration(file, prop) || !composeLintsPropertyIsVal(file, prop) {
		return
	}
	init := composeLintsPropertyInitializer(file, prop)
	if init == 0 || !composeLintsIsCompositionLocalFactoryCall(file, composeLintsUnwrapParens(file, init)) {
		return
	}
	varDecl, _ := file.FlatFindChild(prop, "variable_declaration")
	name := strings.Trim(file.FlatChildTextOrEmpty(varDecl, "simple_identifier"), "`")
	for _, allowed := range r.AllowedCompositionLocals {
		if name == allowed {
			return
		}
	}
	line, col := composeLintsDeclarationStart(file, prop)
	ctx.EmitAt(line, col, composeCompositionLocalUsageMessage)
}

func (r *ComposeCompositionLocalGetterRule) check(ctx *api.Context) {
	file, getter := ctx.File, ctx.Idx
	body, _ := file.FlatFindChild(getter, "function_body")
	if body == 0 {
		return
	}
	var expr uint32
	if first := file.FlatFirstChild(body); first != 0 && file.FlatType(first) == "=" {
		for next := file.FlatNextSib(first); next != 0; next = file.FlatNextSib(next) {
			if file.FlatIsNamed(next) && !composeLintsIsComment(file, next) {
				expr = next
				break
			}
		}
	} else if stmts, ok := file.FlatFindChild(body, "statements"); ok {
		// Lint looks at the first statement of a block body only.
		expr = file.FlatFirstChild(stmts)
		if expr != 0 && file.FlatType(expr) == "jump_expression" {
			if kw := file.FlatFirstChild(expr); kw == 0 || !file.FlatNodeTextEquals(kw, "return") {
				return
			}
			expr = flatLastNamedChild(file, expr)
		}
	}
	if expr == 0 || !composeLintsIsCompositionLocalFactoryCall(file, composeLintsUnwrapParens(file, expr)) {
		return
	}
	// Annotations on the getter (`@Suppress(...) get() = ...`) come before the keyword.
	if kw, ok := file.FlatFindChild(getter, "get"); ok {
		ctx.EmitAtNode(kw, composeCompositionLocalGetterMessage)
	}
}

// composeLintsIsCompositionLocalFactoryCall reports whether node is a call such as `compositionLocalOf { }` whose
// simple callee resolves to the androidx.compose.runtime factory. Lint matches the callee name alone; krit also
// requires the name to resolve, so same-named local functions do not count.
func composeLintsIsCompositionLocalFactoryCall(file *scanner.File, node uint32) bool {
	if node == 0 || file.FlatType(node) != "call_expression" {
		return false
	}
	callee := file.FlatFirstChild(node)
	if callee == 0 || file.FlatType(callee) != "simple_identifier" {
		return false
	}
	name := file.FlatNodeText(callee)
	for _, factory := range composeLintsCompositionLocalFactories {
		if name == factory {
			return acrCalleeResolvesTo(file, node, acrRuntimePackage, name)
		}
	}
	return false
}

// composeLintsPropertyIsVal reports whether a property declaration uses `val`.
func composeLintsPropertyIsVal(file *scanner.File, prop uint32) bool {
	kind, ok := file.FlatFindChild(prop, "binding_pattern_kind")
	return ok && strings.TrimSpace(file.FlatNodeText(kind)) == "val"
}

// composeLintsPropertyInitializer returns the `= expr` initializer of a property declaration, or 0 for delegated
// properties and properties without an initializer.
func composeLintsPropertyInitializer(file *scanner.File, prop uint32) uint32 {
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "=":
			for next := file.FlatNextSib(child); next != 0; next = file.FlatNextSib(next) {
				if file.FlatIsNamed(next) && !composeLintsIsComment(file, next) {
					return next
				}
			}
			return 0
		case "property_delegate", "getter", "setter":
			return 0
		}
	}
	return 0
}

// composeLintsUnwrapParens strips any parentheses around an expression.
func composeLintsUnwrapParens(file *scanner.File, node uint32) uint32 {
	for node != 0 && file.FlatType(node) == "parenthesized_expression" {
		inner := uint32(0)
		for child := file.FlatFirstChild(node); child != 0; child = file.FlatNextSib(child) {
			if file.FlatIsNamed(child) && !composeLintsIsComment(file, child) {
				inner = child
				break
			}
		}
		node = inner
	}
	return node
}
