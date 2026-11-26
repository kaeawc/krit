package rules

import (
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ComposableNamingRule ports runtime-lint's ComposableNaming: a @Composable function that returns Unit should
// start with an uppercase letter, and one that returns a value should start with a lowercase letter. compose-lints'
// ComposeNamingUppercase and ComposeNamingLowercase report the same casing and are Lint aliases of this rule.
type ComposableNamingRule struct {
	FlatDispatchBase
	BaseRule
	// AllowedNames are anchored patterns for function names the rule never reports (compose-lints'
	// `allowed-composable-function-names` option).
	AllowedNames []*regexp.Regexp
}

// ComposableLambdaParameterNamingRule ports runtime-lint's ComposableLambdaParameterNaming: the only composable
// lambda parameter of a Unit @Composable function, when it takes no parameters, should be named `content`.
type ComposableLambdaParameterNamingRule struct {
	FlatDispatchBase
	BaseRule
}

// ComposableLambdaParameterPositionRule ports runtime-lint's ComposableLambdaParameterPosition: the only
// composable lambda parameter of a Unit @Composable function should be its last parameter.
type ComposableLambdaParameterPositionRule struct {
	FlatDispatchBase
	BaseRule
}

// CompositionLocalNamingRule ports runtime-lint's CompositionLocalNaming: CompositionLocal properties should be
// prefixed with `Local`.
type CompositionLocalNamingRule struct {
	FlatDispatchBase
	BaseRule
}

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *ComposableNamingRule) fileMentions() []string { return []string{"androidx.compose.runtime"} }

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *ComposableLambdaParameterNamingRule) fileMentions() []string {
	return []string{"androidx.compose.runtime"}
}

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *ComposableLambdaParameterPositionRule) fileMentions() []string {
	return []string{"androidx.compose.runtime"}
}

// fileMentions gates the rule to files that can reference the Compose runtime.
func (r *CompositionLocalNamingRule) fileMentions() []string {
	return []string{"androidx.compose.runtime"}
}

func (r *ComposableNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposableNaming", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"Expression-bodied functions without a declared return type are skipped: their return type needs type inference.",
			"Only Kotlin declarations are checked.",
			"Under the compose-lints aliases, krit also reports a lowercase Unit composable with a receiver (ComposeNamingUppercase exempts it) and casing on expression-bodied functions with a provable return type (compose-lints skips every expression body), and it skips overrides, which compose-lints reports.",
		},
		Options: []api.ConfigOption{
			api.StringListOption(api.StringListOptionSpec[ComposableNamingRule]{
				Name: "allowedComposableFunctionNames",
				Description: "Regexes matched against the whole function name; matching composables are not reported " +
					"(compose-lints `allowed-composable-function-names`).",
				Apply: func(r *ComposableNamingRule, v []string) {
					r.AllowedNames = r.AllowedNames[:0]
					for _, p := range v {
						if re := api.CompileAnchoredPattern(r.RuleName, "allowedComposableFunctionNames", p); re != nil {
							r.AllowedNames = append(r.AllowedNames, re)
						}
					}
				},
			}),
		}}
}

func (r *ComposableLambdaParameterNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposableLambdaParameterNaming", RuleSet: "compose", DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated}
}

func (r *ComposableLambdaParameterPositionRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ComposableLambdaParameterPosition", RuleSet: "compose", DefaultActive: false,
		OptInReason: api.OptInReasonOpinionated}
}

func (r *CompositionLocalNamingRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "CompositionLocalNaming", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"A property counts as a CompositionLocal only when its declared type or its compositionLocalOf-style initializer proves it.",
		}}
}

func registerAndroidxComposeRuntimeNamingRules() {
	kotlin := []scanner.Language{scanner.LangKotlin}
	naming := &ComposableNamingRule{BaseRule: BaseRule{RuleName: "ComposableNaming", RuleSetName: "compose", Sev: "warning",
		Desc: "Detects @Composable functions whose name casing does not match their return type (Unit: uppercase, value: lowercase)."}}
	registerAcrRule(naming.BaseRule, naming, []string{"function_declaration", "method_declaration"},
		[]scanner.Language{scanner.LangKotlin, scanner.LangJava}, naming.check)

	lambdaNaming := &ComposableLambdaParameterNamingRule{BaseRule: BaseRule{RuleName: "ComposableLambdaParameterNaming",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects the only composable lambda parameter of a @Composable function not being named content."}}
	registerAcrRule(lambdaNaming.BaseRule, lambdaNaming, []string{"function_declaration"}, kotlin, lambdaNaming.check)

	lambdaPosition := &ComposableLambdaParameterPositionRule{BaseRule: BaseRule{RuleName: "ComposableLambdaParameterPosition",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects the only composable lambda parameter of a @Composable function not being its last parameter."}}
	registerAcrRule(lambdaPosition.BaseRule, lambdaPosition, []string{"function_declaration"}, kotlin, lambdaPosition.check)

	local := &CompositionLocalNamingRule{BaseRule: BaseRule{RuleName: "CompositionLocalNaming", RuleSetName: "compose",
		Sev: "warning", Desc: "Detects CompositionLocal properties whose name is not prefixed with Local."}}
	registerAcrRule(local.BaseRule, local, []string{"property_declaration", "field_declaration"},
		[]scanner.Language{scanner.LangKotlin, scanner.LangJava}, local.check)
}

func (r *ComposableNamingRule) check(ctx *api.Context) {
	file, fn := ctx.File, ctx.Idx
	if file.Language == scanner.LangJava {
		r.checkJava(ctx)
		return
	}
	if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) ||
		acrFunctionHasModifier(file, fn, "operator") || acrFunctionHasModifier(file, fn, "override") {
		return
	}
	nameNode := acrNameNode(file, fn)
	if nameNode == 0 {
		return
	}
	name := strings.Trim(file.FlatNodeText(nameNode), "`")
	if name == "" || r.nameAllowed(name) {
		return
	}
	upper := acrFirstRuneIsUpper(name)
	switch acrFunctionReturnKind(file, fn) {
	case acrReturnUnit:
		if !upper {
			ctx.EmitAtNode(nameNode, "Composable functions that return Unit should start with an uppercase letter")
		}
	case acrReturnValue:
		if upper {
			ctx.EmitAtNode(nameNode, "Composable functions with a return type should start with a lowercase letter")
		}
	}
}

// checkJava applies the rule to a Java method annotated @Composable; `void` methods count as returning Unit.
// Methods marked @Override are skipped, as Lint skips methods with super methods.
func (r *ComposableNamingRule) checkJava(ctx *api.Context) {
	file, method := ctx.File, ctx.Idx
	mods, _ := file.FlatFindChild(method, "modifiers")
	if mods == 0 {
		return
	}
	composable := false
	for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
		t := file.FlatType(child)
		if t != "marker_annotation" && t != "annotation" {
			continue
		}
		name := strings.TrimPrefix(file.FlatChildTextOrEmpty(child, "identifier"), "@")
		if name == "" {
			name = strings.TrimPrefix(file.FlatChildTextOrEmpty(child, "scoped_identifier"), "@")
		}
		switch {
		case name == "Override" || name == "java.lang.Override":
			return
		case name == acrComposableFQN, name == acrComposableShort && acrJavaImports(file, acrRuntimePackage, name):
			composable = true
		}
	}
	nameNode, _ := file.FlatFindChild(method, "identifier")
	if !composable || nameNode == 0 {
		return
	}
	if r.nameAllowed(file.FlatNodeText(nameNode)) {
		return
	}
	returnsUnit := file.FlatHasChildOfType(method, "void_type")
	upper := acrFirstRuneIsUpper(file.FlatNodeText(nameNode))
	switch {
	case returnsUnit && !upper:
		ctx.EmitAtNode(nameNode, "Composable functions that return Unit should start with an uppercase letter")
	case !returnsUnit && upper:
		ctx.EmitAtNode(nameNode, "Composable functions with a return type should start with a lowercase letter")
	}
}

// nameAllowed reports whether a configured allowedComposableFunctionNames pattern matches the whole name.
func (r *ComposableNamingRule) nameAllowed(name string) bool {
	for _, re := range r.AllowedNames {
		if re.MatchString(name) {
			return true
		}
	}
	return false
}

// acrComposableLambdaParameter returns the single composable function-type parameter of a Unit @Composable
// function, the parameter's function type and whether it is the last parameter. ok is false when the function
// is out of scope or has zero or several such parameters.
func acrComposableLambdaParameter(file *scanner.File, fn uint32) (param, fnType uint32, last, ok bool) {
	if !acrFunctionIsComposable(file, fn) || acrIsLocalDeclaration(file, fn) ||
		acrFunctionReturnKind(file, fn) != acrReturnUnit {
		return 0, 0, false, false
	}
	params, found := file.FlatFindChild(fn, "function_value_parameters")
	if !found {
		return 0, 0, false, false
	}
	var lastParam uint32
	count := 0
	for child := file.FlatFirstChild(params); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "parameter" {
			continue
		}
		lastParam = child
		if ft := acrComposableFunctionType(file, child); ft != 0 {
			count++
			param, fnType = child, ft
		}
	}
	if count != 1 {
		return 0, 0, false, false
	}
	return param, fnType, param == lastParam, true
}

// acrComposableFunctionType returns the function_type of a parameter typed `@Composable () -> Unit`,
// `@Composable (() -> Unit)?` or `(@Composable () -> Unit)?`, or 0.
func acrComposableFunctionType(file *scanner.File, param uint32) uint32 {
	composable := acrHasComposableAnnotation(file, param)
	for child := file.FlatFirstChild(param); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "function_type":
			if composable {
				return child
			}
			return 0
		case "nullable_type":
			inner := file.FlatFirstChild(child)
			if inner == 0 || file.FlatType(inner) != "parenthesized_type" {
				return 0
			}
			innerComposable := acrHasComposableAnnotation(file, inner)
			ft, _ := file.FlatFindChild(inner, "function_type")
			if ft != 0 && (composable || innerComposable) {
				return ft
			}
			return 0
		case "user_type", "parenthesized_type":
			return 0
		}
	}
	return 0
}

func (r *ComposableLambdaParameterNamingRule) check(ctx *api.Context) {
	file := ctx.File
	param, fnType, _, ok := acrComposableLambdaParameter(file, ctx.Idx)
	if !ok {
		return
	}
	nameNode, _ := file.FlatFindChild(param, "simple_identifier")
	if nameNode == 0 || strings.Trim(file.FlatNodeText(nameNode), "`") == "content" {
		return
	}
	if typeParams, found := file.FlatFindChild(fnType, "function_type_parameters"); found &&
		file.FlatNamedChildCount(typeParams) > 0 {
		return
	}
	ctx.EmitAtNode(nameNode, "Composable lambda parameter should be named `content`")
}

func (r *ComposableLambdaParameterPositionRule) check(ctx *api.Context) {
	file := ctx.File
	param, _, last, ok := acrComposableLambdaParameter(file, ctx.Idx)
	if !ok || last {
		return
	}
	if nameNode, found := file.FlatFindChild(param, "simple_identifier"); found {
		ctx.EmitAtNode(nameNode,
			"Composable lambda parameter should be the last parameter so it can be used as a trailing lambda")
	}
}

// acrCompositionLocalFactories are the androidx.compose.runtime functions that return a ProvidableCompositionLocal.
var acrCompositionLocalFactories = map[string]bool{
	"compositionLocalOf": true, "staticCompositionLocalOf": true, "compositionLocalWithComputedDefaultOf": true,
}

// acrCompositionLocalTypes are the androidx.compose.runtime CompositionLocal class and its subclasses.
var acrCompositionLocalTypes = []string{"CompositionLocal", "ProvidableCompositionLocal"}

func (r *CompositionLocalNamingRule) check(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	var nameNode uint32
	isLocal := false
	if file.Language == scanner.LangJava {
		nameNode, isLocal = acrJavaCompositionLocalField(file, idx)
	} else {
		nameNode, isLocal = acrKotlinCompositionLocalProperty(file, idx)
	}
	if nameNode == 0 || !isLocal {
		return
	}
	name := strings.Trim(file.FlatNodeText(nameNode), "`")
	if len(name) >= 5 && strings.EqualFold(name[:5], "Local") {
		return
	}
	ctx.EmitAtNode(nameNode, "CompositionLocal properties should be prefixed with `Local`")
}

// acrKotlinCompositionLocalProperty returns the name node of a non-local property and whether its declared type
// or initializer proves it is a CompositionLocal.
func acrKotlinCompositionLocalProperty(file *scanner.File, prop uint32) (uint32, bool) {
	if acrIsLocalDeclaration(file, prop) {
		return 0, false
	}
	varDecl, ok := file.FlatFindChild(prop, "variable_declaration")
	if !ok {
		return 0, false
	}
	nameNode, _ := file.FlatFindChild(varDecl, "simple_identifier")
	if nameNode == 0 {
		return 0, false
	}
	for child := file.FlatFirstChild(varDecl); child != 0; child = file.FlatNextSib(child) {
		if t := file.FlatType(child); t == "user_type" || t == "nullable_type" {
			return nameNode, acrTypeIsCompositionLocal(file, child)
		}
	}
	sawEquals := false
	for child := file.FlatFirstChild(prop); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "=":
			sawEquals = true
		case "call_expression":
			if sawEquals {
				return nameNode, acrCallIsCompositionLocalFactory(file, child)
			}
		case "property_delegate", "getter":
			return nameNode, false
		}
	}
	return nameNode, false
}

func acrTypeIsCompositionLocal(file *scanner.File, typeNode uint32) bool {
	if file.FlatType(typeNode) == "nullable_type" {
		typeNode, _ = file.FlatFindChild(typeNode, "user_type")
	}
	for _, name := range acrCompositionLocalTypes {
		if acrTypeIsRuntime(file, typeNode, name) {
			return true
		}
	}
	return false
}

func acrCallIsCompositionLocalFactory(file *scanner.File, call uint32) bool {
	callee := acrCalleeExpression(file, call)
	switch file.FlatType(callee) {
	case "simple_identifier":
		name := file.FlatNodeText(callee)
		return acrCompositionLocalFactories[name] && acrCalleeResolvesTo(file, call, acrRuntimePackage, name)
	case "navigation_expression":
		text := strings.Join(strings.Fields(file.FlatNodeText(callee)), "")
		if pkg, name, ok := strings.Cut(text, acrRuntimePackage+"."); ok && pkg == "" {
			return acrCompositionLocalFactories[name]
		}
	}
	return false
}

// acrJavaCompositionLocalField returns the declarator name of a Java field and whether its declared type is a
// CompositionLocal.
func acrJavaCompositionLocalField(file *scanner.File, field uint32) (uint32, bool) {
	declarator, ok := file.FlatFindChild(field, "variable_declarator")
	if !ok {
		return 0, false
	}
	nameNode, _ := file.FlatFindChild(declarator, "identifier")
	if nameNode == 0 {
		return 0, false
	}
	for child := file.FlatFirstChild(field); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "type_identifier", "scoped_type_identifier", "generic_type":
			text := file.FlatNodeText(child)
			if lt := strings.IndexByte(text, '<'); lt >= 0 {
				text = text[:lt]
			}
			for _, name := range acrCompositionLocalTypes {
				if text == acrRuntimePackage+"."+name || (text == name && acrJavaImports(file, acrRuntimePackage, name)) {
					return nameNode, true
				}
			}
			return nameNode, false
		}
	}
	return nameNode, false
}
