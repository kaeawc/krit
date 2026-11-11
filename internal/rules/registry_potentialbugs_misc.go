package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/oracle"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func registerPotentialbugsMiscRules() {

	// --- from potentialbugs_misc.go ---
	{
		r := &DeprecationRule{BaseRule: BaseRule{RuleName: "Deprecation", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects usage of deprecated functions, classes, or properties."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "navigation_expression", "user_type"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets | api.NeedsOracleExprAnnotations | api.NeedsOracleDiagnostics,
			// No Oracle file filter: the compiler's DEPRECATION verdict is
			// needed wherever a deprecated symbol is referenced, and that is
			// usually a file that never mentions "Deprecated" — a call to a
			// deprecated library API, an inherited member, or a type declared
			// elsewhere.
			Check: func(ctx *api.Context) {
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

				// 0. Compiler verdict: project K2's DEPRECATION when the oracle
				// collected it. It is overload-exact and covers the reference
				// kinds the paths below miss.
				if projectDiagnostic(ctx, deprecationProjection(name)) {
					return
				}

				// The compiler analyzed this file and reported nothing here, so the name-based paths would only add false positives.
				if compilerAnalyzedFile(ctx) {
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

				// 2. Source-level check: look for @Deprecated on declarations in
				// this file. The index lives on the shared filefacts cache, so
				// concurrent file-scan goroutines reuse the same compute.
				info := deprecatedDeclIndex(file)[name]
				if info == nil {
					return
				}

				f := r.Finding(file, line, col, deprecationMessage(name, info))
				f.Fix = deprecationReplaceWithFix(file, idx, info)
				ctx.Emit(f)
			},
		})
	}
	{
		r := &HasPlatformTypeRule{BaseRule: BaseRule{RuleName: "HasPlatformType", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects public functions with expression bodies that lack an explicit return type, risking platform type exposure from Java interop."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if scanner.IsTestFile(file.Path) {
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

				exprText = hasPlatformTypeCodeText(file, body)
				if exprText == "" {
					return
				}

				line, col := declLine(file, idx), declCol(file, idx)
				for _, prefix := range []string{"java.", "javax.", "android.", "Java", "Javax"} {
					if containsIdentifierStart(exprText, prefix) {
						ctx.EmitAt(line, col, extractIdentifierFlat(file, idx)+" has implicit platform type. Type must be declared explicitly.")
						return
					}
				}
			},
		})
	}
	{
		r := &IgnoredReturnValueRule{BaseRule: BaseRule{RuleName: "IgnoredReturnValue", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects discarded return values from functional operations or @CheckReturnValue-annotated functions."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets | api.NeedsOracleExprAnnotations | api.NeedsOracleExprType,
			// Keep oracle input bounded to files that mention configured
			// must-use return-type families or return-value annotations. Return
			// type checks use LookupExpression through the composite resolver.
			Oracle: &api.OracleFilter{Identifiers: []string{"Sequence", "Flow", "Stream", "Function", "->", "CheckReturnValue", "CheckResult", "CanIgnoreReturnValue"}},
			Check: func(ctx *api.Context) {
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
				if ignoredReturnValueInPlaceMutationCalls[funcName] {
					return
				}
				if stringListContains(r.IgnoreFunctionCall, funcName) {
					return
				}
				// Assertion helpers return their argument for chaining; dropping it is the normal use.
				if ignoredReturnValueIsTestAssertion(oracleLookup, file, idx, funcName) {
					return
				}

				// Check if this call's result is discarded (not used as expression)
				if flatIsUsedAsExpression(file, idx) {
					return
				}
				if ignoredReturnValueIsEmitterSignalCall(file, idx, funcName) {
					return
				}
				if ignoredReturnValueInsideVerificationContext(file, idx) {
					return
				}
				if ignoredReturnValueCallIsValueAssignmentReceiver(file, idx) {
					return
				}
				// Android lint's id for this check is CheckResult.
				if callSuppressedByLintID(file, idx, "CheckResult") {
					return
				}
				// A call that is the receiver of a larger expression (member read, indexed or property assignment) is used.
				if ignoredReturnValueCallIsQualifier(file, idx) {
					return
				}
				if ignoredReturnValueIsBuilderCallbackCall(file, idx, funcName) {
					return
				}
				if ignoredReturnValueIsTestSetupSideEffectCall(file, idx, funcName) {
					return
				}
				if ignoredReturnValueIsSideEffectFoldDispatch(file, idx, funcName) {
					return
				}
				// Invoking a function-typed value yields its return value; the call's own type is not the function.
				if ignoredReturnValueIsFunctionValueInvoke(file, idx) {
					return
				}
				// The parser joins a statement with a following line that starts with `(`; the type then belongs to the first call.
				if ignoredReturnValueChainJoinsAcrossLines(file, idx) {
					return
				}
				// Invoking a function-typed value is not a must-use call, whatever type the oracle reports for it.
				if funcName == "invoke" && ignoredReturnValueTargetIsFunctionInvoke(oracleLookup, file, idx) {
					return
				}
				if ignoredReturnValueLambdaReturnsFromEnclosingFunction(file, idx) {
					return
				}
				// subscribeWith returns its argument, which the caller already holds.
				if funcName == "subscribeWith" && ignoredReturnValueHasPlainArgument(file, idx) {
					return
				}
				if ignoredReturnValueHelperResultUsedElsewhere(file, idx, funcName) {
					return
				}

				var oracleReturnType *typeinfer.ResolvedType
				if oracleLookup != nil {
					// The call's own byte range, not its start position, which a receiver can share.
					oracleReturnType = oracleLookupExpressionFlat(oracleLookup, file, idx)
					if oracleReturnType != nil && oracleReturnType.Kind != typeinfer.TypeUnknown &&
						ignoredReturnValueTypeIsUnitOrNothing(oracleReturnType) {
						return
					}
					if ignoredReturnValueIsSideEffectResultHandler(file, idx, funcName, oracleReturnType) {
						return
					}
					if ignoredReturnValueIsFlowBuilderResultHandler(file, idx, funcName, oracleReturnType) {
						return
					}
					if ignoredReturnValueIsRxJavaSubscribeDisposable(file, idx, funcName, oracleReturnType) {
						return
					}
					annotations := ignoredReturnValueMergedOracleAnnotations(oracleLookup, file, idx, line, col)
					hasCheck, hasIgnore := ignoredReturnValueAnnotationEvidence(annotations, r.ReturnValueAnnotations, r.IgnoreReturnValueAnnotations)
					if hasIgnore {
						return
					}
					if hasCheck && ignoredReturnValueTestFileDisposableCall(file, funcName) {
						return
					}
					if hasCheck {
						ctx.EmitAt(calleeLine(file, idx), calleeCol(file, idx),
							ignoredReturnValueAnnotatedMessage(funcName, annotations, r.ReturnValueAnnotations))
						return
					}
					// A scope function yields its lambda's value, so its tail call is what may be ignored.
					if ignoredReturnValueScopeFunctionCalls[funcName] {
						return
					}
					if oracleReturnType != nil && oracleReturnType.Kind != typeinfer.TypeUnknown {
						if ignoredReturnValueTypeMatches(oracleReturnType, r.ReturnValueTypes, r.RestrictToConfig) &&
							!ignoredReturnValueFunctionTypeFromInvoke(file, idx, funcName, oracleReturnType) &&
							ignoredReturnValueCalleeIsKnownPure(oracleLookup, file, idx, funcName) {
							ctx.EmitAt(calleeLine(file, idx), calleeCol(file, idx),
								fmt.Sprintf("Return value of '%s' is ignored. '%s' is a pure stdlib function that returns %s.", funcName, funcName, ignoredReturnValueTypeName(oracleReturnType)))
							return
						}
						return
					}
				}

				if ctx.Resolver != nil {
					if ignoredReturnValueScopeFunctionCalls[funcName] {
						return
					}
					rt := ctx.Resolver.ResolveFlatNode(idx, file)
					if ignoredReturnValueIsSideEffectResultHandler(file, idx, funcName, rt) {
						return
					}
					if ignoredReturnValueIsFlowBuilderResultHandler(file, idx, funcName, rt) {
						return
					}
					if ignoredReturnValueIsRxJavaSubscribeDisposable(file, idx, funcName, rt) {
						return
					}
					if ignoredReturnValueTypeMatches(rt, r.ReturnValueTypes, r.RestrictToConfig) &&
						!ignoredReturnValueFunctionTypeFromInvoke(file, idx, funcName, rt) &&
						ignoredReturnValueCalleeIsKnownPure(oracleLookup, file, idx, funcName) {
						ctx.EmitAt(calleeLine(file, idx), calleeCol(file, idx),
							fmt.Sprintf("Return value of '%s' is ignored. '%s' is a pure stdlib function that returns %s.", funcName, funcName, ignoredReturnValueTypeName(rt)))
						return
					}
					if oracleLookup != nil && ignoredReturnValueTypeKnown(rt) {
						return
					}
					if r.RestrictToConfig {
						return
					}
				}

				if !r.RestrictToConfig && isKnownFunctionalOp &&
					ignoredReturnValueFunctionalFallbackHasReceiverEvidence(file, idx) {
					// No resolver/oracle evidence matched. Keep the historical
					// tree-sitter-only heuristic for no-oracle runs, but require
					// structural receiver evidence so local side-effect APIs named
					// map/filter/etc. do not get flagged solely by callee name.
					ctx.EmitAt(calleeLine(file, idx), calleeCol(file, idx),
						"Return value of functional operation is ignored. If this is intentional, assign to a variable or use 'also' instead.")
				}
			},
		})
	}
	{
		r := &ImplicitDefaultLocaleRule{BaseRule: BaseRule{RuleName: "ImplicitDefaultLocale", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects locale-sensitive string methods called without an explicit Locale argument."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixSemantic, Implementation: r,
			Needs: api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
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
						if containsASCIIInvariantIdentifier(recvText) {
							return
						}
					}
					f := r.Finding(file, callLine(file, idx), callCol(file, idx),
						fmt.Sprintf("'%s()' called without explicit Locale. Use '%s(Locale.ROOT)' or '%s(Locale.getDefault())' to be explicit.", methodName, methodName, methodName))
					if args != 0 && implicitLocaleAcceptsLocaleArg[methodName] && implicitLocaleFileHasLocaleImport(file) {
						argsEnd := int(file.FlatEndByte(args))
						if argsEnd > 0 && argsEnd <= len(file.Content) && file.Content[argsEnd-1] == ')' {
							// getDefault() preserves pre-fix behavior; ROOT would silently change case rules.
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   argsEnd - 1,
								EndByte:     argsEnd - 1,
								Replacement: "Locale.getDefault()",
							}
						}
					}
					ctx.Emit(f)
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
						formatText, isConst := implicitDefaultLocaleConstFormatString(file, firstArg)
						if isConst && isLocaleInsensitiveFormat(formatText) {
							return
						}
						argText := file.FlatNodeText(firstArg)
						isLiteral := strings.HasPrefix(strings.TrimSpace(argText), "\"")
						if isLiteral && isLocaleInsensitiveFormat(argText) {
							return
						}
						// Resource strings and parameters are display text, where the default locale is correct.
						if !isConst && !isLiteral {
							return
						}
					}

					ctx.EmitAt(callLine(file, idx), callCol(file, idx),
						fmt.Sprintf("'%s' uses implicit default locale for string formatting. Pass Locale explicitly, e.g. Locale.ROOT or Locale.US.", receiverText+".format(...)"))
				}
			},
		})
	}
	{
		r := &LocaleDefaultForCurrencyRule{BaseRule: BaseRule{RuleName: "LocaleDefaultForCurrency", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects Currency.getInstance(Locale.getDefault()) in money-related classes where currency should come from business data."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Implementation: r,
			Check: func(ctx *api.Context) {
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

				ctx.EmitAt(callLine(file, idx), callCol(file, idx),
					fmt.Sprintf("Class '%s' derives currency from Locale.getDefault(); use order or transaction currency data instead.", className))
			},
		})
	}
	{
		r := &HardcodedDateFormatRule{BaseRule: BaseRule{RuleName: "HardcodedDateFormat", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects DateTimeFormatter.ofPattern constructed without an explicit Locale."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression"},
			Languages:  []scanner.Language{scanner.LangKotlin},
			Confidence: r.Confidence(), Fix: api.FixSemantic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				navExpr, args := flatCallExpressionParts(file, idx)
				if navExpr == 0 || args == 0 {
					return
				}
				_, argCount := flatValueArgumentStats(file, args)
				if argCount != 1 {
					return
				}
				if flatNavigationExpressionLastIdentifier(file, navExpr) != "ofPattern" {
					return
				}
				receiver := file.FlatNamedChild(navExpr, 0)
				if receiver == 0 {
					return
				}
				receiverText := strings.TrimSpace(file.FlatNodeText(receiver))
				if receiverText != "DateTimeFormatter" && receiverText != "java.time.format.DateTimeFormatter" {
					return
				}
				if receiverText == "DateTimeFormatter" && !sourceImportsOrMentions(file, "java.time.format.DateTimeFormatter") {
					return
				}
				if hardcodedDateFormatHasLocaleChained(file, idx) {
					return
				}
				// Numeric-only or unknown patterns do not vary by locale, and names shown to the user should.
				firstArg, _ := flatValueArgumentStats(file, args)
				pattern, ok := hardcodedDateFormatPatternLiteral(file, firstArg)
				if !ok || !datePatternIsLocaleSensitive(pattern) || !datePatternIsMachineConsumed(file, idx, pattern) {
					return
				}
				message := "DateTimeFormatter.ofPattern(pattern) without explicit Locale. Pass Locale.ROOT for machine-readable formats or Locale.getDefault() to be explicit."
				if hardcodedDateFormatFlowsToUI(file, idx) {
					message = "DateTimeFormatter.ofPattern(pattern) without explicit Locale, and the result is shown in the UI. Pass Locale.getDefault() so the output follows the user's locale on purpose."
				}
				weekYear := datePatternHasWeekYearWithoutWeek(pattern)
				if weekYear {
					message += " The pattern also uses week-based year 'Y' without a week field 'w', which gives the wrong year around New Year; use 'y' unless week-based years are intended."
				}
				f := r.Finding(file, calleeLine(file, idx), calleeCol(file, idx), message)
				if !weekYear && implicitLocaleFileHasLocaleImport(file) {
					argsEnd := int(file.FlatEndByte(args))
					if argsEnd > 0 && argsEnd <= len(file.Content) && file.Content[argsEnd-1] == ')' {
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   argsEnd - 1,
							EndByte:     argsEnd - 1,
							Replacement: ", Locale.getDefault()",
						}
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &HardcodedNumberFormatRule{BaseRule: BaseRule{RuleName: "HardcodedNumberFormat", RuleSetName: "potential-bugs", Sev: "warning", Desc: "Detects DecimalFormat or NumberFormat.getInstance constructed without an explicit Locale."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression"},
			Languages:  []scanner.Language{scanner.LangKotlin},
			Confidence: r.Confidence(), Fix: api.FixSemantic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				navExpr, args := flatCallExpressionParts(file, idx)
				_, argCount := flatValueArgumentStats(file, args)

				if navExpr != 0 {
					if flatNavigationExpressionLastIdentifier(file, navExpr) != "getInstance" {
						return
					}
					receiver := file.FlatNamedChild(navExpr, 0)
					if receiver == 0 {
						return
					}
					receiverText := strings.TrimSpace(file.FlatNodeText(receiver))
					if receiverText != "NumberFormat" && receiverText != "java.text.NumberFormat" {
						return
					}
					if receiverText == "NumberFormat" && !sourceImportsOrMentions(file, "java.text.NumberFormat") {
						return
					}
					if argCount != 0 || !numberFormatResultFlowsToSink(file, idx) {
						return
					}
					f := r.Finding(file, calleeLine(file, idx), calleeCol(file, idx),
						"NumberFormat.getInstance() without explicit Locale. Pass a Locale to produce stable, locale-aware formatting.")
					if args != 0 && implicitLocaleFileHasLocaleImport(file) {
						argsEnd := int(file.FlatEndByte(args))
						if argsEnd > 0 && argsEnd <= len(file.Content) && file.Content[argsEnd-1] == ')' {
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   argsEnd - 1,
								EndByte:     argsEnd - 1,
								Replacement: "Locale.getDefault()",
							}
						}
					}
					ctx.Emit(f)
					return
				}

				if flatCallExpressionName(file, idx) != "DecimalFormat" {
					return
				}
				if !sourceImportsOrMentions(file, "java.text.DecimalFormat") {
					return
				}
				if argCount >= 2 || !numberFormatResultFlowsToSink(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"DecimalFormat without explicit Locale. Pass DecimalFormatSymbols(Locale) to produce stable, locale-aware formatting.")
			},
		})
	}
}
