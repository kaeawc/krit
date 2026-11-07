package rules

import (
	"fmt"
	"strings"

	"github.com/kaeawc/krit/internal/filefacts"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerStyleUnnecessaryRules() {

	// --- from style_unnecessary.go ---
	{
		r := &RedundantHigherOrderMapUsageRule{BaseRule: BaseRule{RuleName: "RedundantHigherOrderMapUsage", RuleSetName: "style", Sev: "warning", Desc: "Detects identity .map { it } calls that are no-ops and can be removed."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "map" {
					return
				}
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				suffix, _ := file.FlatFindChild(idx, "call_suffix")
				if suffix == 0 {
					return
				}
				// Explicit type arguments (`map<A, B> { it }`) change the result type, so the call is not a no-op.
				if file.FlatHasChildOfType(suffix, "type_arguments") {
					return
				}
				_, params, stmts := flatTrailingLambdaParts(file, suffix)
				if stmts == 0 {
					return
				}
				if file.FlatNamedChildCount(stmts) != 1 {
					return
				}
				stmt := file.FlatNamedChild(stmts, 0)
				// The lambda returns its only parameter: the implicit `it` or one that is named.
				paramName := "it"
				if params != 0 {
					paramName = strings.TrimSpace(file.FlatNodeText(params))
					if strings.ContainsAny(paramName, ",:()") {
						return
					}
				}
				if stmt == 0 || !file.FlatNodeTextEquals(stmt, paramName) {
					return
				}
				// On a Set, Map.values, or other Iterable, map { it } converts to a List, so only List receivers are redundant.
				if !mapReceiverIsKnownList(file, idx, file.FlatNamedChild(navExpr, 0)) {
					return
				}
				f := r.Finding(file, callLine(file, idx), callCol(file, idx),
					"This 'map' call can be removed.")
				if child := flatLastChildOfType(file, navExpr, "navigation_suffix"); child != 0 {
					if file.FlatChildTextOrEmpty(child, "simple_identifier") == "map" {
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(child)),
							EndByte:     int(file.FlatEndByte(idx)),
							Replacement: "",
						}
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnnecessaryApplyRule{BaseRule: BaseRule{RuleName: "UnnecessaryApply", RuleSetName: "style", Sev: "warning", Desc: "Detects `receiver.apply { ... }` blocks whose single statement only reaches the receiver, where an ordinary call or assignment is enough."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnnecessaryLetRule{BaseRule: BaseRule{RuleName: "UnnecessaryLet", RuleSetName: "style", Sev: "warning", Desc: "Detects .let {} calls that are identity transforms or single-call chains replaceable by direct invocation."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) { unnecessaryLetCheck(r, ctx) },
		})
	}
	{
		r := &UnnecessaryFilterRule{BaseRule: BaseRule{RuleName: "UnnecessaryFilter", RuleSetName: "style", Sev: "warning", Desc: "Detects .filter {}.first() chains that can be simplified to .first {} with the predicate."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "navigation_expression"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixIdiomatic, Implementation: r,
			Needs: api.NeedsResolver,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatType(idx) == "navigation_expression" {
					checkFilterSize(ctx, r, idx)
					return
				}
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				terminalName := flatNavigationExpressionLastIdentifier(file, navExpr)
				if terminalName == "" {
					return
				}
				replacement, isCall := filterTerminatorCalls[terminalName]
				if !isCall {
					return
				}
				if flatLastChildOfType(file, navExpr, "navigation_suffix") == 0 {
					return
				}
				callSuffix, _ := file.FlatFindChild(idx, "call_suffix")
				if callSuffix == 0 {
					return
				}
				if flatCallSuffixHasArgs(file, callSuffix) {
					return
				}
				filterCall := file.FlatNamedChild(navExpr, 0)
				if filterCall == 0 || file.FlatType(filterCall) != "call_expression" {
					return
				}
				if flatCallExpressionName(file, filterCall) != "filter" {
					return
				}
				predText := flatFilterPredicateText(file, filterCall)
				if predText == "" {
					return
				}
				if ctx.Resolver != nil {
					if skip := flatFilterCheckReceiver(filterCall, file, ctx.Resolver); skip {
						return
					}
				}
				msg := fmt.Sprintf("Replace '.filter %s.%s()' with '.%s %s'.",
					predText, terminalName, replacement, predText)
				f := r.Finding(file, callLine(file, idx), callCol(file, idx), msg)
				filterNavExpr, _ := flatCallExpressionParts(file, filterCall)
				filterNavSuffix := flatLastChildOfType(file, filterNavExpr, "navigation_suffix")
				if filterNavSuffix != 0 {
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(filterNavSuffix)),
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: "." + replacement + " " + predText,
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnnecessaryAnyRule{BaseRule: BaseRule{RuleName: "UnnecessaryAny", RuleSetName: "style", Sev: "warning", Desc: "Detects .any { true } and .filter {}.any() patterns that can be simplified."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				methodName := flatCallExpressionName(file, idx)
				if methodName == "" {
					return
				}
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr == 0 {
					return
				}
				switch methodName {
				case "any", "none":
				default:
					return
				}
				suffix, _ := file.FlatFindChild(idx, "call_suffix")
				if suffix == 0 {
					return
				}
				if methodName == "any" && !flatAnyCallSuffixHasLambda(file, suffix) {
					// .filter { pred }.any() pattern
					if flatCallSuffixHasArgs(file, suffix) {
						return
					}
					filterCall := file.FlatNamedChild(navExpr, 0)
					if filterCall == 0 || file.FlatType(filterCall) != "call_expression" {
						return
					}
					if flatCallExpressionName(file, filterCall) != "filter" {
						return
					}
					callSuffix, _ := file.FlatFindChild(filterCall, "call_suffix")
					predText := flatAnyLambdaFullText(file, callSuffix)
					if predText == "" {
						return
					}
					navExprChild, _ := file.FlatFindChild(filterCall, "navigation_expression")
					receiverNode := file.FlatNamedChild(navExprChild, 0)
					if receiverNode == 0 {
						return
					}
					f := r.Finding(file, callLine(file, idx), callCol(file, idx),
						"Use .any { predicate } instead of .filter { predicate }.any().")
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatEndByte(receiverNode)),
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: ".any " + predText,
					}
					ctx.Emit(f)
					return
				}
				bodyText := flatAnyLambdaBodyText(file, suffix)
				if bodyText == "" {
					return
				}
				navSuffix := flatLastChildOfType(file, navExpr, "navigation_suffix")
				if navSuffix == 0 {
					return
				}
				dotStart := int(file.FlatStartByte(navSuffix))
				// `any { it }` is not covered: on Boolean elements it asks whether any element is true. isNotEmpty() does not exist on Iterable or Sequence, so the fix is the parameterless call.
				if bodyText != "true" {
					return
				}
				f := r.Finding(file, callLine(file, idx), callCol(file, idx),
					"Replace '."+methodName+" { true }' with '."+methodName+"()'.")
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   dotStart,
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: "." + methodName + "()",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnnecessaryBracesAroundTrailingLambdaRule{BaseRule: BaseRule{RuleName: "UnnecessaryBracesAroundTrailingLambda", RuleSetName: "style", Sev: "warning", Desc: "Detects empty parentheses before trailing lambdas that can be removed."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixCosmetic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) < 2 {
					return
				}
				// The grammar reads the class body of `class A : B by make() { ... }` as a trailing lambda.
				if _, inDelegation := flatEnclosingAncestor(file, idx, "delegation_specifier", "explicit_delegation", "class_declaration", "object_declaration"); inDelegation && unnecessaryBracesCallIsDelegate(file, idx) {
					return
				}
				innerCall := file.FlatChild(idx, 0)
				if innerCall == 0 || file.FlatType(innerCall) != "call_expression" {
					return
				}
				outerSuffix := file.FlatChild(idx, 1)
				if outerSuffix == 0 || file.FlatType(outerSuffix) != "call_suffix" {
					return
				}
				if flatCallSuffixLambdaNode(file, outerSuffix) == 0 {
					return
				}
				innerSuffix, _ := file.FlatFindChild(innerCall, "call_suffix")
				if innerSuffix == 0 {
					return
				}
				args, _ := file.FlatFindChild(innerSuffix, "value_arguments")
				if args == 0 {
					return
				}
				for i := 0; i < file.FlatChildCount(args); i++ {
					if file.FlatType(file.FlatChild(args, i)) == "value_argument" {
						return
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(innerSuffix)+1,
					"Empty parentheses before trailing lambda can be removed.")
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(innerSuffix)),
					EndByte:     int(file.FlatEndByte(innerSuffix)),
					Replacement: "",
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnnecessaryFullyQualifiedNameRule{BaseRule: BaseRule{RuleName: "UnnecessaryFullyQualifiedName", RuleSetName: "style", Sev: "warning", Desc: "Detects fully qualified names that are unnecessary because the type is already imported."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression", "user_type"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixNone, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				text := strings.Join(strings.Fields(file.FlatNodeText(idx)), "")
				if !strings.Contains(text, ".") || nodeHasAncestorTypeFlat(file, idx, "import_header", "package_header") {
					return
				}
				imports := filefacts.FileFact(fileFactsCache(), file, slotUnnecessaryFQNImports, func() map[string]bool {
					return unnecessaryFQNImports(file)
				})
				short, ok := "", imports[text]
				if !ok {
					return
				}
				short = text[strings.LastIndex(text, ".")+1:]
				// A user_type that is only the qualifier of a longer type (Outer.Inner) is not a use of the import.
				if p, hasParent := file.FlatParent(idx); hasParent && file.FlatType(p) == "user_type" && file.FlatType(idx) == "user_type" {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Unnecessary fully qualified name '%s'. Use '%s' since it's already imported.", text, short))
			},
		})
	}
	{
		r := &UnnecessaryReversedRule{BaseRule: BaseRule{RuleName: "UnnecessaryReversed", RuleSetName: "style", Sev: "warning", Desc: "Detects chained sort and reverse calls that can be replaced with a single sort operation."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceVeryHigh, Fix: api.FixIdiomatic, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				outerMethod := flatCallExpressionName(file, idx)
				if outerMethod == "" {
					return
				}
				receiverCall := flatReceiverCallExpression(file, idx)
				if receiverCall == 0 {
					return
				}
				innerMethod := flatCallExpressionName(file, receiverCall)
				if innerMethod == "" {
					return
				}
				var sortMethod string
				if unnRevReverseFuncs[outerMethod] && unnRevSortFuncs[innerMethod] {
					sortMethod = innerMethod
				} else if unnRevSortFuncs[outerMethod] && unnRevReverseFuncs[innerMethod] {
					sortMethod = outerMethod
				} else {
					return
				}
				replacement, ok := unnRevSortOpposites[sortMethod]
				if !ok {
					return
				}
				msg := fmt.Sprintf("Replace '%s().%s()' with '%s()' for a single sort operation.",
					innerMethod, outerMethod, replacement)
				f := r.Finding(file, callLine(file, idx), callCol(file, idx), msg)
				navExpr, _ := flatCallExpressionParts(file, receiverCall)
				if navExpr != 0 && file.FlatNamedChildCount(navExpr) > 0 {
					innerReceiver := file.FlatNamedChild(navExpr, 0)
					innerReceiverText := file.FlatNodeText(innerReceiver)
					if unnRevReverseFuncs[outerMethod] {
						sortCallSuffix, _ := file.FlatFindChild(receiverCall, "call_suffix")
						suffixText := "()"
						if sortCallSuffix != 0 {
							suffixText = file.FlatNodeText(sortCallSuffix)
						}
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(idx)),
							EndByte:     int(file.FlatEndByte(idx)),
							Replacement: innerReceiverText + "." + replacement + suffixText,
						}
					} else {
						outerSuffix, _ := file.FlatFindChild(idx, "call_suffix")
						suffixText := "()"
						if outerSuffix != 0 {
							suffixText = file.FlatNodeText(outerSuffix)
						}
						reverseReceiver := file.FlatNamedChild(navExpr, 0)
						if reverseReceiver != 0 {
							reverseReceiverText := file.FlatNodeText(reverseReceiver)
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   int(file.FlatStartByte(idx)),
								EndByte:     int(file.FlatEndByte(idx)),
								Replacement: reverseReceiverText + "." + replacement + suffixText,
							}
						}
					}
				}
				ctx.Emit(f)
			},
		})
	}
}

// unnecessaryBracesCallIsDelegate reports a call that is the delegate expression of a supertype (`by call()`).
func unnecessaryBracesCallIsDelegate(file *scanner.File, call uint32) bool {
	for p, ok := file.FlatParent(call); ok; p, ok = file.FlatParent(p) {
		switch file.FlatType(p) {
		case "delegation_specifier", "explicit_delegation":
			return true
		case "class_body", "function_body", "statements", "source_file":
			return false
		}
	}
	return false
}

// unnecessaryFQNImports returns the plain (non-wildcard, non-aliased) imports of a file whose
// short name is not also declared in the file.
func unnecessaryFQNImports(file *scanner.File) map[string]bool {
	out := make(map[string]bool)
	declared := make(map[string]bool)
	file.FlatWalkAllNodes(0, func(n uint32) {
		switch file.FlatType(n) {
		case "class_declaration", "object_declaration", "type_alias":
			if name := extractIdentifierFlat(file, n); name != "" {
				declared[name] = true
			}
		}
	})
	file.FlatWalkNodes(0, "import_header", func(n uint32) {
		path := importStatementPath(file, n)
		if strings.Contains(path, " as ") || strings.HasSuffix(path, "*") || !strings.Contains(path, ".") {
			return
		}
		path = strings.Join(strings.Fields(path), "")
		if declared[path[strings.LastIndex(path, ".")+1:]] {
			return
		}
		out[path] = true
	})
	return out
}

// checkFilterSize reports `xs.filter { p }.size`, which `xs.count { p }` states directly.
func checkFilterSize(ctx *api.Context, r *UnnecessaryFilterRule, nav uint32) {
	file := ctx.File
	// A navigation that is the callee of a call (`.size()`) is not the property.
	if parent, ok := file.FlatParent(nav); ok && file.FlatType(parent) == "call_expression" && file.FlatFirstChild(parent) == nav {
		return
	}
	if flatNavigationExpressionLastIdentifier(file, nav) != "size" || flatLastChildOfType(file, nav, "navigation_suffix") == 0 {
		return
	}
	filterCall := file.FlatNamedChild(nav, 0)
	if filterCall == 0 || file.FlatType(filterCall) != "call_expression" || flatCallExpressionName(file, filterCall) != "filter" {
		return
	}
	predText := flatFilterPredicateText(file, filterCall)
	if predText == "" {
		return
	}
	if ctx.Resolver != nil && flatFilterCheckReceiver(filterCall, file, ctx.Resolver) {
		return
	}
	msg := fmt.Sprintf("Replace '.filter %s.size' with '.count %s'.", predText, predText)
	f := r.Finding(file, file.FlatRow(nav)+1, file.FlatCol(nav)+1, msg)
	if filterNavExpr, _ := flatCallExpressionParts(file, filterCall); filterNavExpr != 0 {
		if suffix := flatLastChildOfType(file, filterNavExpr, "navigation_suffix"); suffix != 0 {
			f.Fix = &scanner.Fix{
				ByteMode:    true,
				StartByte:   int(file.FlatStartByte(suffix)),
				EndByte:     int(file.FlatEndByte(nav)),
				Replacement: ".count " + predText,
			}
		}
	}
	ctx.Emit(f)
}
