package rules

import (
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// serviceCastOracleConfirmed accepts when no oracle is available or it
// resolves the `.getSystemService(...)` call to `Context.getSystemService`
// (or `ContextCompat.getSystemService` from androidx.core). A
// project-local `getSystemService` resolves to a different FQN and
// is filtered out. When the oracle is available but cannot resolve the
// call, stay silent (do not treat as confirmed).
func serviceCastOracleConfirmed(lookup oracle.Lookup, file *scanner.File, call uint32) bool {
	if lookup == nil {
		return true
	}
	target := oracleLookupCallTargetFlat(lookup, file, call)
	if target == "" {
		return false
	}
	return target == "android.content.Context.getSystemService" ||
		target == "androidx.core.content.ContextCompat.getSystemService"
}

// showToastOracleConfirmed accepts when no oracle is available or it
// resolves `Toast.makeText(...)` to `android.widget.Toast.makeText`.
// A project-local `Toast` class with a `makeText` factory resolves
// to a different FQN and is filtered out. When the oracle is available
// but cannot resolve the call, stay silent (do not treat as confirmed).
func showToastOracleConfirmed(lookup oracle.Lookup, file *scanner.File, idx uint32) bool {
	if lookup == nil {
		return true
	}
	target := oracleLookupCallTargetFlat(lookup, file, idx)
	if target == "" {
		return false
	}
	return target == "android.widget.Toast.makeText"
}

// logCallOracleConfirmed accepts when no oracle is available or resolves
// the `.v/.d/.i/.w/.e/.wtf(...)` call to `android.util.Log.<level>`.
// A project-local `Log` class with the same level-named methods
// resolves to a different FQN and is filtered out. When the oracle is
// available but cannot resolve the call, stay silent (do not treat as confirmed).
// logTagLength counts the tag in UTF-16 code units, as Android does, reading escape sequences such as \u00e9 as one character.
func logTagLength(tag string) int {
	if unquoted, err := strconv.Unquote(`"` + tag + `"`); err == nil {
		tag = unquoted
	}
	return len(utf16.Encode([]rune(tag)))
}

func logCallOracleConfirmed(lookup oracle.Lookup, file *scanner.File, call uint32) bool {
	if lookup == nil {
		return true
	}
	target := oracleLookupCallTargetFlat(lookup, file, call)
	if target == "" {
		return false
	}
	return strings.HasPrefix(target, "android.util.Log.")
}

func registerAndroidSourceRules() {

	// --- from android_source.go ---
	{
		r := &FragmentConstructorRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "FragmentConstructor", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ValidFragment", Brief: "Fragment not instantiatable",
			Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatHasModifier(idx, "abstract") ||
					file.FlatHasModifier(idx, "sealed") {
					return
				}
				// Test fragments are built by the test, not re-instantiated by the framework.
				if scanner.IsTestFile(file.Path) || fragmentLaunchedFromInstance(file, extractIdentifierFlat(file, idx)) {
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
				// DI-annotated constructors are built by a custom FragmentFactory.
				if ctorState.hasParamCtor && !ctorState.hasNoArgCtor && !ctorState.hasInjectCtor {
					ctx.EmitAt(declLine(file, idx), declCol(file, idx),
						"Fragment subclass must have a default (no-arg) constructor for framework re-instantiation.")
				}
			},
		})
	}
	{
		r := &ServiceCastRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ServiceCast", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:  "ServiceCast", Brief: "Wrong system service casts",
			Category: ALCCorrectness, ALSeverity: ALSError, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"as_expression", "cast_expression"},
			Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceHigher, Implementation: r,
			Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Check: serviceCastCheck,
		})
	}
	{
		r := &ToastRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "ShowToast", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "ShowToast", Brief: "Toast created but not shown",
			Category: ALCCorrectness, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceHigh, Implementation: r,
			Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Check: func(ctx *api.Context) {
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
				// Gate on resolved call-target FQN; falls back to AST evidence only without an oracle.
				var oracleLookup oracle.Lookup
				if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
					oracleLookup = cr.Oracle()
				}
				if !showToastOracleConfirmed(oracleLookup, file, idx) {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					"Toast created but not shown: did you forget to call `show()`?")
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "method_invocation"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatType(idx) == "method_invocation" {
					if !getSignaturesJavaCallUsesDeprecatedFlag(file, idx) {
						return
					}
				} else if !getSignaturesCallUsesDeprecatedFlag(file, idx) {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceHigher, Implementation: r,
			Check: func(ctx *api.Context) {
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
				keyProj := file.FlatFirstChild(typeArgs)
				for keyProj != 0 && file.FlatType(keyProj) != "type_projection" {
					keyProj = file.FlatNextSib(keyProj)
				}
				if keyProj == 0 {
					return
				}
				keyUserType, _ := file.FlatFindChild(keyProj, "user_type")
				keyIdent := flatLastChildOfType(file, keyUserType, "type_identifier")
				if keyIdent == 0 {
					return
				}
				keyType := file.FlatNodeText(keyIdent)
				// A SparseArray is not a Map, so a map that escapes as one cannot be replaced.
				if hashMapEscapesAsMap(file, idx) {
					return
				}
				// SparseArray lookups are O(log n), so lint skips maps sized for more than 100 entries.
				if _, args := flatCallExpressionParts(file, idx); args != 0 && file.FlatNamedChildCount(args) == 1 {
					capacity := strings.ReplaceAll(strings.TrimSpace(file.FlatNodeText(file.FlatNamedChild(args, 0))), "_", "")
					if n, err := strconv.Atoi(capacity); err == nil && n > 100 {
						return
					}
				}
				valProj := file.FlatNextSib(keyProj)
				for valProj != 0 && file.FlatType(valProj) != "type_projection" {
					valProj = file.FlatNextSib(valProj)
				}
				var valueType string
				if valProj != 0 {
					valUserType, _ := file.FlatFindChild(valProj, "user_type")
					if valIdent := flatLastChildOfType(file, valUserType, "type_identifier"); valIdent != 0 {
						valueType = file.FlatNodeText(valIdent)
					}
				}
				var suggestion string
				switch keyType {
				case "Int", "Integer":
					switch valueType {
					case "Boolean":
						suggestion = "SparseBooleanArray"
					case "Int", "Integer":
						suggestion = "SparseIntArray"
					case "Long":
						suggestion = "SparseLongArray"
					default:
						suggestion = "SparseArray"
					}
				case "Long":
					suggestion = "LongSparseArray"
				default:
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "object_creation_expression"},
			Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence: api.ConfidenceHigher, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				switch file.FlatType(idx) {
				case "call_expression":
					checkUseValueOfKotlinCall(ctx, r, file, idx)
				case "object_creation_expression":
					checkUseValueOfJavaNew(ctx, r, file, idx)
				}
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Needs:             api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			NeedsLibraryFacts: true,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				// Android 7.0 (API 24) removed the 23 character tag limit.
				if projectMinSdk(ctx) >= 24 {
					return
				}
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
				if logTagLength(tag) <= 23 {
					return
				}
				// Gate on resolved call-target FQN; falls back to AST evidence only without an oracle.
				var oracleLookup oracle.Lookup
				if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
					oracleLookup = cr.Oracle()
				}
				if !logCallOracleConfirmed(oracleLookup, file, idx) {
					return
				}
				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					"Log tag \""+tag+"\" exceeds the 23 character limit.")
			},
		})
	}
	{
		r := &LogTagMismatchRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "LogTagMismatch", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:  "LogTagMismatch", Brief: "Mismatched Log tag",
			Category: ALCCorrectness, ALSeverity: ALSError, Priority: 5,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"class_declaration", "if_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatType(idx) == "if_expression" {
					var lookup oracle.Lookup
					if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
						lookup = cr.Oracle()
					}
					checkLogTagMismatch(ctx, lookup)
					return
				}
				if scanner.IsTestFile(file.Path) {
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
				var oracleLookup oracle.Lookup
				if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
					oracleLookup = cr.Oracle()
				}
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
					if expr == 0 || file.FlatType(expr) != "simple_identifier" || file.FlatNodeText(expr) != "TAG" {
						return
					}
					if !logCallOracleConfirmed(oracleLookup, file, call) {
						return
					}
					classUsesTagInLog = true
				})
				if !classUsesTagInLog {
					return
				}
				ctx.EmitAt(declLine(file, idx), declCol(file, idx),
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
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				if !nonInternationalizedSmsCallFlat(ctx.File, ctx.Idx) {
					return
				}
				ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
					"SMS destination should use international E.164 format starting with '+' to deliver correctly when roaming.")
			},
		})
	}
}

// checkLogTagMismatch reports log calls in the branches of `if (Log.isLoggable(tag, level))`
// whose tag or level differs from the isLoggable arguments.
func checkLogTagMismatch(ctx *api.Context, lookup oracle.Lookup) {
	file, ifExpr := ctx.File, ctx.Idx
	cond := file.FlatNamedChild(ifExpr, 0)
	if cond == 0 || file.FlatType(cond) != "call_expression" || !isReceiverNamed(file, cond, "Log") ||
		flatCallExpressionName(file, cond) != "isLoggable" {
		return
	}
	tag, level := logCallArgText(file, cond, 0), logCallArgText(file, cond, 1)
	for branch := file.FlatFirstChild(ifExpr); branch != 0; branch = file.FlatNextSib(branch) {
		if file.FlatType(branch) != "control_structure_body" {
			continue
		}
		for _, stmt := range logBranchStatements(file, branch) {
			method := flatCallExpressionName(file, stmt)
			if file.FlatType(stmt) != "call_expression" || !isReceiverNamed(file, stmt, "Log") ||
				!logMethodNames[method] || method == "wtf" || !logCallOracleConfirmed(lookup, file, stmt) {
				continue
			}
			for _, msg := range logTagMismatchMessages(tag, level, logCallArgText(file, stmt, 0), method) {
				ctx.EmitAt(callLine(file, stmt), callCol(file, stmt), msg)
			}
		}
	}
}
