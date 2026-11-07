package rules

import (
	"bytes"
	"sort"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	rulesem "github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

// ExpressionBodySyntaxRule detects single-expression functions that could use = syntax.
type ExpressionBodySyntaxRule struct {
	FlatDispatchBase
	BaseRule
	IncludeLineWrapping bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *ExpressionBodySyntaxRule) Confidence() float64 { return api.ConfidenceMedium }

const expressionBodySyntaxMessage = "Functions with exact one statement, the return statement, can be rewritten with ExpressionBodySyntax."

// check reports a block-bodied function whose only statement is a return, as detekt does: at the return
// expression, skipping a wrapped return value unless IncludeLineWrapping is set and a return nested in an elvis.
func (r *ExpressionBodySyntaxRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if scanner.IsTestFile(file.Path) {
		return
	}
	body, _ := file.FlatFindChild(idx, "function_body")
	if body == 0 || file.FlatFirstChild(body) == 0 || file.FlatType(file.FlatFirstChild(body)) != "{" {
		return
	}
	stmts, ok := file.FlatFindChild(body, "statements")
	if !ok || stmts == 0 {
		return
	}
	only := uint32(0)
	for c := file.FlatFirstChild(stmts); c != 0; c = file.FlatNextSib(c) {
		if isFlatCommentNode(file, c) || !file.FlatIsNamed(c) {
			continue
		}
		if only != 0 {
			return
		}
		only = c
	}
	if only == 0 || file.FlatType(only) != "jump_expression" || !bytes.HasPrefix(file.FlatNodeBytes(only), returnPrefix) {
		return
	}
	if returnHasElvisReturn(file, only) {
		return
	}
	value := uint32(0)
	for c := file.FlatFirstChild(only); c != 0; c = file.FlatNextSib(c) {
		if file.FlatIsNamed(c) && !isFlatCommentNode(file, c) && file.FlatType(c) != "label" && file.FlatType(c) != "return" {
			value = c
		}
	}
	if value != 0 && !r.IncludeLineWrapping && strings.Contains(file.FlatNodeText(value), "\n") {
		return
	}
	f := r.Finding(file, file.FlatRow(only)+1, file.FlatCol(only)+1, expressionBodySyntaxMessage)
	if value != 0 && !strings.HasPrefix(file.FlatNodeText(only), "return@") {
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   int(file.FlatStartByte(body)),
			EndByte:     int(file.FlatEndByte(body)),
			Replacement: "= " + file.FlatNodeText(value),
		}
	}
	ctx.Emit(f)
}

// returnHasElvisReturn reports a return nested inside ret that is the right side of an elvis.
func returnHasElvisReturn(file *scanner.File, ret uint32) bool {
	found := false
	file.FlatWalkNodes(ret, "jump_expression", func(n uint32) {
		if n == ret || found || !bytes.HasPrefix(file.FlatNodeBytes(n), returnPrefix) {
			return
		}
		if p, ok := file.FlatParent(n); ok && file.FlatType(p) == "elvis_expression" {
			found = true
		}
	})
	return found
}

// ThrowsCountRule limits the number of throw statements in a function.
type ThrowsCountRule struct {
	FlatDispatchBase
	BaseRule
	Max                 int
	ExcludeGuardClauses bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *ThrowsCountRule) Confidence() float64 { return api.ConfidenceMedium }

func countJumpExpressionsFlat(root uint32, file *scanner.File, prefix string, limit int, accept func(uint32, string) bool) int {
	count := 0
	prefixBytes := returnPrefix
	if prefix == "throw" {
		prefixBytes = throwPrefix
	}
	var walk func(uint32) bool
	walk = func(node uint32) bool {
		if node == 0 {
			return false
		}
		if node != root {
			if isNestedCallableBodyFlat(file.FlatType(node)) {
				return false
			}
		}
		if file.FlatType(node) == "jump_expression" {
			textBytes := file.FlatNodeBytes(node)
			if bytes.HasPrefix(textBytes, prefixBytes) {
				if accept == nil {
					count++
					if count > limit {
						return true
					}
				} else {
					text := string(textBytes)
					if accept(node, text) {
						count++
						if count > limit {
							return true
						}
					}
				}
			}
		}
		for i := 0; i < file.FlatNamedChildCount(node); i++ {
			if walk(file.FlatNamedChild(node, i)) {
				return true
			}
		}
		return false
	}
	walk(root)
	return count
}

// CollapsibleIfStatementsRule detects nested if without else.
type CollapsibleIfStatementsRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *CollapsibleIfStatementsRule) Confidence() float64 { return api.ConfidenceMedium }

// SafeCastRule reports `if (x is T) x else null` (and the negated form), which `x as? T` replaces, as detekt does.
// With ReportRedundantCasts it also reports a cast repeated after an is-check, such as `if (x is Type) { x as Type }`.
// That extension is distinct from UnsafeCast, which is reserved for casts that can never succeed.
type SafeCastRule struct {
	FlatDispatchBase
	BaseRule
	// ReportRedundantCasts enables the krit-only redundant cast check; detekt has no such check.
	ReportRedundantCasts bool
}

// Confidence reports a tier-2 (medium) base confidence. The rule
// matches the `if (x is T) { x as T }` pattern on text, which is
// narrow enough to avoid most false positives but can stumble on
// multi-branch or shadowed variables. Roadmap/17 notes that this
// rule and UnsafeCast fire on overlapping locations; medium
// confidence is appropriate for the redundant-cast half of that
// pair.
func (r *SafeCastRule) Confidence() float64 { return api.ConfidenceMedium }

// safeCastSubjectIsUnstable reports a subject declared in scope as a var, a delegated property, or a
// property with a custom getter, none of which smart cast.
func safeCastSubjectIsUnstable(file *scanner.File, from uint32, name string) bool {
	unstable := func(prop uint32) bool {
		for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "var" || file.FlatNodeTextEquals(child, "var") {
				return true
			}
		}
		if file.FlatHasChildOfType(prop, "property_delegate") {
			return true
		}
		next, ok := file.FlatNextSibling(prop)
		return ok && file.FlatType(next) == "getter"
	}
	for scope, ok := file.FlatParent(from); ok; scope, ok = file.FlatParent(scope) {
		switch file.FlatType(scope) {
		case "function_declaration", "class_body", "source_file":
			result, found := false, false
			file.FlatWalkNodes(scope, "property_declaration", func(prop uint32) {
				if !found && extractIdentifierFlat(file, prop) == name {
					// Skip properties of nested classes or functions that merely share the name.
					if owner, ok := file.FlatParent(prop); ok && (file.FlatType(owner) == "statements" || file.FlatType(owner) == "class_body" || file.FlatType(owner) == "source_file") {
						found, result = true, unstable(prop)
					}
				}
			})
			if found {
				return result
			}
		}
	}
	// Parameters smart cast; a name declared nowhere in the file (an implicit receiver property such
	// as a state holder's `value`) cannot be proven stable.
	return !safeCastSubjectIsParameter(file, from, name)
}

// safeCastSubjectIsParameter reports whether name is a function, lambda, constructor or loop parameter in scope.
func safeCastSubjectIsParameter(file *scanner.File, from uint32, name string) bool {
	if name == "it" {
		return true
	}
	for scope, ok := file.FlatParent(from); ok; scope, ok = file.FlatParent(scope) {
		found := false
		visit := func(decl uint32) {
			if !found && extractIdentifierFlat(file, decl) == name {
				found = true
			}
		}
		switch file.FlatType(scope) {
		case "function_declaration", "anonymous_function", "secondary_constructor":
			file.FlatWalkNodes(scope, "function_value_parameters", func(params uint32) {
				for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) == "parameter" {
						visit(child)
					}
				}
			})
		case "lambda_literal":
			if params, ok := file.FlatFindChild(scope, "lambda_parameters"); ok {
				for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
					if file.FlatType(child) == "variable_declaration" {
						visit(child)
					}
				}
			}
		case "class_declaration":
			file.FlatWalkNodes(scope, "class_parameter", func(param uint32) {
				if !file.FlatHasModifier(param, "var") {
					visit(param)
				}
			})
		case "for_statement":
			if decl, ok := file.FlatFindChild(scope, "variable_declaration"); ok {
				visit(decl)
			}
		}
		if found {
			return true
		}
	}
	return false
}

// safeCastElseCastsToOtherType reports an else branch that casts the same subject to a different type.
func safeCastElseCastsToOtherType(file *scanner.File, ifIdx uint32, subject, subjectType string) bool {
	seenElse := false
	found := false
	for child := file.FlatFirstChild(ifIdx); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) == "else" {
			seenElse = true
			continue
		}
		if !seenElse || file.FlatType(child) != "control_structure_body" {
			continue
		}
		file.FlatWalkAllNodes(child, func(n uint32) {
			if file.FlatType(n) != "as_expression" {
				return
			}
			parts := strings.SplitN(file.FlatNodeText(n), " as ", 2)
			if len(parts) == 2 && strings.TrimSpace(parts[0]) == subject && strings.TrimSpace(strings.TrimPrefix(parts[1], "?")) != subjectType {
				found = true
			}
		})
	}
	return found
}

// frameworkAnnotationNames identifies annotations indicating external
// initialization by a framework — these vars can't be analyzed for mutability.
var frameworkAnnotationNames = map[string]bool{
	"Mock":               true, // Mockito
	"MockK":              true,
	"RelaxedMockK":       true,
	"SpyK":               true,
	"InjectMocks":        true,
	"Inject":             true, // Dagger/JSR-330
	"Autowired":          true, // Spring
	"BindView":           true, // ButterKnife
	"BindViews":          true,
	"Captor":             true, // Mockito
	"Spy":                true,
	"Value":              true, // Spring
	"EJB":                true,
	"Resource":           true,
	"PersistenceContext": true,
}

func hasFrameworkAnnotationFlat(file *scanner.File, idx uint32) bool {
	mods, _ := file.FlatFindChild(idx, "modifiers")
	if mods == 0 {
		return false
	}
	for i := 0; i < file.FlatChildCount(mods); i++ {
		child := file.FlatChild(mods, i)
		if file.FlatType(child) != "annotation" {
			continue
		}
		text := file.FlatNodeText(child)
		raw := text
		text = strings.TrimPrefix(text, "@")
		if colonIdx := strings.Index(text, ":"); colonIdx >= 0 {
			text = text[colonIdx+1:]
		}
		if parenIdx := strings.Index(text, "("); parenIdx >= 0 {
			text = text[:parenIdx]
		}
		text = strings.TrimSpace(text)
		if dotIdx := strings.LastIndex(text, "."); dotIdx >= 0 {
			text = text[dotIdx+1:]
		}
		if frameworkAnnotationNames[text] {
			return true
		}
		if text == "Suppress" || text == "SuppressWarnings" {
			if strings.Contains(raw, `"unused"`) ||
				strings.Contains(raw, `"UNUSED_PARAMETER"`) ||
				strings.Contains(raw, `"UNUSED_VARIABLE"`) ||
				strings.Contains(raw, `"UnusedPrivateProperty"`) ||
				strings.Contains(raw, `"UnusedPrivateMember"`) ||
				strings.Contains(raw, `"UnusedPrivateFunction"`) ||
				strings.Contains(raw, `"UnusedVariable"`) {
				return true
			}
		}
	}
	return false
}

// MayBeConstantRule detects top-level vals that could be const.
type MayBeConstantRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Initializers are
// checked structurally and same-file constant references are resolved when
// they share the same owner, but the finding remains a style preference.
func (r *MayBeConstantRule) Confidence() float64 { return api.ConfidenceMedium }

func mayBeConstantExpressionFlat(ctx *api.Context, expr uint32) bool {
	if ctx.File == nil || expr == 0 {
		return false
	}
	file := ctx.File
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		return !flatContainsStringInterpolation(file, expr)
	case "integer_literal", "long_literal", "real_literal", "hex_literal", "bin_literal", "character_literal", "boolean_literal":
		return true
	case "prefix_expression":
		return prefixConstantExpressionFlat(ctx, expr)
	case "simple_identifier", "navigation_expression":
		_, ok := rulesem.EvalConst(ctx, expr)
		return ok
	case "additive_expression", "multiplicative_expression":
		return binaryConstantExpressionFlat(ctx, expr)
	}
	return false
}

func prefixConstantExpressionFlat(ctx *api.Context, expr uint32) bool {
	file := ctx.File
	seenSign := false
	for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatNodeTextEquals(child, "+") || file.FlatNodeTextEquals(child, "-") {
			seenSign = true
			continue
		}
		if seenSign && file.FlatIsNamed(child) {
			child = flatUnwrapParenExpr(file, child)
			switch file.FlatType(child) {
			case "integer_literal", "long_literal", "real_literal", "hex_literal", "bin_literal":
				return true
			default:
				return mayBeConstantExpressionFlat(ctx, child)
			}
		}
	}
	return false
}

func binaryConstantExpressionFlat(ctx *api.Context, expr uint32) bool {
	file := ctx.File
	named := make([]uint32, 0, 2)
	validOp := false
	for child := file.FlatFirstChild(expr); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			named = append(named, child)
			continue
		}
		switch file.FlatNodeText(child) {
		case "+", "-", "*", "/", "%":
			validOp = true
		}
	}
	if !validOp || len(named) != 2 {
		return false
	}
	return mayBeConstantExpressionFlat(ctx, named[0]) && mayBeConstantExpressionFlat(ctx, named[1])
}

// ModifierOrderRule detects modifiers not in the recommended order.
type ModifierOrderRule struct {
	FlatDispatchBase
	BaseRule
}

var modifierOrder = []string{
	"public", "protected", "private", "internal",
	"expect", "actual",
	"final", "open", "abstract", "sealed",
	"const",
	"external",
	"override",
	"lateinit",
	"tailrec",
	"vararg",
	"suspend",
	"inner",
	"enum", "annotation", "fun",
	"companion",
	"inline", "noinline", "crossinline",
	"value",
	"infix",
	"operator",
	"data",
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *ModifierOrderRule) Confidence() float64 { return api.ConfidenceMedium }

func modifierIndex(mod string) int {
	for i, m := range modifierOrder {
		if m == mod {
			return i
		}
	}
	return -1
}

func sortModifiers(mods []string) []string {
	result := make([]string, len(mods))
	copy(result, mods)
	rank := func(m string) int {
		if i := modifierIndex(m); i >= 0 {
			return i
		}
		return len(modifierOrder)
	}
	sort.SliceStable(result, func(i, j int) bool { return rank(result[i]) < rank(result[j]) })
	return result
}

// FunctionOnlyReturningConstantRule detects functions that only return a constant.
type FunctionOnlyReturningConstantRule struct {
	// IgnoreIdioms skips functions that take parameters, bind through dependency injection or are tests. krit
	// extension; detekt reports them, so detekt's defaults set this to false.
	IgnoreIdioms bool
	FlatDispatchBase
	BaseRule
	ExcludedFunctions         []string
	IgnoreOverridableFunction bool
	IgnoreActualFunction      bool
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *FunctionOnlyReturningConstantRule) Confidence() float64 { return api.ConfidenceMedium }

func isConstant(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) >= 6 && strings.HasPrefix(s, `"""`) && strings.HasSuffix(s, `"""`) && !strings.Contains(s, "$") &&
		!strings.Contains(s[3:len(s)-3], `"""`) {
		return true
	}
	if isStringOrCharLiteral(s) {
		return true
	}
	if s == "true" || s == "false" || s == "null" {
		return true
	}
	return isNumericLiteral(s)
}

func isStringOrCharLiteral(s string) bool {
	if len(s) < 2 {
		return false
	}
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		if strings.Contains(s, "$") {
			return false
		}
		inner := s[1 : len(s)-1]
		return !strings.Contains(inner, "\"")
	}
	return false
}

func isNumericLiteral(s string) bool {
	if len(s) == 0 || s[0] < '0' || s[0] > '9' {
		return false
	}
	if len(s) > 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X' || s[1] == 'b' || s[1] == 'B') {
		for i := 2; i < len(s); i++ {
			c := s[i]
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') && (c < 'A' || c > 'F') && c != '_' && c != 'L' && c != 'u' && c != 'U' {
				return false
			}
		}
		return true
	}
	if numericHasOperatorChar(s) {
		return false
	}
	return numericDigitsOnly(s)
}

func numericHasOperatorChar(s string) bool {
	for _, c := range s {
		switch c {
		case ' ', '\t', '(', ')', '+', '-', '*', '/', '%',
			'&', '|', '^', '!', '~', '<', '>', '=':
			return true
		}
	}
	return false
}

func numericDigitsOnly(s string) bool {
	sawDot := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '.' {
			if sawDot {
				return false
			}
			sawDot = true
			continue
		}
		if c == 'L' || c == 'F' || c == 'f' || c == 'D' || c == 'd' || c == '_' || c == 'u' || c == 'U' {
			continue
		}
		return false
	}
	return true
}

// LoopWithTooManyJumpStatementsRule limits break/continue in loops.
type LoopWithTooManyJumpStatementsRule struct {
	FlatDispatchBase
	BaseRule
	MaxJumps int
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *LoopWithTooManyJumpStatementsRule) Confidence() float64 { return api.ConfidenceMedium }

// ExplicitItLambdaParameterRule detects `{ it -> ... }` using AST-based analysis.
// It finds lambda_literal nodes with exactly one parameter named "it" and flags them.
// When the parameter has a type annotation (e.g. `{ it: Int -> ... }`), a different
// message is used because the parameter cannot simply be removed.
type ExplicitItLambdaParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *ExplicitItLambdaParameterRule) Confidence() float64 { return api.ConfidenceMedium }

func findArrowInLambdaFlat(file *scanner.File, lambda uint32) uint32 {
	for i := 0; i < file.FlatChildCount(lambda); i++ {
		child := file.FlatChild(lambda, i)
		if file.FlatNodeTextEquals(child, "->") {
			return child
		}
	}
	return 0
}

// ExplicitItLambdaMultipleParametersRule detects 'it' as a parameter name in
// lambdas with multiple parameters. Naming a parameter 'it' is confusing when
// there are other parameters because 'it' normally refers to the implicit
// single parameter.
type ExplicitItLambdaMultipleParametersRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Style/expression rule. Detection is pattern-based and the preferred form
// is a style preference. Classified per roadmap/17.
func (r *ExplicitItLambdaMultipleParametersRule) Confidence() float64 { return api.ConfidenceMedium }

func isInsideLambdaUnderFlat(child, stopAt uint32, file *scanner.File) bool {
	for p, ok := file.FlatParent(child); ok && p != stopAt; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "lambda_literal" {
			return true
		}
	}
	return false
}

// mayBeConstantIsRealPropertyDeclaration rejects property nodes that error recovery made out of a
// constructor header, or out of a function-body local after the class ended early.
func mayBeConstantIsRealPropertyDeclaration(file *scanner.File, prop uint32) bool {
	if !file.FlatHasChildOfType(prop, "variable_declaration") || unusedVariablePropertyIsConstructorHeaderArtifact(file, prop) {
		return false
	}
	parent, ok := file.FlatParent(prop)
	if !ok {
		return false
	}
	switch file.FlatType(parent) {
	case "source_file":
		for sib, ok := file.FlatPrevSibling(prop); ok; sib, ok = file.FlatPrevSibling(sib) {
			switch file.FlatType(sib) {
			case "ERROR", "call_expression", "infix_expression", "assignment", "navigation_expression":
				return false
			}
		}
	case "class_body":
		return !flatSubtreeHasNodeType(file, parent, "ERROR")
	}
	return true
}

var mayBeConstantTypes = map[string]bool{
	"Int": true, "Long": true, "Short": true, "Byte": true, "Char": true, "Boolean": true,
	"Float": true, "Double": true, "String": true, "UInt": true, "ULong": true, "UShort": true, "UByte": true,
}

// mayBeConstantDeclarationAllowed rejects properties that cannot be const whatever their initializer is:
// custom accessors (parsed as following siblings), delegates, expect/actual/external, and
// declared types that are nullable or not a primitive or String.
func mayBeConstantDeclarationAllowed(file *scanner.File, idx uint32) bool {
	if next, ok := file.FlatNextSibling(idx); ok {
		if t := file.FlatType(next); t == "getter" || t == "setter" {
			return false
		}
	}
	for _, m := range []string{"actual", "expect", "external", "lateinit"} {
		if file.FlatHasModifier(idx, m) {
			return false
		}
	}
	if file.FlatHasChildOfType(idx, "property_delegate") || file.FlatHasChildOfType(idx, "getter") || file.FlatHasChildOfType(idx, "setter") {
		return false
	}
	if decl, ok := file.FlatFindChild(idx, "variable_declaration"); ok && decl != 0 {
		for c := file.FlatFirstChild(decl); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "nullable_type":
				return false
			case "user_type":
				if !mayBeConstantTypes[strings.TrimSpace(file.FlatNodeText(c))] {
					return false
				}
			}
		}
	}
	return true
}
