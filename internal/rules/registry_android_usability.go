package rules

import (
	"slices"
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func registerAndroidUsabilityRules() {

	// --- from android_usability.go ---
	{
		r := &NewAPIRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "NewApi", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:  "NewApi", Brief: "Calling new APIs on older versions",
			Category: ALCUnknown, ALSeverity: ALSError, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "simple_identifier", "user_type", "navigation_expression"},
			Needs:      api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Confidence: api.ConfidenceMedium, Implementation: r,
			NeedsLibraryFacts: true,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				minSdk := projectMinSdk(ctx)
				line := file.FlatRow(idx)
				if nodeHasAncestorTypeFlat(file, idx, "import_header") {
					return
				}
				if newAPINestedAccessHandledByOuterNode(file, idx) {
					return
				}
				if apiGuardedByVersionCheckFlat(file, idx) {
					return
				}
				name := apiNodeNameFlat(file, idx)
				for api, level := range newAPITable {
					key := strings.TrimSuffix(strings.TrimSuffix(api, "<"), "(")
					if name == key {
						// Nothing to report once the project minSdk covers the API, or when the call is a
						// compat wrapper (ContextCompat.getColor) or a function this file declares itself.
						if level <= minSdk || newAPICompatReceiver(file, idx, name) || fileDeclaresFunctionNamed(file, name) {
							return
						}
						if !newAPICallTargetMatches(ctx, name) {
							return
						}
						ctx.EmitAt(line+1, file.FlatCol(idx)+1,
							key+" requires API "+strconv.Itoa(level)+"; verify that the call is guarded or the project minSdk is at least "+strconv.Itoa(level)+".")
						return
					}
				}
			},
		})
	}
	{
		r := &InlinedAPIRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "InlinedApi", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "InlinedApi", Brief: "Using inlined constants from newer API",
			Category: ALCUnknown, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"simple_identifier", "navigation_expression"}, Needs: api.NeedsTypeInfo, Confidence: api.ConfidenceMedium, Implementation: r,
			NeedsLibraryFacts: true,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				minSdk := projectMinSdk(ctx)
				line := file.FlatRow(idx)
				if nodeHasAncestorTypeFlat(file, idx, "import_header") {
					return
				}
				if newAPINestedAccessHandledByOuterNode(file, idx) {
					return
				}
				if apiGuardedByVersionCheckFlat(file, idx) {
					return
				}
				ref := inlinedAPIReferenceNameFlat(file, idx)
				for _, entry := range inlinedAPITable {
					if ref == entry.Pattern || strings.HasSuffix(ref, "."+entry.Pattern) {
						if entry.Level <= minSdk {
							return
						}
						ctx.EmitAt(line+1, file.FlatCol(idx)+1,
							"Constant "+entry.Pattern+" is inlined from API "+strconv.Itoa(entry.Level)+"; the value may be available at runtime but the constant was introduced in API "+strconv.Itoa(entry.Level)+".")
						return
					}
				}
			},
		})
	}
	{
		r := &OverrideRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "Override", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:  "Override", Brief: "Method override errors",
			Category: ALCUnknown, ALSeverity: ALSError, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				name := extractIdentifierFlat(file, idx)
				paramCounts, known := overrideMethodNames[name]
				if !known || file.FlatHasModifier(idx, "override") {
					return
				}
				if count, _ := analyzeFunctionViewSignature(file, idx); !slices.Contains(paramCounts, count) {
					return
				}
				if !overrideEnclosingAndroidComponentFlat(file, idx, name) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"fun "+name+"(...) should be declared with `override` in Activity/Fragment subclasses.")
			},
		})
	}
	{
		r := &RequiresAPIViolationRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "RequiresApiViolation", RuleSetName: androidRuleSet, Sev: "error",
				Desc: "Detects calls to APIs annotated with @RequiresApi(N) (or @TargetApi(N)) when N is greater than the project's minSdk and the call is not SDK_INT guarded."},
			IssueID: "RequiresApiViolation", Brief: "Calling an API above the module's minSdk",
			Category: ALCUnknown, ALSeverity: ALSError, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes:  []string{"call_expression", "navigation_expression", "user_type"},
			Languages:  []scanner.Language{scanner.LangKotlin},
			Needs:      api.NeedsTypeInfo | api.NeedsOracleCallTargets,
			Confidence: r.Confidence(), Implementation: r,
			NeedsLibraryFacts: true,
			Check:             r.check,
		})
	}
	{
		r := &UnusedResourcesRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "UnusedResources", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "UnusedResources", Brief: "Unused resources",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 3,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
	{
		r := &UnusedIDsRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "UnusedIds", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "UnusedIds", Brief: "Unused id",
			Category: ALCPerformance, ALSeverity: ALSWarning, Priority: 1,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			Needs: api.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), Implementation: r,
			Check: r.check,
		})
	}
}
