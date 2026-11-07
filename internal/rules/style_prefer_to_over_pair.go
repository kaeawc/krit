package rules

import (
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// PreferToOverPairSyntaxRule flags `Pair(a, b)` constructor calls, which read
// better as `a to b`. It matches detekt's PreferToOverPairSyntax: only calls to
// the kotlin.Pair constructor, reported on the call, with no autofix.
type PreferToOverPairSyntaxRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. The match is by name plus
// import and declaration checks, since the Go rule has no full resolution.
func (r *PreferToOverPairSyntaxRule) Confidence() float64 { return api.ConfidenceMedium }

const kotlinPairFQN = "kotlin.Pair"

// pairShadowingWildcards are star-import bases known to declare their own `Pair`,
// which wins over the default kotlin.Pair import.
var pairShadowingWildcards = []string{
	"android.util.*",
	"androidx.core.util.*",
	"javafx.util.*",
	"org.apache.commons.lang3.tuple.*",
	"org.apache.commons.math3.util.*",
}

func checkPreferToOverPairSyntax(ctx *api.Context, r *PreferToOverPairSyntaxRule) {
	idx, file := ctx.Idx, ctx.File
	// `Pair(a) { b }` nests the parenthesized call under an outer call with the lambda.
	inner := idx
	if first := file.FlatFirstChild(idx); first != 0 && file.FlatType(first) == "call_expression" {
		inner = first
	}
	name, qualified := pairCallee(file, inner)
	if name == "" {
		return
	}
	args := pairValueArguments(file, inner)
	if inner != idx {
		args = append(args, pairTrailingLambdas(file, idx)...)
	}
	if len(args) != 2 {
		return
	}
	if !pairCalleeIsKotlinPair(ctx, name, qualified) {
		return
	}
	texts := make([]string, len(args))
	for i, a := range args {
		texts[i] = file.FlatNodeText(a)
	}
	ctx.Emit(r.Finding(file, callLine(file, idx), callCol(file, idx),
		"Pair is created by using the pair constructor. This can replaced by `"+strings.Join(texts, " to ")+"`."))
}

// pairCallee returns the written callee name of a call and whether it is the
// qualified form `kotlin.Pair`. Any other receiver returns an empty name.
func pairCallee(file *scanner.File, call uint32) (name string, qualified bool) {
	first := file.FlatFirstChild(call)
	if first == 0 {
		return "", false
	}
	switch file.FlatType(first) {
	case "simple_identifier":
		return file.FlatNodeText(first), false
	case "navigation_expression":
		if file.FlatNodeText(first) == kotlinPairFQN {
			return "Pair", true
		}
	}
	return "", false
}

// pairValueArguments returns the value arguments of the call, including a
// trailing lambda, or nil when the call has no argument list.
func pairValueArguments(file *scanner.File, call uint32) []uint32 {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return nil
	}
	list := flatCallSuffixValueArgs(file, suffix)
	if list == 0 {
		return nil
	}
	var out []uint32
	for c := file.FlatFirstChild(list); c != 0; c = file.FlatNextSib(c) {
		if file.FlatType(c) == "value_argument" {
			out = append(out, c)
		}
	}
	return append(out, pairTrailingLambdas(file, call)...)
}

// pairTrailingLambdas returns the trailing lambda nodes of the call, which are
// the last value arguments in detekt's PSI.
func pairTrailingLambdas(file *scanner.File, call uint32) []uint32 {
	suffix, ok := file.FlatFindChild(call, "call_suffix")
	if !ok {
		return nil
	}
	var out []uint32
	for c := file.FlatFirstChild(suffix); c != 0; c = file.FlatNextSib(c) {
		if t := file.FlatType(c); t == "annotated_lambda" || t == "lambda_literal" {
			out = append(out, c)
		}
	}
	return out
}

// pairCalleeIsKotlinPair reports whether a callee spelled name constructs
// kotlin.Pair, using the file's imports, its own declarations, and the project
// class index to rule out lookalikes.
func pairCalleeIsKotlinPair(ctx *api.Context, name string, qualified bool) bool {
	file := ctx.File
	imports := fileFactsCache().Imports(file)
	if qualified {
		return true
	}
	if fqn, ok := imports.Aliases[name]; ok {
		return fqn == kotlinPairFQN
	}
	if name != "Pair" {
		return false
	}
	for _, w := range pairShadowingWildcards {
		if _, ok := imports.Wildcards[w]; ok {
			return false
		}
	}
	if pairLookalikeInScope(file, ctx.Idx) {
		return false
	}
	if ctx.Resolver != nil {
		if info := ctx.Resolver.ClassHierarchy("Pair"); info != nil && info.FQN != "" && info.FQN != kotlinPairFQN {
			return false
		}
	}
	return true
}

// pairLookalikeInScope reports whether a class, typealias or function named
// Pair is declared in a scope that encloses call, which shadows kotlin.Pair.
func pairLookalikeInScope(file *scanner.File, call uint32) bool {
	found := false
	file.FlatWalkAllNodes(0, func(n uint32) {
		if found {
			return
		}
		switch file.FlatType(n) {
		case "class_declaration", "object_declaration", "function_declaration", "type_alias":
			if extractIdentifierFlat(file, n) != "Pair" {
				return
			}
			scope, ok := file.FlatParent(n)
			found = ok && (file.FlatType(scope) == "source_file" || isFlatAncestor(file, scope, call))
		}
	})
	return found
}

func isFlatAncestor(file *scanner.File, ancestor, node uint32) bool {
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		if p == ancestor {
			return true
		}
	}
	return false
}
