package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"strings"
)

func registerAndroidSourceRules() {

	// --- from android_source.go ---
	{
		r := &FragmentConstructorRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "FragmentConstructor", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ValidFragment", Brief: "Fragment not instantiatable",
			Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatHasModifier(idx, "abstract") ||
					file.FlatHasModifier(idx, "sealed") {
					return
				}
				isFragment := false
				for i := 0; i < file.FlatChildCount(idx); i++ {
					child := file.FlatChild(idx, i)
					if file.FlatType(child) != "delegation_specifier" {
						continue
					}
					typeName := viewConstructorSupertypeNameFlat(file, child)
					if typeName == "" {
						continue
					}
					for _, base := range fragmentSuperclasses {
						if typeName == base {
							isFragment = true
							break
						}
					}
					if isFragment {
						break
					}
				}
				if !isFragment {
					return
				}
				ctorState := fragmentConstructorStateFlat(file, idx)
				if ctorState.hasParamCtor && !ctorState.hasNoArgCtor {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Fragment subclass must have a default (no-arg) constructor for framework re-instantiation.")
				}
			},
		})
	}
	{
		r := &ServiceCastRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ServiceCast", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ServiceCast", Brief: "Wrong system service cast",
			Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"as_expression"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// First named child is the source expression; we want
				// `getSystemService(...)` calls only.
				call := file.FlatFirstChild(idx)
				for call != 0 && !file.FlatIsNamed(call) {
					call = file.FlatNextSib(call)
				}
				if call == 0 || file.FlatType(call) != "call_expression" {
					return
				}
				if flatCallExpressionName(file, call) != "getSystemService" {
					return
				}
				// Pull the service constant's final identifier. Accepts both
				// `CONNECTIVITY_SERVICE` (simple_identifier) and
				// `Context.CONNECTIVITY_SERVICE` (navigation_expression).
				args := flatCallKeyArguments(file, call)
				firstArg := flatPositionalValueArgument(file, args, 0)
				expr := flatValueArgumentExpression(file, firstArg)
				svcConst := ""
				switch file.FlatType(expr) {
				case "simple_identifier":
					svcConst = file.FlatNodeText(expr)
				case "navigation_expression":
					svcConst = flatNavigationExpressionLastIdentifier(file, expr)
				}
				expectedType, ok := serviceCastMap[svcConst]
				if !ok {
					return
				}
				// Target type of the cast — last type_identifier of user_type.
				castType := ""
				if userType, ok := file.FlatFindChild(idx, "user_type"); ok {
					if ident := flatLastChildOfType(file, userType, "type_identifier"); ident != 0 {
						castType = file.FlatNodeText(ident)
					}
				}
				if castType == "" || castType == expectedType {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"Service cast mismatch: "+svcConst+" should be cast to "+expectedType+", not "+castType+".")
			},
		})
	}
	{
		r := &ToastRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ShowToast", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ShowToast", Brief: "Toast created but not shown",
			Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.85, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "makeText" {
					return
				}
				if !isReceiverNamed(file, idx, "Toast") {
					return
				}
				if toastMakeTextIsShown(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"Toast.makeText() called without .show(). The toast will not be displayed.")
			},
		})
	}
	{
		r := &GetSignaturesRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "GetSignatures", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "PackageManagerGetSignatures", Brief: "Using deprecated GET_SIGNATURES",
			Category: ALCSecurity, ALSeverity: ALSWarning, Priority: 8,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !getSignaturesCallUsesDeprecatedFlag(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"GET_SIGNATURES is deprecated and can be spoofed. Use GET_SIGNING_CERTIFICATES (API 28+) instead.")
			},
		})
	}
	{
		r := &SparseArrayRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "UseSparseArrays", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "UseSparseArrays", Brief: "HashMap can be replaced with SparseArray",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 4,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "HashMap" {
					return
				}
				// First type argument's user_type's identifier is the key type.
				suffix, _ := file.FlatFindChild(idx, "call_suffix")
				if suffix == 0 {
					return
				}
				typeArgs, _ := file.FlatFindChild(suffix, "type_arguments")
				if typeArgs == 0 {
					return
				}
				proj := file.FlatFirstChild(typeArgs)
				for proj != 0 && file.FlatType(proj) != "type_projection" {
					proj = file.FlatNextSib(proj)
				}
				if proj == 0 {
					return
				}
				userType, _ := file.FlatFindChild(proj, "user_type")
				ident := flatLastChildOfType(file, userType, "type_identifier")
				if ident == 0 {
					return
				}
				keyType := file.FlatNodeText(ident)
				var suggestion string
				switch keyType {
				case "Int", "Integer":
					suggestion = "SparseArray"
				case "Long":
					suggestion = "LongSparseArray"
				default:
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"Use "+suggestion+" instead of HashMap<"+keyType+", ...> for better performance on Android.")
			},
		})
	}
	{
		r := &UseValueOfRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "UseValueOf", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "UseValueOf", Brief: "Should use valueOf instead of constructor",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 4,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.9, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				// Only fire on calls whose callee is a bare boxed-primitive
				// identifier — `Integer(42)`, not `Integer.valueOf(42)` (where
				// flatCallExpressionName returns `valueOf`) nor any qualified
				// form. This is the one case AOSP Lint flags.
				typeName := flatCallExpressionName(file, idx)
				if !boxedPrimitiveConstructors[typeName] {
					return
				}
				// Skip if the callee is actually qualified (e.g. `a.Integer(42)`).
				// For unqualified calls the first named child is a bare
				// simple_identifier; for qualified calls it's a navigation_expression.
				firstNamed := file.FlatFirstChild(idx)
				for firstNamed != 0 && !file.FlatIsNamed(firstNamed) {
					firstNamed = file.FlatNextSib(firstNamed)
				}
				if firstNamed == 0 || file.FlatType(firstNamed) != "simple_identifier" {
					return
				}
				// Require exactly one positional argument — `Integer()` zero-arg
				// and `Integer(x, radix)` multi-arg overloads are not the
				// single-value boxing we want to flag.
				args := flatCallKeyArguments(file, idx)
				argCount := 0
				for a := file.FlatFirstChild(args); a != 0; a = file.FlatNextSib(a) {
					if file.FlatType(a) == "value_argument" {
						argCount++
					}
				}
				if argCount != 1 {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"Use "+typeName+".valueOf() instead of new "+typeName+"() constructor for better performance.")
			},
		})
	}
	{
		r := &LogTagLengthRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "LongLogTag", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:  "LongLogTag", Brief: "Log tag exceeds 23 characters",
			Category: ALCCorrectness, ALSeverity: ALSError, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if !isReceiverNamed(file, idx, "Log") || !logMethodNames[flatCallExpressionName(file, idx)] {
					return
				}
				args := flatCallKeyArguments(file, idx)
				firstArg := flatPositionalValueArgument(file, args, 0)
				expr := flatValueArgumentExpression(file, firstArg)
				tag, ok := resolveLogTagStringValue(file, expr)
				if !ok {
					return
				}
				if len(tag) > 23 {
					ctx.EmitAt(file.FlatRow(idx)+1, 1,
						"Log tag \""+tag+"\" exceeds the 23 character limit.")
				}
			},
		})
	}
	{
		r := &LogTagMismatchRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "LogTagMismatch", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "LogTagMismatch", Brief: "Mismatched Log tag",
			Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				className := extractIdentifierFlat(file, idx)
				if className == "" {
					return
				}
				tagValue := findDirectCompanionTagFlat(file, idx)
				if tagValue == "" {
					return
				}
				if tagValue == className {
					return
				}
				if len(className) > 23 && strings.HasPrefix(className, tagValue) {
					return
				}
				// The class must actually contain a `Log.{v|d|i|w|e|s}(TAG, …)`
				// call that would expose the mismatch. Walk call_expression
				// nodes in the class body instead of regex-scanning the whole
				// class text — the regex version matched `Log.d(TAG, x)`
				// inside string literals or comments in unrelated methods.
				classUsesTagInLog := false
				file.FlatWalkNodes(idx, "call_expression", func(call uint32) {
					if classUsesTagInLog {
						return
					}
					if !isReceiverNamed(file, call, "Log") {
						return
					}
					if !logMethodNames[flatCallExpressionName(file, call)] {
						return
					}
					args := flatCallKeyArguments(file, call)
					firstArg := flatPositionalValueArgument(file, args, 0)
					expr := flatValueArgumentExpression(file, firstArg)
					if expr != 0 && file.FlatType(expr) == "simple_identifier" && file.FlatNodeText(expr) == "TAG" {
						classUsesTagInLog = true
					}
				})
				if !classUsesTagInLog {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1,
					"Log TAG value \""+tagValue+"\" doesn't match class name \""+className+"\".")
			},
		})
	}
	{
		r := &NonInternationalizedSmsRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "NonInternationalizedSms", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "UnlocalizedSms", Brief: "SMS with non-i18n considerations",
			Category: ALCI18N, ALSeverity: ALSWarning, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
				if !nonInternationalizedSmsCallFlat(ctx.File, ctx.Idx) {
					return
				}
				ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
					"SMS destination should use international E.164 format starting with '+' to deliver correctly when roaming.")
			},
		})
	}
}
