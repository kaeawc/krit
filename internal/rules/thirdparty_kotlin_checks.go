package rules

// Ports of five single-node slack-lints checks (slackhq/slack-lints 0.11.1, slack-lint-checks/src/main/java/slack/lint/):
// NotNullOperatorDetector (AvoidUsingNotNullOperator), ExceptionMessageDetector (ExceptionMessage),
// NullableConcurrentHashMapDetector (NullableConcurrentHashMap), GuavaPreconditionsDetector's Kotlin issue
// (GuavaPreconditionsUsedInKotlin) and parcel/ParcelizeFunctionPropertyDetector (ParcelizeFunctionProperty).
// Each resolves the names it needs through the file's imports and abstains when a same-file declaration could
// shadow them.

import (
	"bytes"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

const (
	slackAvoidUsingNotNullOperator      = "AvoidUsingNotNullOperator"
	slackExceptionMessage               = "ExceptionMessage"
	slackNullableConcurrentHashMap      = "NullableConcurrentHashMap"
	slackGuavaPreconditionsUsedInKotlin = "GuavaPreconditionsUsedInKotlin"
	slackParcelizeFunctionProperty      = "ParcelizeFunctionProperty"

	fqcnConcurrentHashMap     = "java.util.concurrent.ConcurrentHashMap"
	fqcnGuavaPreconditions    = "com.google.common.base.Preconditions"
	fqcnParcelize             = "kotlinx.parcelize.Parcelize"
	slackParcelizeExplanation = "While technically (and surprisingly) supported by Parcelize, function types should not be used " +
		"in Parcelize classes. There are only limited conditions where it will work and it's usually a sign that you're " +
		"modeling your data wrong."
)

// --- AvoidUsingNotNullOperator ---

func notNullOperatorFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("!!"))
}

// checkAvoidUsingNotNullOperator reports every `!!`, at the operator, as NotNullOperatorDetector does.
func checkAvoidUsingNotNullOperator(ctx *api.Context) {
	for child := ctx.File.FlatFirstChild(ctx.Idx); child != 0; child = ctx.File.FlatNextSib(child) {
		if ctx.File.FlatType(child) == "!!" {
			ctx.EmitAtNode(child, "Avoid using the `!!` operator")
		}
	}
}

// --- ExceptionMessage ---

// exceptionMessageFunctions are the kotlin stdlib preconditions that take a lazyMessage.
var exceptionMessageFunctions = map[string]bool{"check": true, "checkNotNull": true, "require": true, "requireNotNull": true}

// checkExceptionMessage reports a kotlin check/checkNotNull/require/requireNotNull call without a lazyMessage.
func checkExceptionMessage(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	// The detector does not run on test sources.
	if scanner.IsTestFile(projectPath(ctx, file.Path)) {
		return
	}
	// `requireNotNull<T>(x) { ... }` parses as a call whose callee is the call with the arguments; the inner call
	// is the one checked, and the outer one carries its trailing lambda.
	if file.FlatType(file.FlatFirstChild(idx)) == "call_expression" {
		return
	}
	call, ok := lintCallAt(file, idx)
	if !ok || !exceptionMessageFunctions[call.Name] || !slackResolvesToKotlinStdlib(file, call) {
		return
	}
	if len(call.Args) >= 2 || slackLastLambdaArgument(file, idx) != 0 || slackHasOuterTrailingLambda(file, idx) {
		return
	}
	for _, label := range call.Labels {
		if label == "lazyMessage" {
			return
		}
	}
	if name := slackCalleeNameNode(file, idx); name != 0 {
		ctx.EmitAtNode(name, "Please specify a `lazyMessage` param for "+call.Name)
	}
}

// slackResolvesToKotlinStdlib reports whether an unqualified (or `kotlin.`-qualified) call resolves to the kotlin
// package function of that name: no import of the name from elsewhere and no same-file function that shadows it.
func slackResolvesToKotlinStdlib(file *scanner.File, call lintCall) bool {
	switch call.ReceiverText {
	case "kotlin":
		return true
	case "":
	default:
		return false
	}
	full, imported := lintImportsOf(file).bySimple[call.Name]
	if imported && full != "kotlin."+call.Name {
		return false
	}
	// Without an explicit import the stdlib function comes in as a default import, which a top-level function of
	// the file's package shadows.
	return !slackFunctionShadows(file, call.Idx, call.Name, imported)
}

// slackHasOuterTrailingLambda reports whether idx is the inner call of `f<T>(args) { ... }`.
func slackHasOuterTrailingLambda(file *scanner.File, idx uint32) bool {
	parent, _ := file.FlatParent(idx)
	if file.FlatType(parent) != "call_expression" || file.FlatFirstChild(parent) != idx {
		return false
	}
	return slackLastLambdaArgument(file, parent) != 0
}

// --- NullableConcurrentHashMap ---

func concurrentHashMapFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("ConcurrentHashMap"))
}

// checkNullableConcurrentHashMap reports a nullable key or value type argument of a ConcurrentHashMap variable type
// or constructor call.
func checkNullableConcurrentHashMap(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	var typ uint32
	switch file.FlatType(idx) {
	case "property_declaration":
		if varDecl, ok := file.FlatFindChild(idx, "variable_declaration"); ok {
			typ = slackDeclaredTypeAfterColon(file, varDecl)
		}
	case "class_parameter", "parameter":
		typ = slackDeclaredTypeAfterColon(file, idx)
	case "call_expression":
		callee := file.FlatFirstChild(idx)
		suffix, ok := file.FlatFindChild(idx, "call_suffix")
		if !ok || !lintTypeRefMatches(file, strings.Join(strings.Fields(file.FlatNodeText(callee)), ""), fqcnConcurrentHashMap) {
			return
		}
		if args, ok := file.FlatFindChild(suffix, "type_arguments"); ok {
			slackReportNullableTypeArguments(ctx, args)
		}
		return
	case "local_variable_declaration", "field_declaration", "formal_parameter", "object_creation_expression":
		for child := file.FlatFirstChild(idx); child != 0; child = file.FlatNextSib(child) {
			if file.FlatType(child) == "generic_type" {
				typ = child
				break
			}
		}
	}
	if typ == 0 {
		return
	}
	if file.FlatType(typ) == "nullable_type" {
		typ = file.FlatFirstChild(typ)
	}
	// A qualified Kotlin user_type splits into type_identifier children (java, util, ...); a Java generic_type starts
	// with its (scoped) type identifier.
	var parts []string
	for c := file.FlatFirstChild(typ); c != 0; c = file.FlatNextSib(c) {
		switch file.FlatType(c) {
		case "type_identifier", "scoped_type_identifier":
			parts = append(parts, strings.TrimSpace(file.FlatNodeText(c)))
		}
	}
	if len(parts) == 0 || !lintTypeRefMatches(file, strings.Join(parts, "."), fqcnConcurrentHashMap) {
		return
	}
	if args, ok := file.FlatFindChild(typ, "type_arguments"); ok {
		slackReportNullableTypeArguments(ctx, args)
	}
}

// slackReportNullableTypeArguments reports the nullable arguments of a two-argument type argument list: a Kotlin
// nullable type, or a Java type annotated with an annotation whose name ends in Nullable.
func slackReportNullableTypeArguments(ctx *api.Context, args uint32) {
	file := ctx.File
	var params []uint32
	for child := file.FlatFirstChild(args); child != 0; child = file.FlatNextSib(child) {
		if file.FlatIsNamed(child) {
			params = append(params, child)
		}
	}
	if len(params) != 2 {
		return
	}
	for i, param := range params {
		kind := "key"
		if i == 1 {
			kind = "value"
		}
		node := param
		if file.FlatType(node) == "type_projection" {
			node = file.FlatFirstChild(node)
		}
		nullable := file.FlatType(node) == "nullable_type"
		if file.FlatType(node) == "annotated_type" {
			file.FlatForEachChild(node, func(ann uint32) {
				switch file.FlatType(ann) {
				case "marker_annotation", "annotation":
					if a, ok := moshiReadAnnotation(file, ann); ok && (a.name == "Nullable" || strings.HasSuffix(a.name, ".Nullable")) {
						nullable = true
					}
				}
			})
		}
		if nullable {
			ctx.EmitAtNode(node, "ConcurrentHashMap should not use nullable "+kind+" types")
		}
	}
}

// --- GuavaPreconditionsUsedInKotlin ---

func guavaPreconditionsFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("com.google.common.base"))
}

// checkGuavaPreconditionsUsedInKotlin reports a Kotlin call to any com.google.common.base.Preconditions method.
func checkGuavaPreconditionsUsedInKotlin(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	call, ok := lintCallAt(file, idx)
	if !ok {
		return
	}
	if call.ReceiverText != "" {
		if !lintTypeRefMatches(file, call.ReceiverText, fqcnGuavaPreconditions) {
			return
		}
	} else if !slackStaticallyImports(file, idx, fqcnGuavaPreconditions, call.Name) {
		return
	}
	if name := slackCalleeNameNode(file, idx); name != 0 {
		ctx.EmitAtNode(name, "Kotlin precondition checks should use the Kotlin standard library checks")
	}
}

// slackStaticallyImports reports whether an unqualified call at idx to name resolves to a member of owner through an
// import of the member or of all of owner's members, with no function in scope shadowing it.
func slackStaticallyImports(file *scanner.File, idx uint32, owner, name string) bool {
	imp := lintImportsOf(file)
	if full, ok := imp.bySimple[name]; ok {
		return full == owner+"."+name && !slackFunctionShadows(file, idx, name, true)
	}
	for _, star := range imp.stars {
		if star == owner {
			return !slackFunctionShadows(file, idx, name, false)
		}
	}
	return false
}

// --- ParcelizeFunctionProperty ---

func parcelizeFileGate(file *scanner.File) bool {
	return bytes.Contains(file.Content, []byte("kotlinx.parcelize"))
}

// checkParcelizeFunctionProperty reports function-typed primary constructor parameters of a @Parcelize class.
// Lint skips a parameter annotated @IgnoredOnParcel, but that annotation targets properties only, so it never
// lands on the parameter and never exempts one; krit does not look for it either.
func checkParcelizeFunctionProperty(ctx *api.Context) {
	file, idx := ctx.File, ctx.Idx
	if moshiFindAnnotation(file, moshiAnnotationsOf(file, idx), fqcnParcelize) == nil {
		return
	}
	ctor, ok := file.FlatFindChild(idx, "primary_constructor")
	if !ok {
		return
	}
	file.FlatForEachChild(ctor, func(param uint32) {
		if file.FlatType(param) != "class_parameter" {
			return
		}
		typ := slackDeclaredTypeAfterColon(file, param)
		if typ != 0 && slackIsFunctionType(file, typ) {
			ctx.EmitAtNode(typ, slackParcelizeExplanation)
		}
	})
}

// slackIsFunctionType reports whether a Kotlin type is a function type (kotlin.jvm.functions.FunctionN on the JVM):
// `(A) -> B`, `suspend () -> B`, a nullable or parenthesized one, a kotlin FunctionN, or a same-file typealias of one.
func slackIsFunctionType(file *scanner.File, typ uint32) bool {
	for depth := 0; typ != 0 && depth < 8; depth++ {
		switch file.FlatType(typ) {
		case "function_type":
			return true
		case "type_modifiers":
			// `suspend () -> T`; a type annotation such as Parcelize's `@RawValue Foo` also parses as type_modifiers,
			// followed by the type itself.
			if strings.Contains(file.FlatNodeText(typ), "suspend") {
				return true
			}
			next := uint32(0)
			for sib := file.FlatNextSib(typ); sib != 0; sib = file.FlatNextSib(sib) {
				if file.FlatIsNamed(sib) {
					next = sib
					break
				}
			}
			typ = next
		case "nullable_type", "parenthesized_type", "type_projection":
			next := uint32(0)
			for child := file.FlatFirstChild(typ); child != 0; child = file.FlatNextSib(child) {
				if file.FlatIsNamed(child) {
					next = child
					break
				}
			}
			typ = next
		case "user_type":
			var parts []string
			for child := file.FlatFirstChild(typ); child != 0; child = file.FlatNextSib(child) {
				if file.FlatType(child) == "type_identifier" {
					parts = append(parts, file.FlatNodeText(child))
				}
			}
			text := strings.Join(parts, ".")
			if i := strings.LastIndexByte(text, '.'); i >= 0 && (text[:i] == "kotlin" || text[:i] == "kotlin.jvm.functions") {
				text = text[i+1:]
			}
			if strings.HasPrefix(text, "Function") && len(text) > len("Function") && strings.Trim(text[len("Function"):], "0123456789") == "" {
				return true
			}
			return slackFunctionTypeAlias(file, text)
		default:
			return false
		}
	}
	return false
}

// slackFunctionTypeAlias reports whether the file declares a typealias of name to a function type.
func slackFunctionTypeAlias(file *scanner.File, name string) bool {
	found := false
	file.FlatWalkNodes(0, "type_alias", func(alias uint32) {
		if found {
			return
		}
		var aliasName string
		var target uint32
		for child := file.FlatFirstChild(alias); child != 0; child = file.FlatNextSib(child) {
			switch file.FlatType(child) {
			case "type_identifier", "simple_identifier":
				if aliasName == "" {
					aliasName = file.FlatNodeText(child)
				}
			case "=":
				target = file.FlatNextSib(child)
			}
		}
		if aliasName == name && target != 0 && file.FlatType(target) != "user_type" {
			found = slackIsFunctionType(file, target)
		}
	})
	return found
}
