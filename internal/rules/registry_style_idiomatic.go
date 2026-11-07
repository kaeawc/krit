package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func registerStyleIdiomaticRules() {

	// --- from style_idiomatic.go ---
	{
		r := &UseCheckNotNullRule{BaseRule: BaseRule{RuleName: "UseCheckNotNull", RuleSetName: "style", Sev: "warning", Desc: "Detects check(x != null) calls that should use checkNotNull(x) instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs: api.NeedsResolver, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				argText, suffixText, ok := flatNonNullCheckText(file, idx, "check")
				if !ok {
					return
				}
				if ctx.Resolver != nil {
					resolved := ctx.Resolver.ResolveByNameFlat(argText, idx, file)
					if resolved != nil && resolved.Kind != typeinfer.TypeUnknown && !resolved.IsNullable() {
						return
					}
				}
				f := r.Finding(file, callLine(file, idx), callCol(file, idx),
					"Use checkNotNull() instead of check() for checking not-null.")
				replacement := "checkNotNull(" + argText + ")"
				if suffixText != "" {
					replacement += " " + suffixText
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: replacement,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UseRequireNotNullRule{BaseRule: BaseRule{RuleName: "UseRequireNotNull", RuleSetName: "style", Sev: "warning", Desc: "Detects require(x != null) calls that should use requireNotNull(x) instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs: api.NeedsResolver, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				argText, suffixText, ok := flatNonNullCheckText(file, idx, "require")
				if !ok {
					return
				}
				if ctx.Resolver != nil {
					resolved := ctx.Resolver.ResolveByNameFlat(argText, idx, file)
					if resolved != nil && resolved.Kind != typeinfer.TypeUnknown && !resolved.IsNullable() {
						return
					}
				}
				f := r.Finding(file, callLine(file, idx), callCol(file, idx),
					"Use requireNotNull() instead of require() for checking not-null.")
				replacement := "requireNotNull(" + argText + ")"
				if suffixText != "" {
					replacement += " " + suffixText
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: replacement,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UseCheckOrErrorRule{BaseRule: BaseRule{RuleName: "UseCheckOrError", RuleSetName: "style", Sev: "warning", Desc: "Detects if (cond) throw IllegalStateException patterns that should use check()."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"jump_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs: api.NeedsResolver, Implementation: r,
			Check: func(ctx *api.Context) {
				flatThrowPattern(ctx, r.BaseRule, "IllegalStateException", "check", "Use check() or error() instead of throwing an IllegalStateException.", false)
			},
		})
	}
	{
		r := &UseRequireRule{BaseRule: BaseRule{RuleName: "UseRequire", RuleSetName: "style", Sev: "warning", Desc: "Detects if (cond) throw IllegalArgumentException patterns that should use require()."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"jump_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs: api.NeedsResolver, Implementation: r,
			Check: func(ctx *api.Context) {
				flatThrowPattern(ctx, r.BaseRule, "IllegalArgumentException", "require", "Use require() instead of throwing an IllegalArgumentException.", true)
			},
		})
	}
	{
		r := &UseIsNullOrEmptyRule{BaseRule: BaseRule{RuleName: "UseIsNullOrEmpty", RuleSetName: "style", Sev: "warning", Desc: "Detects x == null || x.isEmpty() patterns that should use isNullOrEmpty()."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"disjunction_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Needs:          api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Oracle:         &api.OracleFilter{Identifiers: []string{"isEmpty", "count", ".size", ".length", "\"\""}},
			Implementation: r,
			Check: func(ctx *api.Context) {
				flatUseIsNullOrEmpty(ctx, r.BaseRule)
			},
		})
	}
	{
		r := &UseOrEmptyRule{BaseRule: BaseRule{RuleName: "UseOrEmpty", RuleSetName: "style", Sev: "warning", Desc: "Detects x ?: emptyList() patterns that should use .orEmpty() instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"elvis_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) < 3 {
					return
				}
				left := file.FlatChild(idx, 0)
				right := file.FlatChild(idx, 2)
				if left == 0 || right == 0 {
					return
				}
				if !flatIsEmptyRHS(file, right) {
					return
				}
				if hasFlatAncestorTypeName(file, idx, "string_literal", "raw_string_literal", "line_string_literal", "multi_line_string_literal") {
					return
				}
				leftText := file.FlatNodeText(left)
				if strings.Contains(leftText, "?.") {
					return
				}
				if strings.Contains(leftText, "?.let") {
					return
				}
				rightText := strings.TrimSpace(file.FlatNodeText(right))
				// Array<T>?.orEmpty() returns Array<out T>, which an invariant Array<T> rejects, so emptyArray() is
				// left alone. detekt reports it; krit does not (see the FIR twin).
				if strings.HasPrefix(rightText, "emptyArray(") {
					return
				}
				// orEmpty() only exists for a matching nullable receiver type; skip when the declared type clearly differs.
				if file.FlatType(left) == "simple_identifier" {
					if typeName, found := declaredTypeNameOfIdentifier(file, idx, leftText); found && !orEmptyReceiverMatches(typeName, rightText) {
						return
					}
				}
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					fmt.Sprintf("Use '.orEmpty()' instead of '?: %s'.", rightText))
				var replacement string
				if lhsNeedsParensFlat(file, left) {
					replacement = "(" + leftText + ").orEmpty()"
				} else {
					replacement = leftText + ".orEmpty()"
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: replacement,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UseAnyOrNoneInsteadOfFindRule{BaseRule: BaseRule{RuleName: "UseAnyOrNoneInsteadOfFind", RuleSetName: "style", Sev: "warning", Desc: "Detects .find {} != null patterns that should use .any {} or .none {} instead."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"equality_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) < 3 {
					return
				}
				left := file.FlatChild(idx, 0)
				op := file.FlatChild(idx, 1)
				right := file.FlatChild(idx, file.FlatChildCount(idx)-1)
				if left == 0 || op == 0 || right == 0 {
					return
				}
				opText := file.FlatNodeText(op)
				leftText := strings.TrimSpace(file.FlatNodeText(left))
				rightText := strings.TrimSpace(file.FlatNodeText(right))
				isNullLeft := leftText == "null"
				isNullRight := rightText == "null"
				if !isNullLeft && !isNullRight {
					return
				}
				var replacement string
				switch opText {
				case "!=":
					replacement = "any"
				case "==":
					replacement = "none"
				default:
					return
				}
				callSideIdx := left
				if isNullLeft {
					callSideIdx = right
				}
				if file.FlatType(callSideIdx) != "call_expression" {
					return
				}
				nav, _ := file.FlatFindChild(callSideIdx, "navigation_expression")
				if nav == 0 {
					return
				}
				if flatLastChildOfType(file, nav, "navigation_suffix") == 0 {
					return
				}
				funcName := flatNavigationExpressionLastIdentifier(file, nav)
				if !anyOrNoneFindFuncs[funcName] {
					return
				}
				callSuffix, _ := file.FlatFindChild(callSideIdx, "call_suffix")
				if callSuffix == 0 {
					return
				}
				lambda := flatCallSuffixLambdaNode(file, callSuffix)
				if lambda == 0 {
					return
				}
				receiver := file.FlatNamedChild(nav, 0)
				if receiver == 0 {
					return
				}
				receiverText := file.FlatNodeText(receiver)
				lambdaText := file.FlatNodeText(lambda)
				msg := fmt.Sprintf("Use '.%s {}' instead of '.%s {} %s null'.", replacement, funcName, opText)
				f := r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1, msg)
				// A safe-call receiver makes any/none yield a nullable Boolean, so the null case is kept explicit.
				replaced := receiverText + "." + replacement + " " + lambdaText
				if suffix := flatLastChildOfType(file, nav, "navigation_suffix"); strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(suffix)), "?.") {
					if replacement == "any" {
						replaced = receiverText + "?.any " + lambdaText + " == true"
					} else {
						replaced = receiverText + "?.none " + lambdaText + " ?: true"
					}
				}
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(idx)),
					EndByte:     int(file.FlatEndByte(idx)),
					Replacement: replaced,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &UseEmptyCounterpartRule{BaseRule: BaseRule{RuleName: "UseEmptyCounterpart", RuleSetName: "style", Sev: "warning", Desc: "Detects listOf(), setOf(), and similar calls with no arguments that should use emptyList(), emptySet(), etc."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium, Fix: api.FixIdiomatic,
			Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				if file.FlatChildCount(idx) == 0 {
					return
				}
				callee := file.FlatChild(idx, 0)
				if callee == 0 || file.FlatType(callee) != "simple_identifier" {
					return
				}
				calleeName := file.FlatNodeText(callee)
				replacement, ok := emptyCounterparts[calleeName]
				if !ok {
					return
				}
				suffix, _ := file.FlatFindChild(idx, "call_suffix")
				if suffix == 0 {
					return
				}
				args := flatCallSuffixValueArgs(file, suffix)
				if args == 0 {
					return
				}
				for i := 0; i < file.FlatChildCount(args); i++ {
					child := file.FlatChild(args, i)
					if file.FlatType(child) == "value_argument" {
						return
					}
				}
				f := r.Finding(file, callLine(file, idx), callCol(file, idx),
					fmt.Sprintf("Use '%s()' instead of '%s()'.", replacement, calleeName))
				// Only the callee name changes, so explicit type arguments survive.
				f.Fix = &scanner.Fix{
					ByteMode:    true,
					StartByte:   int(file.FlatStartByte(callee)),
					EndByte:     int(file.FlatEndByte(callee)),
					Replacement: replacement,
				}
				ctx.Emit(f)
			},
		})
	}
	{
		r := &PreferToOverPairSyntaxRule{BaseRule: BaseRule{RuleName: "PreferToOverPairSyntax", RuleSetName: "style", Sev: "warning", Desc: "Detects Pair(a, b) constructor calls that should use the a to b syntax."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"call_expression"}, Confidence: api.ConfidenceMedium,
			Needs: api.NeedsResolver, Implementation: r,
			Check: func(ctx *api.Context) { checkPreferToOverPairSyntax(ctx, r) },
		})
	}
	{
		r := &ExplicitBackingFieldCandidateRule{BaseRule: BaseRule{RuleName: "ExplicitBackingFieldCandidate", RuleSetName: "style", Sev: "info", Desc: "Detects private mutable backing properties that only expose a read-only supertype and can use a Kotlin 2.4 explicit backing field."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"property_declaration"}, Confidence: api.ConfidenceMedium,
			NeedsLibraryFacts: true, Implementation: r,
			Check: func(ctx *api.Context) { checkExplicitBackingFieldCandidate(ctx, r) },
		})
	}
}
