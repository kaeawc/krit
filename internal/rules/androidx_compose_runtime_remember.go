package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// UnrememberedMutableStateRule ports runtime-lint's UnrememberedMutableState (UnrememberedStateDetector): a state
// object created during composition without `remember`.
type UnrememberedMutableStateRule struct {
	FlatDispatchBase
	BaseRule
}

// RememberInCompositionRule ports runtime-lint's RememberInComposition: a @RememberInComposition declaration
// called during composition without `remember`.
type RememberInCompositionRule struct {
	FlatDispatchBase
	BaseRule
}

// FrequentlyChangingValueRule ports runtime-lint's FrequentlyChangingValue: a @FrequentlyChangingValue value read
// during composition.
type FrequentlyChangingValueRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports medium confidence: the factory and composition proofs are source-level.
func (r *UnrememberedMutableStateRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: the annotation and composition proofs are source-level.
func (r *RememberInCompositionRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: the annotation and composition proofs are source-level.
func (r *FrequentlyChangingValueRule) Confidence() float64 { return api.ConfidenceMedium }

// acrUnrememberedMutableStateAliases is the retired krit rule this port replaced; its ID still suppresses findings
// and configures the rule.
var acrUnrememberedMutableStateAliases = []string{"ComposeMutableStateInComposition"}

// acrRememberInCompositionAliases are the retired Lint IDs RememberInComposition still honors for suppression.
var acrRememberInCompositionAliases = []string{"UnrememberedAnimatable", "UnrememberedMutableInteractionSource"}

// acrFrequentlyChangingValueAliases is the retired Lint ID FrequentlyChangingValue still honors for suppression.
var acrFrequentlyChangingValueAliases = []string{"FrequentlyChangedStateReadInComposition"}

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *UnrememberedMutableStateRule) fileMentions() []string { return []string{"androidx.compose"} }

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *RememberInCompositionRule) fileMentions() []string { return []string{"androidx.compose"} }

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *FrequentlyChangingValueRule) fileMentions() []string { return []string{"androidx.compose"} }

func (r *UnrememberedMutableStateRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UnrememberedMutableState", RuleSet: "compose", DefaultActive: true,
		Aliases: acrUnrememberedMutableStateAliases,
		KnownLimitations: append([]string{
			"User @StateFactoryMarker functions and State subclasses are recognized only when declared in the same file.",
		}, acrCompositionLimitations[0])}
}

func (r *RememberInCompositionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RememberInComposition", RuleSet: "compose", DefaultActive: true,
		Aliases: acrRememberInCompositionAliases,
		KnownLimitations: append([]string{
			"Annotated declarations are recognized in the same file and for the Compose 1.12.1 / material3 1.5.0 APIs (Animatable, MutableInteractionSource, FocusRequester, TextFieldState, SliderState, movableContentOf, ...).",
			"The retired IDs UnrememberedAnimatable and UnrememberedMutableInteractionSource suppress every finding of this rule in their scope, not only Animatable and MutableInteractionSource calls.",
		}, acrCompositionLimitations[0])}
}

func (r *FrequentlyChangingValueRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "FrequentlyChangingValue", RuleSet: "compose", DefaultActive: true,
		Aliases: acrFrequentlyChangingValueAliases,
		KnownLimitations: append([]string{
			"Annotated declarations are recognized in the same file and for the Compose 1.12.1 / material3 1.5.0 state classes (LazyListState, LazyGridState, ScrollState, PagerState, ...), whose receiver type must be proven by a same-file declaration or initializer.",
		}, acrCompositionLimitations...)}
}

func registerAndroidxComposeRuntimeRememberRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	unremembered := &UnrememberedMutableStateRule{BaseRule: BaseRule{RuleName: "UnrememberedMutableState",
		RuleSetName: "compose", Sev: "error",
		Desc: "Detects state objects created during composition without remember, which are recreated and lose their value on recomposition."}}
	registerAcrRule(unremembered.BaseRule, unremembered, []string{"call_expression", "object_literal"}, kotlin,
		unremembered.check, acrUnrememberedMutableStateAliases...)

	remember := &RememberInCompositionRule{BaseRule: BaseRule{RuleName: "RememberInComposition", RuleSetName: "compose",
		Sev: "error", Desc: "Detects @RememberInComposition APIs such as Animatable or MutableInteractionSource called during composition without remember."}}
	registerAcrRule(remember.BaseRule, remember, []string{"call_expression", "navigation_expression"},
		kotlin, remember.check, acrRememberInCompositionAliases...)

	frequent := &FrequentlyChangingValueRule{BaseRule: BaseRule{RuleName: "FrequentlyChangingValue", RuleSetName: "compose",
		Sev: "warning", Desc: "Detects @FrequentlyChangingValue values such as LazyListState.firstVisibleItemIndex read during composition."}}
	registerAcrRule(frequent.BaseRule, frequent, []string{"call_expression", "navigation_expression"},
		kotlin, frequent.check, acrFrequentlyChangingValueAliases...)
}

const acrUnrememberedStateMessage = "Creating a state object during composition without using `remember`"

// acrStateFactories are the androidx.compose.runtime functions annotated @StateFactoryMarker in 1.12.1.
var acrStateFactories = map[string]bool{
	"mutableStateOf": true, "mutableIntStateOf": true, "mutableLongStateOf": true, "mutableFloatStateOf": true,
	"mutableDoubleStateOf": true, "mutableStateListOf": true, "mutableStateMapOf": true, "mutableStateSetOf": true,
	"derivedStateOf": true,
}

// acrStateTypes are the androidx.compose.runtime State interfaces.
var acrStateTypes = []string{
	"State", "MutableState", "IntState", "MutableIntState", "LongState", "MutableLongState", "FloatState",
	"MutableFloatState", "DoubleState", "MutableDoubleState",
}

func (r *UnrememberedMutableStateRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	if file.FlatType(node) == "object_literal" {
		if acrClassSupertypeIs(file, node, 0, acrRuntimePackage, acrStateTypes...) && acrNotRemembered(file, node) {
			ctx.EmitAtNode(node, acrUnrememberedStateMessage)
		}
		return
	}
	if acrIsInnerCallOfTrailingLambda(file, node) || !acrCallCreatesState(file, node) || !acrNotRemembered(file, node) {
		return
	}
	_, nameNode, _ := acrCallCallee(file, node)
	ctx.EmitAtNode(nameNode, acrUnrememberedStateMessage)
}

// acrCallCreatesState reports whether a call is a state factory (a runtime @StateFactoryMarker function or a
// same-file one) or the constructor of a same-file class implementing State.
func acrCallCreatesState(file *scanner.File, call uint32) bool {
	name, _, hasReceiver := acrCallCallee(file, call)
	if name == "" {
		return false
	}
	if acrStateFactories[name] {
		if hasReceiver {
			return acrCallTargets(file, call, acrRuntimePackage, name)
		}
		if acrCalleeResolvesTo(file, call, acrRuntimePackage, name) {
			return true
		}
	}
	if hasReceiver {
		return false
	}
	if class := flatFindSameFileClassLikeDeclaration(file, name); class != 0 && file.FlatType(class) == "class_declaration" {
		return acrClassSupertypeIs(file, class, 0, acrRuntimePackage, acrStateTypes...)
	}
	if fn := acrSameFileFunction(file, name); fn != 0 {
		return acrHasAnnotation(file, fn, acrRuntimePackage+".snapshots", "StateFactoryMarker", "")
	}
	return false
}

const acrRememberInCompositionMessage = "Calling a @RememberInComposition annotated declaration inside composition without using `remember`"

// acrRememberInCompositionAPIs are the library functions and constructors annotated @RememberInComposition in
// Compose 1.12.1 and material3 1.5.0, by simple name and package.
var acrRememberInCompositionAPIs = map[string][]string{
	"movableContentOf":                  {acrRuntimePackage},
	"movableContentWithReceiverOf":      {acrRuntimePackage},
	"MutableInteractionSource":          {"androidx.compose.foundation.interaction"},
	"BringIntoViewRequester":            {"androidx.compose.foundation.relocation"},
	"MutableStyleState":                 {"androidx.compose.foundation.style"},
	"BorderLogic":                       {"androidx.compose.foundation.border"},
	"FocusRequester":                    {"androidx.compose.ui.focus"},
	"MeshGradientPainter":               {"androidx.compose.ui.graphics"},
	"SliderState":                       {"androidx.compose.material3"},
	"RangeSliderState":                  {"androidx.compose.material3"},
	"CarouselParallaxScrollEffectState": {"androidx.compose.material3.carousel"},
	"Animatable":                        {"androidx.compose.animation.core", "androidx.compose.animation"},
}

func (r *RememberInCompositionRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	switch file.FlatType(node) {
	case "call_expression":
		if acrIsInnerCallOfTrailingLambda(file, node) || !acrCallIsRememberInComposition(file, node) ||
			!acrNotRemembered(file, node) {
			return
		}
		_, nameNode, _ := acrCallCallee(file, node)
		ctx.EmitAtNode(nameNode, acrRememberInCompositionMessage)
	default:
		if nameNode := acrAnnotatedPropertyRead(file, node, "RememberInComposition"); nameNode != 0 && acrNotRemembered(file, node) {
			ctx.EmitAtNode(nameNode, acrRememberInCompositionMessage)
		}
	}
}

func acrCallIsRememberInComposition(file *scanner.File, call uint32) bool {
	name, _, hasReceiver := acrCallCallee(file, call)
	if name == "" {
		return false
	}
	if !hasReceiver && !acrFileDeclaresAnyFunction(file, name) && flatFindSameFileClassLikeDeclaration(file, name) == 0 {
		for _, pkg := range acrRememberInCompositionAPIs[name] {
			if acrNameResolvesTo(file, pkg, name) {
				return true
			}
		}
	}
	if hasReceiver {
		for _, pkg := range acrRememberInCompositionAPIs[name] {
			if acrCallTargets(file, call, pkg, name) {
				return true
			}
		}
	}
	return acrFileMentions(file, "RememberInComposition") &&
		acrSameFileCallIsAnnotated(file, call, acrAnnotationPackage, "RememberInComposition")
}

// acrFileMentions is a cheap gate for the same-file annotation paths.
func acrFileMentions(file *scanner.File, text string) bool {
	return fileFactsCache().Mentions(file, text)
}

// acrAnnotatedPropertyRead returns the name node of a property read (`name` or `receiver.name`, not a call callee
// or an assignment target) whose same-file getter carries the runtime annotation, or 0.
func acrAnnotatedPropertyRead(file *scanner.File, node uint32, ann string) uint32 {
	if !acrFileMentions(file, ann) {
		return 0
	}
	nameNode := acrPropertyReadName(file, node)
	if nameNode == 0 || !acrSameFilePropertyReadIsAnnotated(file, node, acrAnnotationPackage, ann) {
		return 0
	}
	return nameNode
}

// acrPropertyReadName returns the property name node of a read: a bare identifier that is not a callee, label,
// declaration name or navigation member, or the last member of a navigation expression that is not a callee.
func acrPropertyReadName(file *scanner.File, node uint32) uint32 {
	parent, ok := file.FlatParent(node)
	if !ok {
		return 0
	}
	switch file.FlatType(node) {
	case "simple_identifier":
		switch file.FlatType(parent) {
		case "statements", "property_declaration", "value_argument", "parenthesized_expression", "jump_expression",
			"function_body", "additive_expression", "multiplicative_expression", "comparison_expression",
			"equality_expression", "string_template", "interpolated_expression", "control_structure_body":
			if file.FlatType(parent) == "property_declaration" && acrInitializer(file, parent) != node {
				return 0
			}
			return node
		}
		return 0
	case "navigation_expression":
		if file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == node {
			return 0
		}
		suffix := flatLastNamedChild(file, node)
		if suffix == 0 || file.FlatType(suffix) != "navigation_suffix" {
			return 0
		}
		nameNode, _ := file.FlatFindChild(suffix, "simple_identifier")
		return nameNode
	}
	return 0
}

const acrFrequentlyChangingValueMessage = "Reading a value annotated with @FrequentlyChangingValue inside composition"

// acrFrequentlyChangingType lists a library class whose members are annotated @FrequentlyChangingValue in Compose
// 1.12.1 / material3 1.5.0, and the factories whose result proves a receiver has that type.
type acrFrequentlyChangingType struct {
	pkg, name  string
	properties map[string]bool
	calls      map[string]bool
	factories  map[string]string // function name -> package
}

func acrNames(names ...string) map[string]bool {
	out := make(map[string]bool, len(names))
	for _, n := range names {
		out[n] = true
	}
	return out
}

var acrFrequentlyChangingTypes = []acrFrequentlyChangingType{
	{pkg: "androidx.compose.foundation.lazy", name: "LazyListState",
		properties: acrNames("firstVisibleItemIndex", "firstVisibleItemScrollOffset", "layoutInfo"),
		factories:  map[string]string{"rememberLazyListState": "androidx.compose.foundation.lazy", "LazyListState": "androidx.compose.foundation.lazy"}},
	{pkg: "androidx.compose.foundation.lazy.grid", name: "LazyGridState",
		properties: acrNames("firstVisibleItemIndex", "firstVisibleItemScrollOffset", "layoutInfo"),
		factories:  map[string]string{"rememberLazyGridState": "androidx.compose.foundation.lazy.grid", "LazyGridState": "androidx.compose.foundation.lazy.grid"}},
	{pkg: "androidx.compose.foundation", name: "ScrollState", properties: acrNames("value"),
		factories: map[string]string{"rememberScrollState": "androidx.compose.foundation", "ScrollState": "androidx.compose.foundation"}},
	{pkg: "androidx.compose.foundation.pager", name: "PagerState", properties: acrNames("currentPageOffsetFraction"),
		factories: map[string]string{"rememberPagerState": "androidx.compose.foundation.pager", "PagerState": "androidx.compose.foundation.pager"}},
	{pkg: "androidx.compose.foundation.gestures", name: "AnchoredDraggableState", properties: acrNames("offset", "progress"),
		calls: acrNames("progress", "requireOffset"), factories: map[string]string{"AnchoredDraggableState": "androidx.compose.foundation.gestures"}},
	{pkg: "androidx.compose.foundation", name: "ScrollIndicatorState", properties: acrNames("scrollOffset")},
	{pkg: "androidx.compose.material3", name: "SearchBarState", properties: acrNames("progress"),
		factories: map[string]string{"rememberSearchBarState": "androidx.compose.material3"}},
	{pkg: "androidx.compose.material3", name: "SearchBarScrollState", properties: acrNames("scrollOffset", "contentOffset")},
	{pkg: "androidx.compose.ui", name: "UiMediaScope", properties: acrNames("windowWidth", "windowHeight")},
	{pkg: "androidx.compose.ui.layout", name: "LayoutBoundsHolder", properties: acrNames("bounds")},
	{pkg: "androidx.compose.material3.carousel", name: "CarouselParallaxScrollEffectState", calls: acrNames("calculateItemInfo")},
}

func (r *FrequentlyChangingValueRule) check(ctx *api.Context) {
	file, node := ctx.File, ctx.Idx
	var nameNode uint32
	switch file.FlatType(node) {
	case "call_expression":
		if acrIsInnerCallOfTrailingLambda(file, node) {
			return
		}
		name, callName, hasReceiver := acrCallCallee(file, node)
		if hasReceiver {
			recv := file.FlatFirstChild(acrCalleeExpression(file, node))
			for i := range acrFrequentlyChangingTypes {
				if t := &acrFrequentlyChangingTypes[i]; t.calls[name] && acrExprHasLibraryType(file, recv, t, 0) {
					nameNode = callName
				}
			}
		}
		if nameNode == 0 && acrFileMentions(file, "FrequentlyChangingValue") &&
			acrSameFileCallIsAnnotated(file, node, acrAnnotationPackage, "FrequentlyChangingValue") {
			nameNode = callName
		}
	case "navigation_expression":
		if member := acrPropertyReadName(file, node); member != 0 {
			name := file.FlatNodeText(member)
			for i := range acrFrequentlyChangingTypes {
				if t := &acrFrequentlyChangingTypes[i]; t.properties[name] && acrExprHasLibraryType(file, file.FlatFirstChild(node), t, 0) {
					nameNode = member
				}
			}
		}
		if nameNode == 0 {
			nameNode = acrAnnotatedPropertyRead(file, node, "FrequentlyChangingValue")
		}
	default:
		nameNode = acrAnnotatedPropertyRead(file, node, "FrequentlyChangingValue")
	}
	if nameNode == 0 || !acrInComposition(file, node) {
		return
	}
	ctx.EmitAtNode(nameNode, acrFrequentlyChangingValueMessage)
}

// acrExprHasLibraryType proves an expression has the library type: a same-file declaration typed with it (or a
// same-file subclass), or initialized by one of its factories.
func acrExprHasLibraryType(file *scanner.File, expr uint32, t *acrFrequentlyChangingType, depth int) bool {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return false
	}
	switch file.FlatType(expr) {
	case "simple_identifier", "navigation_expression":
		decl := acrExprDeclaration(file, expr)
		if decl == 0 && file.FlatType(expr) == "simple_identifier" {
			if recv := acrScopeLambdaReceiver(file, expr); recv != 0 {
				return acrExprHasLibraryType(file, recv, t, depth+1)
			}
		}
		if decl == 0 {
			return false
		}
		if typeNode := acrDeclaredType(file, decl); typeNode != 0 {
			return acrTypeIs(file, typeNode, t.pkg, t.name)
		}
		return acrExprHasLibraryType(file, acrInitializer(file, decl), t, depth+1)
	case "call_expression":
		name, _, _ := acrCallCallee(file, expr)
		if pkg, ok := t.factories[name]; ok {
			return acrCallTargets(file, expr, pkg, name)
		}
	}
	return false
}
