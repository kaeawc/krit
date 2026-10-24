package rules

import (
	"bytes"
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

// SafeCastRule detects `if (x is Type) { x as Type }` patterns that should use `x as? Type`.
// This is distinct from UnsafeCast, which is reserved for casts that can never succeed.
// SafeCast only fires when an is-check + cast pattern is detected (the cast is redundant
// because the is-check already proves the type).
type SafeCastRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. The rule
// matches the `if (x is T) { x as T }` pattern on text, which is
// narrow enough to avoid most false positives but can stumble on
// multi-branch or shadowed variables. Roadmap/17 notes that this
// rule and UnsafeCast fire on overlapping locations; medium
// confidence is appropriate for the redundant-cast half of that
// pair.
func (r *SafeCastRule) Confidence() float64 { return api.ConfidenceMedium }

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
	for i := 0; i < len(result); i++ {
		for j := i + 1; j < len(result); j++ {
			ii := modifierIndex(result[i])
			jj := modifierIndex(result[j])
			if ii < 0 {
				ii = 9999
			}
			if jj < 0 {
				jj = 9999
			}
			if jj < ii {
				result[i], result[j] = result[j], result[i]
			}
		}
	}
	return result
}

// FunctionOnlyReturningConstantRule detects functions that only return a constant.
type FunctionOnlyReturningConstantRule struct {
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
