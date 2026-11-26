package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Ports of SharedTransitionScopeDetector and LookaheadAnimationVisualDebuggingDetector from
// androidx.compose.animation:animation-android 1.13.0, and ArcAnimationSpecTypeDetector from
// androidx.compose.animation:animation-core-android 1.13.0.

const (
	axlLookaheadDebugMessage = "LookaheadAnimationVisualDebugging and CustomizedLookaheadAnimationVisualDebugging " +
		"are disallowed in production code."
	axlArcMessage = "Arc animation is intended for 2D values such as Offset, IntOffset or DpOffset.\n" +
		"Otherwise, the animation might not be what you expect."
)

// UnusedSharedTransitionModifierParameterRule ports UnusedSharedTransitionModifierParameter: the content lambda of
// SharedTransitionScope ignores the Modifier it is given.
type UnusedSharedTransitionModifierParameterRule struct {
	FlatDispatchBase
	BaseRule
}

// ConstantContentStateKeyInItemsCall ports ConstantContentStateKeyInItemsCall: a shared element in a LazyList
// `items` block keyed by a constant, so every item shares one key.
type ConstantContentStateKeyInItemsCallRule struct {
	FlatDispatchBase
	BaseRule
}

// DisallowLookaheadAnimationVisualDebugRule ports DisallowLookaheadAnimationVisualDebug: the lookahead animation
// debugging wrappers left in production code.
type DisallowLookaheadAnimationVisualDebugRule struct {
	FlatDispatchBase
	BaseRule
}

// ArcAnimationSpecTypeIssueRule ports ArcAnimationSpecTypeIssue: an ArcAnimationSpec for a value that is not a 2D
// offset.
type ArcAnimationSpecTypeIssueRule struct {
	FlatDispatchBase
	BaseRule
}

func (r *UnusedSharedTransitionModifierParameterRule) fileMentions() []string {
	return []string{axlAnimationPackage}
}
func (r *ConstantContentStateKeyInItemsCallRule) fileMentions() []string {
	return []string{"rememberSharedContentState"}
}
func (r *DisallowLookaheadAnimationVisualDebugRule) fileMentions() []string {
	return []string{"LookaheadAnimationVisualDebugging"}
}
func (r *ArcAnimationSpecTypeIssueRule) fileMentions() []string { return []string{"ArcAnimationSpec"} }

// Confidence is medium: the composable in between is proven by its name and an androidx.compose import or a
// same-file @Composable declaration, where Lint resolves it.
func (r *ConstantContentStateKeyInItemsCallRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *UnusedSharedTransitionModifierParameterRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnusedSharedTransitionModifierParameter", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: append([]string{
			"Lint's quick fix, which moves the Modifier onto the single top-level layout composable, is not ported.",
		}, axlTargetStateLimitations...)}
}

func (r *ConstantContentStateKeyInItemsCallRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ConstantContentStateKeyInItemsCall", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"The key must fold to a constant from literals, + concatenation and same-file constants.",
			"The call between sharedElement/sharedBounds and items must be a same-file @Composable function or an " +
				"upper-case function imported from androidx.compose.",
		}}
}

func (r *DisallowLookaheadAnimationVisualDebugRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DisallowLookaheadAnimationVisualDebug", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"Lint's quick fix, which unwraps the content lambda, is not ported.",
		}}
}

func (r *ArcAnimationSpecTypeIssueRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ArcAnimationSpecTypeIssue", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"As in Lint, only an explicit type argument is checked; `ArcAnimationSpec()` with an inferred type is not.",
		}}
}

// check mirrors SharedTransitionScopeDetector's unused-Modifier check on the content lambda of
// SharedTransitionScope { }.
func (r *UnusedSharedTransitionModifierParameterRule) check(ctx *api.Context) {
	file, lambda := ctx.File, ctx.Idx
	// content is SharedTransitionScope's only parameter, so Lint's getArgumentForParameter(0) also matches it
	// positionally: SharedTransitionScope({ modifier -> }).
	arg, ok := acrLambdaArgumentOf(file, lambda)
	if !ok || (!arg.Trailing && arg.Label != "content" && arg.Index != 0) ||
		!axlAnimationCallTargets(file, arg.Call, "SharedTransitionScope") {
		return
	}
	for _, p := range axlUnreferencedLambdaParams(file, lambda, 1) {
		ctx.EmitAtNode(p.Node, "Supplied Modifier parameter should be used on the top most Composable. "+
			"Otherwise, consider using `SharedTransitionLayout`.")
	}
}

// check mirrors SharedTransitionScopeDetector's constant-key check: rememberSharedContentState(constant) whose
// enclosing calls are sharedElement/sharedBounds, then a composable, then a LazyList items/itemsIndexed.
func (r *ConstantContentStateKeyInItemsCallRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if acrIsInnerCallOfTrailingLambda(file, node) {
		return
	}
	name, _, _ := acrCallCallee(file, node)
	if name != "rememberSharedContentState" || len(acrFunctionDecls(file, name)) > 0 ||
		!acuiImportsPackage(file, axlAnimationPackage) {
		return
	}
	call, ok := lintCallAt(file, node)
	if !ok || !axlIsConstant(file, call.Arg(0, "key")) {
		return
	}
	shared := axlEnclosingCallUAST(file, node)
	sharedName, _, _ := acrCallCallee(file, shared)
	if shared == 0 || (sharedName != "sharedElement" && sharedName != "sharedBounds") ||
		len(acrFunctionDecls(file, sharedName)) > 0 {
		return
	}
	composable := axlEnclosingCallUAST(file, shared)
	if composable == 0 || !axlCallIsComposable(file, composable) {
		return
	}
	items := axlEnclosingCallUAST(file, composable)
	if items == 0 || !axlIsLazyItemsCall(file, items) {
		return
	}
	ctx.EmitAtNode(node, "Each Composable within a LazyList `items` call should have unique content state keys. "+
		"Make sure to either associate a unique key related to the item's data, or simply append the item's index "+
		"to the key.")
}

// axlEnclosingCallUAST returns the nearest call whose arguments or lambda contain node, as UAST's
// getParentOfType<UCallExpression> sees it: a call that only has node in its receiver chain is not a parent.
// `Box(args) { }` parses as an outer call wrapping the inner `Box(args)`; UAST sees one call, so a node in the
// inner call's arguments has the outer call as its parent.
func axlEnclosingCallUAST(file *scanner.File, node uint32) uint32 {
	child := node
	for p, ok := file.FlatParent(node); ok; p, ok = file.FlatParent(p) {
		if file.FlatType(p) == "call_expression" && file.FlatType(child) == "call_suffix" {
			if acrIsInnerCallOfTrailingLambda(file, p) {
				outer, _ := file.FlatParent(p)
				return outer
			}
			return p
		}
		child = p
	}
	return 0
}

// axlIsConstant mirrors UExpression.evaluate() != null for a key: a literal, or a string or integer folded from
// literals, `+` and same-file constants.
func axlIsConstant(file *scanner.File, expr uint32) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 {
		return false
	}
	switch file.FlatType(expr) {
	case "boolean_literal", "character_literal", "real_literal", "null":
		return true
	}
	// evaluate() follows a `val` but not a `var`, even one never reassigned (which lintEvalStringStrict accepts).
	followsVar := false
	file.FlatWalkNodes(expr, "simple_identifier", func(id uint32) {
		if p, ok := file.FlatParent(id); ok && file.FlatType(p) == "navigation_suffix" {
			return
		}
		if lintInitializerOf(file, id, file.FlatNodeText(id), false) == 0 {
			followsVar = true
		}
	})
	if followsVar {
		return false
	}
	if _, ok := lintEvalStringStrict(file, expr); ok {
		return true
	}
	_, ok := lintEvalInt(file, expr, nil)
	return ok
}

// axlCallIsComposable proves that a call targets a @Composable function: a same-file @Composable declaration, or
// an upper-case function (the Compose naming convention for emitters) imported from androidx.compose.
func axlCallIsComposable(file *scanner.File, call uint32) bool {
	name, _, hasReceiver := acrCallCallee(file, call)
	if name == "" {
		return false
	}
	if decls := acrFunctionDecls(file, name); len(decls) > 0 {
		for _, d := range decls {
			if !acrFunctionIsComposable(file, d) {
				return false
			}
		}
		return true
	}
	if hasReceiver || !acrFirstRuneIsUpper(name) {
		return false
	}
	facts := fileFactsCache().Imports(file)
	if target, ok := facts.Aliases[name]; ok {
		return strings.HasPrefix(target, "androidx.compose.")
	}
	for w := range facts.Wildcards {
		if strings.HasPrefix(w, "androidx.compose.") {
			return true
		}
	}
	return false
}

// axlIsLazyItemsCall reports whether call is items/itemsIndexed from an androidx foundation lazy package
// (Compose foundation, TV or Wear): imported by name, or the member of a LazyListScope receiver when the file
// imports such a package.
func axlIsLazyItemsCall(file *scanner.File, call uint32) bool {
	name, _, _ := acrCallCallee(file, call)
	if (name != "items" && name != "itemsIndexed") || len(acrFunctionDecls(file, name)) > 0 {
		return false
	}
	facts := fileFactsCache().Imports(file)
	isLazyPackage := func(fqn string) bool {
		return strings.HasPrefix(fqn, "androidx.") && strings.Contains(fqn, "foundation")
	}
	if target, ok := facts.Aliases[name]; ok {
		return isLazyPackage(target)
	}
	for fqn := range facts.FQNs {
		if isLazyPackage(fqn) && strings.Contains(fqn, ".lazy.") {
			return true
		}
	}
	for w := range facts.Wildcards {
		if isLazyPackage(w) && strings.Contains(w, "lazy") {
			return true
		}
	}
	return false
}

// check mirrors LookaheadAnimationVisualDebuggingDetector: a call of LookaheadAnimationVisualDebugging or
// CustomizedLookaheadAnimationVisualDebugging from androidx.compose.animation, at the whole call.
func (r *DisallowLookaheadAnimationVisualDebugRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if scanner.IsTestFile(file.Path) || acrIsInnerCallOfTrailingLambda(file, node) {
		return
	}
	if !axlAnimationCallTargets(file, node, "LookaheadAnimationVisualDebugging") &&
		!axlAnimationCallTargets(file, node, "CustomizedLookaheadAnimationVisualDebugging") {
		return
	}
	ctx.EmitAtNode(node, axlLookaheadDebugMessage)
}

// axlArcPreferredTypes are the 2D offset types ArcAnimationSpec is meant for.
var axlArcPreferredTypes = map[string]bool{
	"androidx.compose.ui.geometry.Offset": true, "androidx.compose.ui.unit.IntOffset": true,
	"androidx.compose.ui.unit.DpOffset": true,
}

// check mirrors ArcAnimationSpecTypeDetector for Kotlin (Java cannot call the constructor, which takes an inline
// class): an androidx.compose.animation.core.ArcAnimationSpec constructor with
// an explicit type argument that is not Offset, IntOffset or DpOffset, at the constructor name.
func (r *ArcAnimationSpecTypeIssueRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if scanner.IsTestFile(file.Path) || acrIsInnerCallOfTrailingLambda(file, node) {
		return
	}
	if !axlCallResolvesToClass(file, node, axlAnimationCorePackage, "ArcAnimationSpec") {
		return
	}
	typeArg := recyclerCallTypeArgument(file, node)
	_, nameNode, _ := acrCallCallee(file, node)
	if typeArg == "" || strings.Contains(typeArg, ",") || axlArcPreferredTypes[axlQualifyType(file, typeArg)] {
		return
	}
	ctx.EmitAtNode(nameNode, axlArcMessage)
}

// axlCallResolvesToClass reports whether a Kotlin call's callee names pkg.name, directly, fully qualified, or
// through an import alias.
func axlCallResolvesToClass(file *scanner.File, call uint32, pkg, name string) bool {
	if acrCallTargets(file, call, pkg, name) {
		return true
	}
	callee := acrCalleeExpression(file, call)
	return file.FlatType(callee) == "simple_identifier" &&
		acrSimpleNameRefersTo(file, file.FlatNodeText(callee), pkg, name) && !acrFileDeclaresTopLevel(file, name)
}

// axlQualifyType returns the fully qualified name of a type argument: as written when qualified, otherwise through
// the file's explicit imports (Lint's canonicalText), or the simple name when nothing imports it. canonicalText
// expands type aliases, so a same-file `typealias Pos = Offset` qualifies as Offset (an explicit import still wins
// over a same-file declaration, as in Kotlin).
func axlQualifyType(file *scanner.File, typ string) string {
	imports := fileFactsCache().Imports(file).Aliases
	typ = strings.TrimSuffix(strings.Join(strings.Fields(typ), ""), "?")
	for depth := 0; depth < 8 && !strings.Contains(typ, "."); depth++ {
		if _, imported := imports[typ]; imported {
			break
		}
		target := lintSameFileTypeAlias(file, typ)
		if target == "" {
			break
		}
		typ = strings.Join(strings.Fields(target), "")
	}
	if strings.Contains(typ, ".") {
		return typ
	}
	if target, ok := imports[typ]; ok {
		return target
	}
	for fqn := range axlArcPreferredTypes {
		pkg, simple := fqn[:strings.LastIndex(fqn, ".")], fqn[strings.LastIndex(fqn, ".")+1:]
		if simple == typ && acrNameResolvesTo(file, pkg, simple) {
			return fqn
		}
	}
	return typ
}
