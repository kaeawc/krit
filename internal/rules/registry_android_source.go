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

const toastDurationMessage = "Expected duration `Toast.LENGTH_SHORT` or `Toast.LENGTH_LONG`, a custom duration value is not supported"

// toastLiteralDuration returns the duration argument of a three-argument
// makeText call when it is an integer literal (parentheses skipped), as
// Lint's ToastDetector reports it; a negative number is a prefix expression,
// not a literal.
func toastLiteralDuration(file *scanner.File, call uint32) uint32 {
	_, args := flatCallExpressionParts(file, call)
	if args == 0 {
		return 0
	}
	var values []uint32
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) == "value_argument" {
			values = append(values, arg)
		}
	}
	if len(values) != 3 {
		return 0
	}
	expr := flatUnwrapParenExpr(file, flatValueArgumentExpression(file, values[2]))
	switch file.FlatType(expr) {
	case "integer_literal", "long_literal", "hex_literal", "bin_literal":
		return expr
	}
	return 0
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
			NodeTypes: []string{"class_declaration", "object_literal"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: fragmentConstructorCheck,
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
				// A Toast imported from another package is not android.widget.Toast.
				if imp, ok := fileFactsCache().Imports(file).Aliases["Toast"]; ok && imp != "android.widget.Toast" {
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
				// Lint reports a literal duration whether or not the toast is shown.
				if duration := toastLiteralDuration(file, idx); duration != 0 {
					ctx.EmitAtNode(duration, toastDurationMessage)
				}
				if toastMakeTextIsShown(file, idx) {
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
			NodeTypes: []string{"call_expression", "object_creation_expression"},
			Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava}, Confidence: api.ConfidenceHigher, Implementation: r,
			Check: sparseArrayCheck,
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
				tagIndex, isTagMethod := longLogTagArgIndex[flatCallExpressionName(file, idx)]
				if !isTagMethod || !isReceiverNamed(file, idx, "Log") {
					return
				}
				args := flatCallKeyArguments(file, idx)
				tagArg := flatPositionalValueArgument(file, args, tagIndex)
				expr := flatValueArgumentExpression(file, tagArg)
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
				// Without the oracle, as in a file that does not compile, the imports decide what Log is.
				if oracleLookup == nil && !logReceiverImportsAndroidLog(file) {
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

// fileDeclaresTypeVisibleAt reports whether a same-named declaration visible at at shadows platformType.
// Type aliases that resolve to platformType are not shadows.
func fileDeclaresTypeVisibleAt(file *scanner.File, name, platformType string, at uint32) bool {
	if file == nil || at == 0 {
		return false
	}
	found := false
	file.FlatWalkAllNodes(0, func(decl uint32) {
		if found {
			return
		}
		switch file.FlatType(decl) {
		case "class_declaration", "object_declaration", "companion_object", "type_alias", "interface_declaration", "enum_declaration", "record_declaration":
			if lintDeclTypeName(file, decl) != name || !typeDeclarationVisibleAt(file, decl, at) {
				return
			}
			if file.FlatType(decl) == "type_alias" && typeAliasResolvesTo(file, decl, platformType) {
				return
			}
			found = true
		}
	})
	return found
}

const maxSameFileTypeAliasDepth = 32

func typeAliasResolvesTo(file *scanner.File, alias uint32, platformType string) bool {
	seen := make(map[uint32]struct{})
	for depth := 0; alias != 0 && depth < maxSameFileTypeAliasDepth; depth++ {
		if _, ok := seen[alias]; ok {
			return false
		}
		seen[alias] = struct{}{}

		target := file.FlatNamedChild(alias, file.FlatNamedChildCount(alias)-1)
		if target == 0 {
			return false
		}
		targetName := lintTrimGenerics(file.FlatNodeText(target))
		if lintTypeRefMatches(file, targetName, platformType) {
			return true
		}
		if strings.Contains(targetName, ".") {
			return false
		}

		var next uint32
		file.FlatWalkAllNodes(0, func(decl uint32) {
			if next != 0 || file.FlatType(decl) != "type_alias" ||
				lintDeclTypeName(file, decl) != targetName || !typeDeclarationVisibleAt(file, decl, alias) {
				return
			}
			next = decl
		})
		alias = next
	}
	return false
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
	// Without an oracle, a Log class or object declared in the file is not android.util.Log.
	if lookup == nil && fileDeclaresTypeVisibleAt(file, "Log", "android.util.Log", ifExpr) {
		return
	}
	tag, level := logCallArgText(file, cond, 0), logLevelConstantName(file, logCallArgText(file, cond, 1))
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
