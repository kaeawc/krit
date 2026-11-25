package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// The four checks of runtime-retain-lint's RetainDetector, which inspects every
// androidx.compose.runtime.retain.retain call: its result type (the calculation's) and its key arguments.

// RetainUnitTypeRule ports RetainUnitType: a retain call whose calculation returns Unit.
type RetainUnitTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// RetainRememberObserverRule ports RetainRememberObserver: a retained value implementing RememberObserver but not
// RetainObserver.
type RetainRememberObserverRule struct {
	FlatDispatchBase
	BaseRule
}

// RetainingDoNotRetainTypeRule ports RetainingDoNotRetainType: a retained value or key whose class (or a direct
// supertype) is annotated @DoNotRetain.
type RetainingDoNotRetainTypeRule struct {
	FlatDispatchBase
	BaseRule
}

// RetainLeaksContextRule ports RetainLeaksContext: a retained value or key that is a Context, View, Window,
// Resources, Fragment or Dialog.
type RetainLeaksContextRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports medium confidence: the calculation's type is proven from same-file declarations.
func (r *RetainUnitTypeRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: the retained type is proven from same-file declarations.
func (r *RetainRememberObserverRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: the retained type is proven from same-file declarations.
func (r *RetainingDoNotRetainTypeRule) Confidence() float64 { return api.ConfidenceMedium }

// Confidence reports medium confidence: the retained type is proven from declarations and known Android types.
func (r *RetainLeaksContextRule) Confidence() float64 { return api.ConfidenceMedium }

const acrRetainPackage = acrRuntimePackage + ".retain"

// fileMentions gates the rule to files that can reference runtime-retain.
func (r *RetainUnitTypeRule) fileMentions() []string { return []string{acrRetainPackage} }

// fileMentions gates the rule to files that can reference runtime-retain.
func (r *RetainRememberObserverRule) fileMentions() []string { return []string{acrRetainPackage} }

// fileMentions gates the rule to files that can reference runtime-retain.
func (r *RetainingDoNotRetainTypeRule) fileMentions() []string { return []string{acrRetainPackage} }

// fileMentions gates the rule to files that can reference runtime-retain.
func (r *RetainLeaksContextRule) fileMentions() []string { return []string{acrRetainPackage} }

var acrRetainLimitations = []string{
	"The retained value's type is proven from same-file classes, declared types of parameters and properties, Android constructors and LocalContext/LocalView; values typed only by inference across files, and containers such as listOf<Context>(), are not checked.",
}

func (r *RetainUnitTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RetainUnitType", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"The calculation's Unit type is proven only from `retain<Unit>`, assignments and loops, `Unit`, println, same-file Unit functions and `() -> Unit` values.",
		}}
}

func (r *RetainRememberObserverRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RetainRememberObserver", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: acrRetainLimitations}
}

func (r *RetainingDoNotRetainTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RetainingDoNotRetainType", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: append([]string{"Only same-file @DoNotRetain classes are recognized."}, acrRetainLimitations...)}
}

func (r *RetainLeaksContextRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RetainLeaksContext", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: acrRetainLimitations}
}

func registerAndroidxComposeRuntimeRetainRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	unit := &RetainUnitTypeRule{BaseRule: BaseRule{RuleName: "RetainUnitType", RuleSetName: "compose", Sev: "error",
		Desc: "Detects retain calls whose calculation returns Unit; use SideEffect or RetainedEffect instead."}}
	registerAcrRule(unit.BaseRule, unit, []string{"call_expression"}, kotlin, unit.check)

	observer := &RetainRememberObserverRule{BaseRule: BaseRule{RuleName: "RetainRememberObserver", RuleSetName: "compose",
		Sev: "error", Desc: "Detects retained values implementing RememberObserver but not RetainObserver."}}
	registerAcrRule(observer.BaseRule, observer, []string{"call_expression"}, kotlin, observer.check)

	marked := &RetainingDoNotRetainTypeRule{BaseRule: BaseRule{RuleName: "RetainingDoNotRetainType", RuleSetName: "compose",
		Sev: "error", Desc: "Detects retained values or keys whose type is annotated @DoNotRetain."}}
	registerAcrRule(marked.BaseRule, marked, []string{"call_expression"}, kotlin, marked.check)

	leaks := &RetainLeaksContextRule{BaseRule: BaseRule{RuleName: "RetainLeaksContext", RuleSetName: "compose", Sev: "error",
		Desc: "Detects retained values or keys that are a Context, View, Window, Resources, Fragment or Dialog, which leak a Context."}}
	registerAcrRule(leaks.BaseRule, leaks, []string{"call_expression"}, kotlin, leaks.check)
}

// acrRetainCall reports whether call is androidx.compose.runtime.retain.retain.
func acrRetainCall(file *scanner.File, call uint32) bool {
	_, ok := acrRuntimeCall(file, call, acrRetainPackage, "retain")
	return ok
}

// acrRetainedValue returns the retained value expression (the calculation's result), or 0.
func acrRetainedValue(file *scanner.File, call uint32) uint32 {
	if acrHasTrailingLambda(file, call) {
		return acrTrailingLambdaResult(file, call)
	}
	return 0
}

// acrRetainKeys returns retain's key arguments (positional arguments before the calculation).
func acrRetainKeys(file *scanner.File, call uint32) []uint32 {
	args := acrCallArguments(file, call)
	var keys []uint32
	for i, a := range args {
		if a.label != "" || a.spread || (i == len(args)-1 && !acrHasTrailingLambda(file, call)) {
			continue
		}
		keys = append(keys, a.expr)
	}
	return keys
}

// acrRetainType is what the source proves about a retained value's or key's type.
type acrRetainType struct {
	canonical string // canonical text as Lint prints it (FQN)
	class     uint32 // the same-file class, or 0
}

// acrRetainTypeOf proves the type of a retained value or key.
func acrRetainTypeOf(file *scanner.File, expr uint32, depth int) acrRetainType {
	expr = acrUnwrapParens(file, expr)
	if expr == 0 || depth > 4 {
		return acrRetainType{}
	}
	switch file.FlatType(expr) {
	case "call_expression":
		if acrIsInnerCallOfTrailingLambda(file, expr) {
			return acrRetainType{}
		}
		name, _, hasReceiver := acrCallCallee(file, expr)
		if hasReceiver || name == "" {
			return acrRetainType{}
		}
		if class := flatFindSameFileClassLikeDeclaration(file, name); class != 0 && file.FlatType(class) == "class_declaration" {
			return acrRetainType{canonical: acrCanonicalClassName(file, class), class: class}
		}
		if fqn := acrImportedFQN(file, name); acrAndroidLeakTypes[fqn] {
			return acrRetainType{canonical: fqn}
		}
	case "navigation_expression":
		text := strings.Join(strings.Fields(file.FlatNodeText(expr)), "")
		for local, fqn := range acrCompositionLocalLeaks {
			if text == local+".current" && acrNameResolvesTo(file, "androidx.compose.ui.platform", local) {
				return acrRetainType{canonical: fqn}
			}
		}
		if decl := acrExprDeclaration(file, expr); decl != 0 {
			return acrDeclRetainType(file, decl, depth)
		}
	case "simple_identifier":
		if decl := acrValueDeclaration(file, expr, file.FlatNodeText(expr)); decl != 0 {
			return acrDeclRetainType(file, decl, depth)
		}
	}
	return acrRetainType{}
}

func acrDeclRetainType(file *scanner.File, decl uint32, depth int) acrRetainType {
	t := acrDeclaredType(file, decl)
	if t == 0 {
		return acrRetainTypeOf(file, acrInitializer(file, decl), depth+1)
	}
	if file.FlatType(t) != "user_type" {
		return acrRetainType{}
	}
	name := acrTypeNodeName(file, t)
	if file.FlatNodeText(t) != name {
		// Generic types print with their arguments; only plain class types are reproduced.
		return acrRetainType{}
	}
	if class := acrSameFileClass(file, t); class != 0 {
		return acrRetainType{canonical: acrCanonicalClassName(file, class), class: class}
	}
	if strings.Contains(name, ".") {
		return acrRetainType{canonical: name}
	}
	if fqn := acrImportedFQN(file, name); fqn != "" {
		return acrRetainType{canonical: fqn}
	}
	return acrRetainType{}
}

// acrImportedFQN returns the FQN an explicit import binds the simple name to, or "".
func acrImportedFQN(file *scanner.File, name string) string {
	if facts := fileFactsCache().Imports(file); facts != nil {
		return facts.Aliases[name]
	}
	return ""
}

// acrCanonicalClassName returns a same-file class's FQN (package, enclosing classes, name).
func acrCanonicalClassName(file *scanner.File, class uint32) string {
	parts := []string{acrDeclarationName(file, class)}
	for p, ok := file.FlatParent(class); ok; p, ok = file.FlatParent(p) {
		if t := file.FlatType(p); t == "class_declaration" || t == "object_declaration" {
			parts = append([]string{acrDeclarationName(file, p)}, parts...)
		}
	}
	if pkg := filePackageName(file); pkg != "" {
		parts = append([]string{pkg}, parts...)
	}
	return strings.Join(parts, ".")
}

// acrCompositionLocalLeaks maps androidx.compose.ui.platform CompositionLocals to the type of their value.
var acrCompositionLocalLeaks = map[string]string{
	"LocalContext": "android.content.Context", "LocalView": "android.view.View",
}

// acrAndroidLeakTypes are Android and AndroidX classes that are, or extend, the types RetainDetector treats as
// Context leaks: Context, View, Window, Resources, Fragment and Dialog.
var acrAndroidLeakTypes = acrNames(
	"android.content.Context", "android.content.ContextWrapper", "android.view.ContextThemeWrapper",
	"android.app.Activity", "android.app.Application", "android.app.Service", "android.app.IntentService",
	"androidx.activity.ComponentActivity", "androidx.core.app.ComponentActivity",
	"androidx.fragment.app.FragmentActivity", "androidx.appcompat.app.AppCompatActivity",
	"android.view.View", "android.view.ViewGroup", "android.view.SurfaceView", "android.view.TextureView",
	"android.widget.TextView", "android.widget.EditText", "android.widget.Button", "android.widget.ImageView",
	"android.widget.ImageButton", "android.widget.LinearLayout", "android.widget.FrameLayout",
	"android.widget.RelativeLayout", "android.widget.ScrollView", "android.widget.ListView",
	"android.widget.ProgressBar", "android.widget.CheckBox", "android.widget.Spinner", "android.webkit.WebView",
	"androidx.compose.ui.platform.ComposeView", "android.view.Window", "android.content.res.Resources",
	"android.app.Dialog", "android.app.AlertDialog", "androidx.appcompat.app.AlertDialog",
	"androidx.appcompat.app.AppCompatDialog", "android.app.Fragment", "androidx.fragment.app.Fragment",
	"androidx.fragment.app.DialogFragment",
)

// acrRetainLeaks reports whether a proven type leaks a Context: a known Android type, or a same-file class
// extending one.
func acrRetainLeaks(file *scanner.File, t acrRetainType) bool {
	if acrAndroidLeakTypes[t.canonical] {
		return true
	}
	return t.class != 0 && acrClassExtendsAndroidLeak(file, t.class, 0)
}

func acrClassExtendsAndroidLeak(file *scanner.File, class uint32, depth int) bool {
	if depth > 4 {
		return false
	}
	for child := file.FlatFirstChild(class); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		typeNode := acrDelegationSpecifierType(file, child)
		name := acrTypeNodeName(file, typeNode)
		if acrAndroidLeakTypes[name] || acrAndroidLeakTypes[acrImportedFQN(file, name)] {
			return true
		}
		if super := acrSameFileClass(file, typeNode); super != 0 && super != class && acrClassExtendsAndroidLeak(file, super, depth+1) {
			return true
		}
	}
	return false
}

// acrDoNotRetainExplanation returns whether a same-file class (or a direct same-file supertype) is annotated
// @DoNotRetain, and the annotation's explanation.
func acrDoNotRetainExplanation(file *scanner.File, class uint32) (string, bool) {
	if explanation, ok := acrClassDoNotRetain(file, class); ok {
		return explanation, true
	}
	for child := file.FlatFirstChild(class); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "delegation_specifier" {
			continue
		}
		if super := acrSameFileClass(file, acrDelegationSpecifierType(file, child)); super != 0 && super != class {
			if explanation, ok := acrClassDoNotRetain(file, super); ok {
				return explanation, true
			}
		}
	}
	return "", false
}

func acrClassDoNotRetain(file *scanner.File, class uint32) (string, bool) {
	mods, ok := file.FlatFindChild(class, "modifiers")
	if !ok {
		return "", false
	}
	for m := file.FlatFirstChild(mods); m != 0; m = file.FlatNextSib(m) {
		if file.FlatType(m) != "annotation" || !acrAnnotationMatches(file, m, acrAnnotationPackage, "DoNotRetain", "") {
			continue
		}
		explanation := ""
		if ctor, ok := file.FlatFindChild(m, "constructor_invocation"); ok {
			file.FlatWalkNodes(ctor, "value_argument", func(arg uint32) {
				if explanation == "" {
					if label := flatValueArgumentLabel(file, arg); label == "" || label == "explanation" {
						explanation = flatStringLiteralExprContent(file, flatValueArgumentExpression(file, arg))
					}
				}
			})
		}
		return explanation, true
	}
	return "", false
}

func (r *RetainUnitTypeRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	if !acrRetainCall(file, call) || !acrCalculationReturnsUnit(file, call, "calculation") {
		return
	}
	_, nameNode, _ := acrCallCallee(file, call)
	ctx.EmitAtNode(nameNode, "`retain` calls must not return `Unit`.")
}

func (r *RetainRememberObserverRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	if !acrRetainCall(file, call) {
		return
	}
	t := acrRetainTypeOf(file, acrRetainedValue(file, call), 0)
	if t.class == 0 || !acrClassSupertypeIs(file, t.class, 0, acrRuntimePackage, "RememberObserver") ||
		acrClassSupertypeIs(file, t.class, 0, acrRetainPackage, "RetainObserver") {
		return
	}
	_, nameNode, _ := acrCallCallee(file, call)
	ctx.EmitAtNode(nameNode, "Declared retained type `"+t.canonical+"` implements `RememberObserver` but not `RetainObserver`.")
}

func (r *RetainingDoNotRetainTypeRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	if !acrRetainCall(file, call) || !acrFileMentions(file, "DoNotRetain") {
		return
	}
	_, nameNode, _ := acrCallCallee(file, call)
	report := func(prefix string, t acrRetainType) {
		if t.class == 0 {
			return
		}
		if explanation, ok := acrDoNotRetainExplanation(file, t.class); ok {
			msg := prefix + t.canonical + " is annotated as `@DoNotRetain`"
			if explanation != "" {
				msg += ": " + explanation
			}
			ctx.EmitAtNode(nameNode, msg)
		}
	}
	report("", acrRetainTypeOf(file, acrRetainedValue(file, call), 0))
	for _, key := range acrRetainKeys(file, call) {
		report("Key type ", acrRetainTypeOf(file, key, 0))
	}
}

func (r *RetainLeaksContextRule) check(ctx *api.Context) {
	file, call := ctx.File, ctx.Idx
	if !acrRetainCall(file, call) {
		return
	}
	_, nameNode, _ := acrCallCallee(file, call)
	if t := acrRetainTypeOf(file, acrRetainedValue(file, call), 0); t.canonical != "" && acrRetainLeaks(file, t) {
		ctx.EmitAtNode(nameNode, "Retaining "+t.canonical+" will leak a Context reference.")
	}
	for _, key := range acrRetainKeys(file, call) {
		if t := acrRetainTypeOf(file, key, 0); t.canonical != "" && acrRetainLeaks(file, t) {
			ctx.EmitAtNode(nameNode, "Retaining a key of type "+t.canonical+" will leak a Context reference.")
		}
	}
}
