package rules

// Ports of the source checks bundled in androidx.appcompat:appcompat 1.8.0 (appcompat-lint): SetActionBarDetector,
// ColorStateListLoadingDetector, DrawableLoadingDetector, TextViewCompoundDrawablesApiDetector and
// SwitchUsageCodeDetector. The first four are BaseMethodDeprecationDetector subclasses: they report a call whose
// resolved method is declared by a framework class and, for setActionBar, whose receiver extends AppCompatActivity.

import (
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/rules/semantics"
	"github.com/kaeawc/krit/internal/scanner"
)

// UseSupportActionBarRule mirrors UseSupportActionBar: Activity.setActionBar(Toolbar) on an AppCompatActivity.
type UseSupportActionBarRule struct {
	FlatDispatchBase
	AndroidRule
}

// UseCompatLoadingForColorStateListsRule mirrors UseCompatLoadingForColorStateLists: Resources.getColorStateList(int).
type UseCompatLoadingForColorStateListsRule struct {
	FlatDispatchBase
	AndroidRule
}

// UseCompatLoadingForDrawablesRule mirrors UseCompatLoadingForDrawables: Context.getDrawable(int),
// Resources.getDrawable(int) and Resources.getDrawable(int, Theme).
type UseCompatLoadingForDrawablesRule struct {
	FlatDispatchBase
	AndroidRule
}

// UseCompatTextViewDrawableApisRule mirrors UseCompatTextViewDrawableApis: TextView.setCompoundDrawableTintList and
// TextView.setCompoundDrawableTintMode, including Kotlin property assignments that call them.
type UseCompatTextViewDrawableApisRule struct {
	FlatDispatchBase
	AndroidRule
}

// UseSwitchCompatOrMaterialCodeRule mirrors UseSwitchCompatOrMaterialCode: a variable of type android.widget.Switch or
// a class that extends it.
type UseSwitchCompatOrMaterialCodeRule struct {
	FlatDispatchBase
	AndroidRule
}

// Confidence reports a tier-2 (medium) base confidence: receivers are proven from declarations, imports and the
// resolver hierarchy, so receivers typed only by inference across files are missed rather than guessed.
func (r *UseSupportActionBarRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence; see UseSupportActionBarRule.
func (r *UseCompatLoadingForColorStateListsRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence; see UseSupportActionBarRule.
func (r *UseCompatLoadingForDrawablesRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence; see UseSupportActionBarRule.
func (r *UseCompatTextViewDrawableApisRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports a tier-2 (medium) base confidence; see UseSupportActionBarRule.
func (r *UseSwitchCompatOrMaterialCodeRule) Confidence() float64 { return api.ConfidenceMedium }

func registerAndroidXAppCompatCallChecks() {
	needs := api.NeedsTypeInfo | api.NeedsOracleCallTargets | api.NeedsResolver
	callNodes := []string{"call_expression", "method_invocation"}

	actionBar := &UseSupportActionBarRule{AndroidRule: lintAndroidRule("UseSupportActionBar",
		"Should not call `Activity.setActionBar` if you extend `AppCompatActivity`", ALCCorrectness, ALSWarning, 1)}
	registerLintRule(actionBar.AndroidRule, actionBar, actionBar.Confidence(), actionBar.check, callNodes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"setActionBar"}})

	colors := &UseCompatLoadingForColorStateListsRule{AndroidRule: lintAndroidRule("UseCompatLoadingForColorStateLists",
		"Should not call `Resources.getColorStateList` directly", ALCCorrectness, ALSWarning, 1)}
	registerLintRule(colors.AndroidRule, colors, colors.Confidence(), colors.check, callNodes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"getColorStateList"}})

	drawables := &UseCompatLoadingForDrawablesRule{AndroidRule: lintAndroidRule("UseCompatLoadingForDrawables",
		"Should not call `Context.getDrawable` or `Resources.getDrawable` directly", ALCCorrectness, ALSWarning, 1)}
	registerLintRule(drawables.AndroidRule, drawables, drawables.Confidence(), drawables.check, callNodes, needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"getDrawable"}})

	tint := &UseCompatTextViewDrawableApisRule{AndroidRule: lintAndroidRule("UseCompatTextViewDrawableApis",
		"Should not call `TextView.setCompoundDrawableTintList` or `TextView.setCompoundDrawableTintMode` directly",
		ALCCorrectness, ALSWarning, 1)}
	registerLintRule(tint.AndroidRule, tint, tint.Confidence(), tint.check, append(callNodes, "assignment"), needs,
		&api.JavaFactProfile{ReceiverTypesForCallees: []string{"setCompoundDrawableTintList", "setCompoundDrawableTintMode"}})

	sw := &UseSwitchCompatOrMaterialCodeRule{AndroidRule: lintAndroidRule("UseSwitchCompatOrMaterialCode",
		"Replace usage of `Switch` widget", ALCCorrectness, ALSWarning, 5)}
	registerLintRule(sw.AndroidRule, sw, sw.Confidence(), sw.check, []string{
		"property_declaration", "parameter", "class_parameter", "class_declaration",
		"field_declaration", "local_variable_declaration", "formal_parameter",
	}, 0, nil)
}

const (
	appCompatActivityClass  = "androidx.appcompat.app.AppCompatActivity"
	appCompatResourcesClass = "android.content.res.Resources"
	appCompatTextViewClass  = "android.widget.TextView"
	appCompatSwitchClass    = "android.widget.Switch"
)

// appCompatActivityKnown and appCompatTextViewKnown are the library classes known to extend AppCompatActivity and
// TextView, for when the resolver has no hierarchy for them.
var (
	appCompatActivityKnown = map[string]struct{}{"AppCompatActivity": {}}
	appCompatTextViewKnown = map[string]struct{}{
		"TextView": {}, "Button": {}, "EditText": {}, "CheckBox": {}, "RadioButton": {}, "Switch": {}, "ToggleButton": {},
		"CompoundButton": {}, "CheckedTextView": {}, "Chronometer": {}, "TextClock": {}, "AutoCompleteTextView": {},
		"MultiAutoCompleteTextView": {}, "ExtractEditText": {}, "AppCompatTextView": {}, "AppCompatButton": {},
		"AppCompatEditText": {}, "AppCompatCheckBox": {}, "AppCompatRadioButton": {}, "AppCompatCheckedTextView": {},
		"AppCompatAutoCompleteTextView": {}, "AppCompatMultiAutoCompleteTextView": {}, "AppCompatToggleButton": {},
		"SwitchCompat": {}, "MaterialButton": {}, "MaterialTextView": {}, "TextInputEditText": {}, "Chip": {},
		"MaterialSwitch": {}, "MaterialCheckBox": {}, "MaterialRadioButton": {}, "ExtendedFloatingActionButton": {},
		"MaterialAutoCompleteTextView": {},
	}
	appCompatContextKnown = func() map[string]struct{} {
		out := make(map[string]struct{}, len(contextSubclassNames))
		for name := range contextSubclassNames {
			out[name] = struct{}{}
		}
		return out
	}()
)

// appCompatDeclared reports whether the project declares a dependency that brings appcompat, whose AAR ships these
// checks. Lint only runs them when that AAR is on the classpath; an undeclared or transitive-only appcompat is missed.
func appCompatDeclared(ctx *api.Context) bool {
	return librarymodel.EnsureFacts(ctx.LibraryFacts).Profile.HasAnyDependency(appCompatCoordinates...)
}

// appCompatResolvedOwner returns the declaring class of the call from Java facts or the oracle, when they resolve it.
func appCompatResolvedOwner(ctx *api.Context, call lintCall) (string, bool) {
	if fact, ok := javaSemanticCallFact(ctx, call.Idx); ok && fact.MethodOwner != "" {
		return fact.MethodOwner, true
	}
	target, ok := semantics.ResolveCallTarget(ctx, call.Idx)
	if !ok || !target.Resolved {
		return "", false
	}
	owner := strings.ReplaceAll(target.QualifiedName, "#", ".")
	if dot := strings.LastIndex(owner, "."); dot >= 0 {
		owner = owner[:dot]
	}
	return owner, true
}

// appCompatReceiverText is the receiver's source text without `!!`, a trailing `?` and a `this.` qualifier.
func appCompatReceiverText(call lintCall) string {
	text := strings.TrimSpace(call.ReceiverText)
	text = strings.TrimSuffix(strings.TrimSuffix(text, "!!"), "?")
	return strings.TrimPrefix(text, "this.")
}

// appCompatReceiverReaches reports whether the call's receiver, explicit or implicit, is an instance of fqn.
func appCompatReceiverReaches(ctx *api.Context, call lintCall, fqn string, known map[string]struct{}) bool {
	if call.ReceiverNode == 0 || appCompatReceiverText(call) == "this" {
		return appCompatImplicitReceiverReaches(ctx, call.Idx, fqn, known)
	}
	if javaSemanticCallReceiverTypeMatches(ctx, call.Idx, fqn) {
		return true
	}
	if typ := appCompatExpressionTypeText(ctx.File, call.Idx, call.ReceiverNode); typ != "" {
		return lintTypeNameReaches(ctx, typ, fqn, known, 0)
	}
	return false
}

// appCompatExpressionTypeText names the type of expr from its declaration, constructor call, cast or the type argument
// of a findViewById-style call, or "" when unknown.
func appCompatExpressionTypeText(file *scanner.File, scope, expr uint32) string {
	expr = flatUnwrapParenExpr(file, expr)
	switch file.FlatType(expr) {
	case "call_expression":
		if inner, ok := lintCallAt(file, expr); ok && appCompatViewLookups[inner.Name] {
			return appCompatCallTypeArgument(file, expr)
		}
	case "cast_expression", "as_expression":
		return lintReceiverClassText(file, lintCall{File: file, Idx: scope, ReceiverNode: expr,
			ReceiverText: strings.TrimSpace(file.FlatNodeText(expr))})
	}
	return lintReceiverTypeText(file, lintCall{File: file, Idx: scope, ReceiverNode: expr,
		ReceiverText: strings.TrimSpace(file.FlatNodeText(expr))})
}

// appCompatViewLookups are the generic view lookups whose type argument names the returned view type.
var appCompatViewLookups = map[string]bool{"findViewById": true, "requireViewById": true}

// appCompatCallTypeArgument returns the single type argument of a Kotlin call such as findViewById<TextView>(id).
func appCompatCallTypeArgument(file *scanner.File, call uint32) string {
	for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "call_suffix" {
			continue
		}
		args, ok := file.FlatFindChild(child, "type_arguments")
		if !ok {
			return ""
		}
		text := strings.TrimSpace(file.FlatNodeText(args))
		text = strings.TrimSuffix(strings.TrimPrefix(text, "<"), ">")
		if strings.ContainsAny(text, ",<") {
			return ""
		}
		return strings.TrimSuffix(strings.TrimSpace(text), "?")
	}
	return ""
}

// appCompatImplicitReceiverReaches reports whether the implicit receiver at idx is an instance of fqn: the subject of
// an enclosing apply/run/with lambda, the receiver of an enclosing extension function, or the enclosing class. Inside a
// scope function or extension function whose subject type is unknown, it reports false instead of guessing.
func appCompatImplicitReceiverReaches(ctx *api.Context, idx uint32, fqn string, known map[string]struct{}) bool {
	file := ctx.File
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "lambda_literal":
			if subject, scoped := appCompatScopeSubjectType(file, cur); scoped {
				return subject != "" && lintTypeNameReaches(ctx, subject, fqn, known, 0)
			}
		case "function_declaration":
			if recv := functionReceiverTypeText(file, cur); recv != "" {
				return lintTypeNameReaches(ctx, strings.TrimSuffix(strings.TrimSpace(recv), "?"), fqn, known, 0)
			}
		case "class_declaration", "object_declaration", "object_literal":
			return lintClassExtends(ctx, cur, fqn, known)
		case "object_creation_expression":
			if _, hasBody := file.FlatFindChild(cur, "class_body"); hasBody {
				return lintClassExtends(ctx, cur, fqn, known)
			}
		}
	}
	return false
}

// appCompatScopeSubjectType reports whether the lambda is the block of apply, run or with, which rebind `this`, and
// the type of the subject they bind it to ("" when unknown).
func appCompatScopeSubjectType(file *scanner.File, lambda uint32) (string, bool) {
	call := lambda
	for i := 0; i < 3; i++ {
		parent, ok := file.FlatParent(call)
		if !ok {
			return "", false
		}
		call = parent
		if file.FlatType(call) == "call_expression" {
			break
		}
	}
	if file.FlatType(call) != "call_expression" {
		return "", false
	}
	c, ok := lintCallAt(file, call)
	if inner := file.FlatFirstChild(call); (!ok || c.Name == "") && inner != 0 && file.FlatType(inner) == "call_expression" {
		// `with(x) { }` parses as a call of the inner call `with(x)` with a trailing lambda.
		c, ok = lintCallAt(file, inner)
		call = inner
	}
	if !ok {
		return "", false
	}
	switch c.Name {
	case "apply", "run":
		if c.ReceiverNode == 0 {
			// A receiver-less run keeps the enclosing `this`.
			return "", c.Name == "apply"
		}
		return appCompatExpressionTypeText(file, call, c.ReceiverNode), true
	case "with":
		if len(c.Args) == 0 {
			return "", true
		}
		return appCompatExpressionTypeText(file, call, c.Args[0]), true
	}
	return "", false
}

// appCompatReceiverIsResources reports whether the call's receiver is a Resources instance: a `resources` or
// getResources() accessor that no same-file declaration shadows with another type, or a Resources-typed variable.
func appCompatReceiverIsResources(ctx *api.Context, call lintCall) bool {
	if call.ReceiverNode == 0 {
		return false
	}
	file := ctx.File
	if javaSemanticCallReceiverTypeMatches(ctx, call.Idx, appCompatResourcesClass) {
		return true
	}
	text := appCompatReceiverText(call)
	last := text[strings.LastIndex(text, ".")+1:]
	if last == "resources" || last == "getResources()" {
		if typ := lintDeclaredTypeText(file, call.Idx, "resources"); typ != "" && !lintTypeRefMatches(file, typ, appCompatResourcesClass) {
			return false
		}
		return true
	}
	return lintReceiverIsType(ctx, call, appCompatResourcesClass)
}

// appCompatReceiverIsContext reports whether the call's receiver, explicit or implicit, is a Context.
func appCompatReceiverIsContext(ctx *api.Context, call lintCall) bool {
	if call.ReceiverNode == 0 || appCompatReceiverText(call) == "this" {
		return appCompatImplicitReceiverReaches(ctx, call.Idx, contextClass, appCompatContextKnown)
	}
	return receiverIsContext(ctx, call)
}

// appCompatCallAt returns the call at the dispatched node when it is a plain call (no trailing lambda) of name with
// the given number of arguments.
func appCompatCallAt(ctx *api.Context, name string, arities ...int) (lintCall, bool) {
	call, ok := lintCallAt(ctx.File, ctx.Idx)
	if !ok || call.Name != name || flatCallTrailingLambda(ctx.File, ctx.Idx) != 0 {
		return lintCall{}, false
	}
	for _, arity := range arities {
		if len(call.Args) == arity {
			return call, true
		}
	}
	return lintCall{}, false
}

func (r *UseSupportActionBarRule) check(ctx *api.Context) {
	call, ok := appCompatCallAt(ctx, "setActionBar", 1)
	if !ok || !appCompatDeclared(ctx) {
		return
	}
	if owner, resolved := appCompatResolvedOwner(ctx, call); resolved && owner != "android.app.Activity" {
		return
	}
	if !appCompatReceiverReaches(ctx, call, appCompatActivityClass, appCompatActivityKnown) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Use `AppCompatActivity.setSupportActionBar`"))
}

// appCompatMinSdk returns the minSdk the file is built against, or 0 when the build does not show it.
func appCompatMinSdk(ctx *api.Context) int {
	if levels := android.ResolveSDKLevels(ctx.File.Path); levels.MinSdk > 0 {
		return levels.MinSdk
	}
	return librarymodel.EnsureFacts(ctx.LibraryFacts).Profile.MinSdkVersion
}

func (r *UseCompatLoadingForColorStateListsRule) check(ctx *api.Context) {
	call, ok := appCompatCallAt(ctx, "getColorStateList", 1)
	if !ok || !appCompatDeclared(ctx) {
		return
	}
	if owner, resolved := appCompatResolvedOwner(ctx, call); resolved {
		if owner != appCompatResourcesClass {
			return
		}
	} else if !appCompatReceiverIsResources(ctx, call) {
		return
	}
	// Lint suggests ContextCompat from API 24 and AppCompatResources below it, where an unknown minSdk counts as 1.
	msg := "Use `AppCompatResources.getColorStateList()`"
	if appCompatMinSdk(ctx) >= 24 {
		msg = "Use `ContextCompat.getColorStateList()`"
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", msg))
}

func (r *UseCompatLoadingForDrawablesRule) check(ctx *api.Context) {
	call, ok := appCompatCallAt(ctx, "getDrawable", 1, 2)
	if !ok || !appCompatDeclared(ctx) {
		return
	}
	resources, context := false, false
	if owner, resolved := appCompatResolvedOwner(ctx, call); resolved {
		resources, context = owner == appCompatResourcesClass, owner == contextClass
	} else {
		resources = appCompatReceiverIsResources(ctx, call)
		context = !resources && len(call.Args) == 1 && appCompatReceiverIsContext(ctx, call)
	}
	switch {
	case resources:
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Use `ResourcesCompat.getDrawable()`"))
	case context && len(call.Args) == 1:
		ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", "Use `AppCompatResources.getDrawable()`"))
	}
}

// appCompatTintSetters maps each TextView setter, and the Kotlin property that calls it, to Lint's message.
var appCompatTintSetters = map[string]string{
	"setCompoundDrawableTintList": "Use `TextViewCompat.setCompoundDrawableTintList()`",
	"setCompoundDrawableTintMode": "Use `TextViewCompat.setCompoundDrawableTintMode()`",
	"compoundDrawableTintList":    "Use `TextViewCompat.setCompoundDrawableTintList()`",
	"compoundDrawableTintMode":    "Use `TextViewCompat.setCompoundDrawableTintMode()`",
}

func (r *UseCompatTextViewDrawableApisRule) check(ctx *api.Context) {
	if ctx.File.FlatType(ctx.Idx) == "assignment" {
		r.checkAssignment(ctx)
		return
	}
	call, ok := lintCallAt(ctx.File, ctx.Idx)
	if !ok || !strings.HasPrefix(call.Name, "set") || len(call.Args) != 1 || flatCallTrailingLambda(ctx.File, ctx.Idx) != 0 {
		return
	}
	msg, ok := appCompatTintSetters[call.Name]
	if !ok || !appCompatDeclared(ctx) {
		return
	}
	if owner, resolved := appCompatResolvedOwner(ctx, call); resolved {
		if owner != appCompatTextViewClass {
			return
		}
	} else if !appCompatReceiverReaches(ctx, call, appCompatTextViewClass, appCompatTextViewKnown) {
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", msg))
}

// checkAssignment reports `view.compoundDrawableTintList = x`, which calls the TextView setter.
func (r *UseCompatTextViewDrawableApisRule) checkAssignment(ctx *api.Context) {
	file := ctx.File
	lhs, ok := file.FlatFindChild(ctx.Idx, "directly_assignable_expression")
	if !ok {
		return
	}
	text := strings.TrimSpace(file.FlatNodeText(lhs))
	receiver, prop := "", text
	if dot := strings.LastIndex(text, "."); dot >= 0 {
		receiver, prop = strings.TrimSpace(text[:dot]), strings.TrimSpace(text[dot+1:])
	}
	msg, ok := appCompatTintSetters[prop]
	if !ok || strings.HasPrefix(prop, "set") || !appCompatDeclared(ctx) {
		return
	}
	receiver = strings.TrimSuffix(strings.TrimSuffix(receiver, "?"), "!!")
	switch {
	case receiver == "" || receiver == "this":
		if !appCompatImplicitReceiverReaches(ctx, ctx.Idx, appCompatTextViewClass, appCompatTextViewKnown) {
			return
		}
	case lintIdentOnly(receiver):
		typ := lintDeclaredTypeText(file, ctx.Idx, receiver)
		if typ == "" || !lintTypeNameReaches(ctx, typ, appCompatTextViewClass, appCompatTextViewKnown, 0) {
			return
		}
	default:
		return
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", msg))
}

func (r *UseSwitchCompatOrMaterialCodeRule) check(ctx *api.Context) {
	file := ctx.File
	if !appCompatDeclared(ctx) {
		return
	}
	const msg = "Use `SwitchCompat` from AppCompat or `MaterialSwitch` from Material library"
	switch file.FlatType(ctx.Idx) {
	case "class_declaration":
		for _, ref := range lintDirectSupertypeRefs(file, ctx.Idx) {
			if lintTypeRefMatches(file, ref.name, appCompatSwitchClass) {
				if at := appCompatSupertypeNode(file, ctx.Idx, ref.name); at != 0 {
					ctx.Emit(lintFindingAtNode(ctx, at, "", msg))
				}
			}
		}
		return
	case "property_declaration":
		vd, ok := file.FlatFindChild(ctx.Idx, "variable_declaration")
		if !ok {
			return
		}
		typ := lintChildTypeText(file, vd)
		if typ == "" {
			typ = appCompatInitializerTypeText(file, lintInitializerAfterEquals(file, ctx.Idx))
		}
		if !lintTypeRefMatches(file, typ, appCompatSwitchClass) {
			return
		}
	case "local_variable_declaration", "field_declaration":
		typ := lintChildTypeText(file, ctx.Idx)
		if typ == "var" {
			if declarator, ok := file.FlatFindChild(ctx.Idx, "variable_declarator"); ok {
				typ = lintConstructorCallType(file, lintInitializerAfterEquals(file, declarator))
			}
		}
		if !lintTypeRefMatches(file, typ, appCompatSwitchClass) {
			return
		}
	default: // parameter, class_parameter, formal_parameter
		if !lintTypeRefMatches(file, lintChildTypeText(file, ctx.Idx), appCompatSwitchClass) {
			return
		}
	}
	ctx.Emit(lintFindingAtNode(ctx, ctx.Idx, "", msg))
}

// appCompatInitializerTypeText names the type a Kotlin initializer produces: a constructor call, a cast, or a
// findViewById-style lookup with a type argument.
func appCompatInitializerTypeText(file *scanner.File, init uint32) string {
	init = flatUnwrapParenExpr(file, init)
	if init == 0 {
		return ""
	}
	switch file.FlatType(init) {
	case "as_expression":
		if typ := flatLastNamedChild(file, init); typ != 0 {
			return strings.TrimSuffix(strings.TrimSpace(file.FlatNodeText(typ)), "?")
		}
	case "call_expression":
		if call, ok := lintCallAt(file, init); ok && appCompatViewLookups[call.Name] {
			return appCompatCallTypeArgument(file, init)
		}
	}
	return lintConstructorCallType(file, init)
}

// appCompatSupertypeNode returns the supertype entry of the class declaration written as name.
func appCompatSupertypeNode(file *scanner.File, class uint32, name string) uint32 {
	var found uint32
	file.FlatWalkAllNodes(class, func(n uint32) {
		if found != 0 {
			return
		}
		switch file.FlatType(n) {
		case "class_body", "enum_class_body":
			return
		case "delegation_specifier", "superclass":
			text := strings.TrimSpace(file.FlatNodeText(n))
			text = strings.TrimSpace(strings.TrimPrefix(text, "extends"))
			if text == name || strings.HasPrefix(text, name+"(") || strings.HasPrefix(text, name+"<") {
				found = n
				if file.FlatType(n) == "superclass" {
					// Lint reports the type reference, after the `extends` keyword.
					for child := file.FlatFirstChild(n); child != 0; child = file.FlatNextSib(child) {
						if file.FlatIsNamed(child) {
							found = child
							break
						}
					}
				}
			}
		}
	})
	return found
}
