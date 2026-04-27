package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"strings"
)

func registerStyleUnnecessaryRules() {

	// --- from style_unnecessary.go ---
	{
		r := &RedundantHigherOrderMapUsageRule{BaseRule: BaseRule{RuleName: "RedundantHigherOrderMapUsage", RuleSetName: "style", Sev: "warning", Desc: "Detects identity .map { it } calls that are no-ops and can be removed."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.95, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				_, _, stmts := flatTrailingLambdaParts(file, suffix)
				if stmts == 0 {
					return
				}
				if file.FlatNamedChildCount(stmts) != 1 {
					return
				}
				stmt := file.FlatNamedChild(stmts, 0)
				if stmt == 0 || !file.FlatNodeTextEquals(stmt, "it") {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"Redundant '.map { it }' \u2014 this is a no-op.")
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
		r := &UnnecessaryApplyRule{BaseRule: BaseRule{RuleName: "UnnecessaryApply", RuleSetName: "style", Sev: "warning", Desc: "Detects .apply {} blocks that are empty or do not reference the receiver."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.95, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if flatCallExpressionName(file, idx) != "apply" {
					return
				}
				suffix, _ := file.FlatFindChild(idx, "call_suffix")
				if suffix == 0 {
					return
				}
				if flatCallSuffixLambdaNode(file, suffix) == 0 {
					return
				}
				_, _, stmts := flatTrailingLambdaParts(file, suffix)
				var msg string
				if stmts == 0 {
					msg = "Unnecessary empty '.apply {}' block."
				} else if file.FlatChildCount(stmts) == 0 {
					msg = "Unnecessary empty '.apply {}' block."
				} else if !flatApplyBodyReferencesThis(file, stmts) {
					msg = "'.apply {}' block does not reference the receiver \u2014 consider removing it or using 'also'/'let'."
				} else {
					return
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
				navExpr, _ := flatCallExpressionParts(file, idx)
				if navExpr != 0 {
					if child := flatLastChildOfType(file, navExpr, "navigation_suffix"); child != 0 {
						if strings.Contains(file.FlatNodeText(child), "apply") {
							f.Fix = &scanner.Fix{
								ByteMode:    true,
								StartByte:   int(file.FlatStartByte(child)),
								EndByte:     int(file.FlatEndByte(idx)),
								Replacement: "",
							}
						}
					}
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UnnecessaryLetRule{BaseRule: BaseRule{RuleName: "UnnecessaryLet", RuleSetName: "style", Sev: "warning", Desc: "Detects .let {} calls that are identity transforms or single-call chains replaceable by direct invocation."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.95, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				navNode, _ := flatCallExpressionParts(file, idx)
				if navNode == 0 {
					return
				}
				navSuffix := flatLastChildOfType(file, navNode, "navigation_suffix")
				if navSuffix == 0 {
					return
				}
				funcIdent, _ := file.FlatFindChild(navSuffix, "simple_identifier")
				if funcIdent == 0 || !file.FlatNodeTextEquals(funcIdent, "let") {
					return
				}
				isSafeCall := strings.Contains(file.FlatNodeText(navSuffix), "?.")
				suffix, _ := file.FlatFindChild(idx, "call_suffix")
				if suffix == 0 {
					return
				}
				lambdaLit, params, stmts := flatTrailingLambdaParts(file, suffix)
				if lambdaLit == 0 {
					return
				}
				paramName := "it"
				if params != 0 && file.FlatChildCount(params) > 0 {
					paramName = file.FlatNodeText(file.FlatChild(params, 0))
				}
				if stmts == 0 || file.FlatChildCount(stmts) != 1 {
					return
				}
				stmtText := strings.TrimSpace(file.FlatNodeText(file.FlatChild(stmts, 0)))
				if stmtText == paramName {
					msg := "Unnecessary '.let { " + paramName + " }' \u2014 the value can be used directly."
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(navSuffix)),
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: "",
					}
					ctx.Emit(f)
					return
				}
				prefix := paramName + "."
				if strings.HasPrefix(stmtText, prefix) && !strings.Contains(stmtText, "{") {
					remainder := stmtText[len(paramName):]
					if isSafeCall {
						msg := "Unnecessary '?.let { " + stmtText + " }' \u2014 can be replaced with '?." + stmtText[len(prefix):] + "'."
						f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
						f.Fix = &scanner.Fix{
							ByteMode:    true,
							StartByte:   int(file.FlatStartByte(navSuffix)),
							EndByte:     int(file.FlatEndByte(idx)),
							Replacement: "?" + remainder,
						}
						ctx.Emit(f)
						return
					}
					msg := "Unnecessary '.let { " + stmtText + " }' \u2014 can be replaced with '" + remainder + "'."
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   int(file.FlatStartByte(navSuffix)),
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: remainder,
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &UnnecessaryFilterRule{BaseRule: BaseRule{RuleName: "UnnecessaryFilter", RuleSetName: "style", Sev: "warning", Desc: "Detects .filter {}.first() chains that can be simplified to .first {} with the predicate."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.95, Fix: v2.FixIdiomatic, OriginalV1: r,
			Needs: v2.NeedsResolver,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
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
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.95, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
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
				switch {
				case methodName == "any" && (bodyText == "true" || bodyText == "it"):
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"Replace '.any { "+bodyText+" }' with '.isNotEmpty()'.")
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   dotStart,
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: ".isNotEmpty()",
					}
					ctx.Emit(f)
				case methodName == "none" && bodyText == "true":
					f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
						"Replace '.none { true }' with '.isEmpty()'.")
					f.Fix = &scanner.Fix{
						ByteMode:    true,
						StartByte:   dotStart,
						EndByte:     int(file.FlatEndByte(idx)),
						Replacement: ".isEmpty()",
					}
					ctx.Emit(f)
				}
			},
		})
	}
	{
		r := &UnnecessaryBracesAroundTrailingLambdaRule{BaseRule: BaseRule{RuleName: "UnnecessaryBracesAroundTrailingLambda", RuleSetName: "style", Sev: "warning", Desc: "Detects empty parentheses before trailing lambdas that can be removed."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.95, Fix: v2.FixCosmetic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) < 2 {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression"}, Confidence: 0.95, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				text := file.FlatNodeText(idx)
				if !strings.Contains(text, ".") {
					return
				}
				imports := make(map[string]bool)
				for _, line := range file.Lines {
					trimmed := strings.TrimSpace(line)
					if strings.HasPrefix(trimmed, "import ") {
						imp := strings.TrimPrefix(trimmed, "import ")
						imp = strings.TrimSpace(imp)
						imports[imp] = true
					}
				}
				for imp := range imports {
					parts := strings.Split(imp, ".")
					if len(parts) < 2 {
						continue
					}
					if strings.Contains(text, imp) {
						shortName := parts[len(parts)-1]
						ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
							fmt.Sprintf("Unnecessary fully qualified name '%s'. Use '%s' since it's already imported.", imp, shortName))
						return
					}
				}
			},
		})
	}
	{
		r := &UnnecessaryReversedRule{BaseRule: BaseRule{RuleName: "UnnecessaryReversed", RuleSetName: "style", Sev: "warning", Desc: "Detects chained sort and reverse calls that can be replaced with a single sort operation."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: 0.95, Fix: v2.FixIdiomatic, OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
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
