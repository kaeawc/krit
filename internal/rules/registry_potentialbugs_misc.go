package rules

import (
	"fmt"
	"github.com/kaeawc/krit/internal/oracle"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
	"strings"
)

func registerPotentialbugsMiscRules() {

	// --- from potentialbugs_misc.go ---
	{
		r := &DeprecationRule{BaseRule: BaseRule{RuleName: "Deprecation", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects usage of deprecated functions, classes, or properties."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "navigation_expression", "user_type"}, Confidence: 0.75, OriginalV1: r,
			Needs:             v2.NeedsTypeInfo,
			OracleCallTargets: &v2.OracleCallTargetFilter{AnnotatedIdentifiers: []string{"Deprecated"}},
			// Narrow by the "Deprecated" token — captures @Deprecated,
			// @kotlin.Deprecated, @java.lang.Deprecated, and any import
			// header that aliases kotlin.Deprecated. Inherited deprecations
			// from base types that live in files without the token are a
			// documented trade-off (see issue #306).
			Oracle: &v2.OracleFilter{Identifiers: []string{"Deprecated"}},
			// Uses LookupCallTargetAnnotations (annotations embedded directly in
			// call-resolution data) so no declaration extraction is needed.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				var oracleLookup oracle.Lookup
				if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
					oracleLookup = cr.Oracle()
				}
				line := file.FlatRow(idx) + 1
				col := file.FlatCol(idx) + 1
				nodeType := file.FlatType(idx)

				// Skip nodes inside import statements if configured
				if r.ExcludeImportStatements && file.FlatHasAncestorOfType(idx, "import_header") {
					return
				}

				// Avoid double-reporting: if this navigation_expression is the direct
				// child of a call_expression, let the call_expression visit handle it.
				if nodeType == "navigation_expression" {
					if parent, ok := file.FlatParent(idx); ok && file.FlatType(parent) == "call_expression" {
						return
					}
				}

				// For user_type nodes, skip if inside an annotation (the @Deprecated itself)
				if nodeType == "user_type" && file.FlatHasAncestorOfType(idx, "annotation") {
					return
				}

				// Extract the name being referenced
				name := flatDeprecationRefName(file, idx)
				if name == "" {
					return
				}

				// 1. Oracle-based check: annotations are embedded in the call-target
				// resolution entry, so no member-declaration extraction is needed.
				if oracleLookup != nil {
					annotations := oracleLookupCallTargetAnnotationsFlat(oracleLookup, file, idx)
					for _, ann := range annotations {
						if ann == "kotlin.Deprecated" || ann == "java.lang.Deprecated" {
							ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
								fmt.Sprintf("'%s' is deprecated.", name))
							return
						}
					}
				}

				// 2. Source-level check: look for @Deprecated on declarations in this
				// file. The index is cached per-file and guarded by cacheMu since
				// dispatch runs one rule instance across parallel file goroutines.
				r.cacheMu.Lock()
				r.ensureDeprecatedIndex(file)
				info := r.deprecatedInfos[name]
				r.cacheMu.Unlock()
				if info == nil {
					return
				}

				msg := fmt.Sprintf("'%s' is deprecated.", name)
				if info.level != "" {
					msg = fmt.Sprintf("'%s' is deprecated (level=%s).", name, info.level)
				}
				if info.message != "" {
					msg = fmt.Sprintf("'%s' is deprecated: %s", name, info.message)
					if info.level != "" {
						msg = fmt.Sprintf("'%s' is deprecated (level=%s): %s", name, info.level, info.message)
					}
				}

				f := r.Finding(file, line, col, msg)

				// If replaceWith is available, offer an auto-fix
				if info.replaceWith != "" && (nodeType == "call_expression" || nodeType == "navigation_expression") {
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(idx)),
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: info.replaceWith,
					}
				}

				ctx.Emit(f)
			},
		})
	}
	{
		r := &HasPlatformTypeRule{BaseRule: BaseRule{RuleName: "HasPlatformType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects public functions with expression bodies that lack an explicit return type, risking platform type exposure from Java interop."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if isTestFile(file.Path) {
					return
				}
				if file.FlatHasModifier(idx, "private") || file.FlatHasModifier(idx, "internal") ||
					file.FlatHasModifier(idx, "protected") || file.FlatHasModifier(idx, "override") {
					return
				}

				body, _ := file.FlatFindChild(idx, "function_body")
				if body == 0 {
					return
				}
				bodyText := strings.TrimSpace(file.FlatNodeText(body))
				if !strings.HasPrefix(bodyText, "=") || strings.HasPrefix(bodyText, "= {") {
					return
				}

				exprText := strings.TrimSpace(strings.TrimPrefix(bodyText, "="))
				if exprText == "Unit" {
					return
				}

				params, _ := file.FlatFindChild(idx, "function_value_parameters")
				bodyStart := file.FlatStartByte(body)
				hasReturnType := false
				file.FlatForEachChild(idx, func(child uint32) {
					if hasReturnType {
						return
					}
					switch file.FlatType(child) {
					case ":":
						hasReturnType = true
					case "user_type", "nullable_type", "type_identifier":
						if params != 0 && file.FlatStartByte(child) > file.FlatEndByte(params) &&
							(body == 0 || file.FlatStartByte(child) < bodyStart) {
							hasReturnType = true
						}
					}
				})
				if hasReturnType {
					return
				}

				line := file.FlatRow(idx) + 1
				for _, prefix := range []string{"java.", "javax.", "android.", "Java", "Javax"} {
					if strings.Contains(exprText, prefix) {
						ctx.EmitAt(line, 1,
							"Public function with expression body should have an explicit return type to avoid platform types.")
						return
					}
				}
			},
		})
	}
	{
		r := &IgnoredReturnValueRule{BaseRule: BaseRule{RuleName: "IgnoredReturnValue", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects discarded return values from functional operations or @CheckReturnValue-annotated functions."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Needs: v2.NeedsTypeInfo,
			OracleCallTargets: &v2.OracleCallTargetFilter{
				DiscardedOnly:        true,
				AnnotatedIdentifiers: []string{"CheckReturnValue", "CheckResult", "CanIgnoreReturnValue"},
			},
			// Keep oracle input bounded to files that mention detekt's default
			// must-use return-type families or return-value annotations. Return
			// type checks use LookupExpression through the composite resolver;
			// call-target resolution remains narrowed to annotated declarations.
			Oracle: &v2.OracleFilter{Identifiers: []string{"Sequence", "Flow", "Stream", "Function", "->", "CheckReturnValue", "CheckResult", "CanIgnoreReturnValue"}},
			// Uses expression-level type data and LookupCallTargetAnnotations
			// (annotations embedded directly in call-resolution data), so no
			// declaration extraction is needed.
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				var oracleLookup oracle.Lookup
				if cr, ok := ctx.Resolver.(*oracle.CompositeResolver); ok {
					oracleLookup = cr.Oracle()
				}
				funcName := flatCallExpressionName(file, idx)
				if funcName == "" {
					return
				}

				line := file.FlatRow(idx) + 1
				col := file.FlatCol(idx) + 1
				isKnownFunctionalOp := functionalOps[funcName]
				if stringListContains(r.IgnoreFunctionCall, funcName) {
					return
				}

				// Check if this call's result is discarded (not used as expression)
				if flatIsUsedAsExpression(file, idx) {
					return
				}

				var oracleReturnType *typeinfer.ResolvedType
				if oracleLookup != nil {
					oracleReturnType = oracleLookup.LookupExpression(file.Path, line, col)
					if oracleReturnType != nil && oracleReturnType.Kind != typeinfer.TypeUnknown &&
						ignoredReturnValueTypeIsUnitOrNothing(oracleReturnType) {
						return
					}
					annotations := ignoredReturnValueMergedOracleAnnotations(oracleLookup, file, idx, line, col)
					hasCheck, hasIgnore := ignoredReturnValueAnnotationEvidence(annotations, r.ReturnValueAnnotations, r.IgnoreReturnValueAnnotations)
					if hasIgnore {
						return
					}
					if hasCheck {
						ctx.EmitAt(line, col,
							fmt.Sprintf("Return value of '%s' is ignored. The function is annotated with @CheckReturnValue.", funcName))
						return
					}
					if oracleReturnType != nil && oracleReturnType.Kind != typeinfer.TypeUnknown {
						if ignoredReturnValueTypeMatches(oracleReturnType, r.ReturnValueTypes, r.RestrictToConfig) {
							ctx.EmitAt(line, col,
								fmt.Sprintf("Return value of '%s' is ignored. The call returns %s.", funcName, ignoredReturnValueTypeName(oracleReturnType)))
							return
						}
						return
					}
				}

				if ctx.Resolver != nil {
					rt := ctx.Resolver.ResolveFlatNode(idx, file)
					if ignoredReturnValueTypeMatches(rt, r.ReturnValueTypes, r.RestrictToConfig) {
						ctx.EmitAt(line, col,
							fmt.Sprintf("Return value of '%s' is ignored. The call returns %s.", funcName, ignoredReturnValueTypeName(rt)))
						return
					}
					if oracleLookup != nil && ignoredReturnValueTypeKnown(rt) {
						return
					}
					if r.RestrictToConfig {
						return
					}
				}

				if !r.RestrictToConfig && isKnownFunctionalOp {
					// No resolver/oracle evidence matched. Keep the historical
					// tree-sitter-only heuristic for no-KAA runs, but only after
					// resolved Unit/Nothing, type, and annotation checks had a
					// chance to classify the call.
					ctx.EmitAt(line, col,
						"Return value of functional operation is ignored. If this is intentional, assign to a variable or use 'also' instead.")
				}
			},
		})
	}
	{
		r := &ImplicitDefaultLocaleRule{BaseRule: BaseRule{RuleName: "ImplicitDefaultLocale", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects locale-sensitive string methods called without an explicit Locale argument."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Needs: v2.NeedsTypeInfo,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if strings.HasSuffix(file.Path, ".gradle.kts") {
					return
				}
				if strings.HasSuffix(file.Path, "Table.kt") || strings.HasSuffix(file.Path, "Tables.kt") ||
					strings.HasSuffix(file.Path, "Dao.kt") || strings.HasSuffix(file.Path, "DAO.kt") {
					return
				}
				navExpr, args := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}

				methodName := flatNavigationExpressionLastIdentifier(file, navExpr)
				if methodName == "" {
					return
				}
				var receiverIdx uint32
				if file.FlatNamedChildCount(navExpr) > 0 {
					receiverIdx = file.FlatNamedChild(navExpr, 0)
				}

				firstArg, argCount := flatValueArgumentStats(file, args)

				if implicitLocaleMethods[methodName] {
					if argCount != 0 {
						return
					}
					if receiverIdx != 0 {
						recvText := file.FlatNodeText(receiverIdx)
						if containsAsciiInvariantIdentifier(recvText) {
							return
						}
					}
					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("'%s()' called without explicit Locale. Use '%s(Locale.ROOT)' or '%s(Locale.getDefault())' to be explicit.", methodName, methodName, methodName))
					return
				}

				if methodName == "format" {
					if receiverIdx == 0 {
						return
					}
					receiverType := file.FlatType(receiverIdx)

					isStringLiteral := receiverType == "string_literal" || receiverType == "line_string_literal" || receiverType == "multi_line_string_literal"
					receiverText := ""
					isStringCompanion := false
					if !isStringLiteral {
						receiverText = file.FlatNodeText(receiverIdx)
						isStringCompanion = receiverText == "String"
					}

					if !isStringCompanion && !isStringLiteral {
						if receiverText == "String" {
							isStringLiteral = true
						}
						if !isStringLiteral {
							return
						}
					}
					if receiverText == "" {
						receiverText = file.FlatNodeText(receiverIdx)
					}
					if target, ok := implicitDefaultLocaleOracleCallTarget(ctx, idx); ok {
						if !implicitDefaultLocaleIsStringFormatTarget(target) {
							return
						}
					} else if isStringLiteral && fileDeclaresStringFormatExtension(file) {
						return
					}

					if isExplicitLocaleArgFlat(file, firstArg) {
						return
					}

					if isStringLiteral && isLocaleInsensitiveFormat(receiverText) {
						return
					}
					if isStringCompanion && firstArg != 0 {
						argText := file.FlatNodeText(firstArg)
						if strings.HasPrefix(strings.TrimSpace(argText), "\"") {
							if isLocaleInsensitiveFormat(argText) {
								return
							}
						}
					}

					ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						fmt.Sprintf("'%s' uses implicit default locale for string formatting. Pass Locale explicitly, e.g. Locale.ROOT or Locale.US.", receiverText+".format(...)"))
				}
			},
		})
	}
	{
		r := &LocaleDefaultForCurrencyRule{BaseRule: BaseRule{RuleName: "LocaleDefaultForCurrency", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects Currency.getInstance(Locale.getDefault()) in money-related classes where currency should come from business data."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.75, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				className := enclosingClassNameFlat(file, idx)
				if !isCurrencyCarrierClassName(className) {
					return
				}

				navExpr, args := flatCallExpressionParts(file, idx)
				if navExpr == 0 || args == 0 {
					return
				}

				if flatNavigationExpressionLastIdentifier(file, navExpr) != "getInstance" {
					return
				}

				receiver := file.FlatNamedChild(navExpr, 0)
				if receiver == 0 {
					return
				}
				receiverText := compactKotlinExpr(file.FlatNodeText(receiver))
				if receiverText != "Currency" && receiverText != "java.util.Currency" {
					return
				}

				firstArg, argCount := flatValueArgumentStats(file, args)
				if argCount != 1 || firstArg == 0 {
					return
				}

				argText := compactKotlinExpr(file.FlatNodeText(firstArg))
				if argText != "Locale.getDefault()" && argText != "java.util.Locale.getDefault()" {
					return
				}

				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Class '%s' derives currency from Locale.getDefault(); use order or transaction currency data instead.", className))
			},
		})
	}
}
