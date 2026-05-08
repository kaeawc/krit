package rules

import (
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
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
			Confidence: 0.75, Implementation: r,
			OracleCallTargets:      &api.OracleCallTargetFilter{CalleeNames: []string{"setElevation", "getSystemService", "setDecorFitsSystemWindows", "requestPermissions", "checkSelfPermission", "getColor", "getDrawable", "setTranslationZ", "setClipToOutline", "createNotificationChannel"}},
			OracleDeclarationNeeds: &api.OracleDeclarationProfile{},
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
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
						ctx.EmitAt(line+1, 1,
							api+" requires API "+strconv.Itoa(level)+"; verify that the call is guarded or the project minSdk is at least "+strconv.Itoa(level)+".")
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
			NodeTypes: []string{"simple_identifier", "navigation_expression"}, Needs: api.NeedsTypeInfo, Confidence: 0.75, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				line := file.FlatRow(idx)
				if nodeHasAncestorTypeFlat(file, idx, "import_header") {
					return
				}
				if apiGuardedByVersionCheckFlat(file, idx) {
					return
				}
				ref := inlinedAPIReferenceNameFlat(file, idx)
				for _, entry := range inlinedAPITable {
					if ref == entry.Pattern || strings.HasSuffix(ref, "."+entry.Pattern) {
						ctx.EmitAt(line+1, 1,
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
				if !overrideMethodNames[name] || file.FlatHasModifier(idx, "override") {
					return
				}
				if !overrideEnclosingAndroidComponentFlat(file, idx) {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, file.FlatCol(idx)+1,
					"fun "+name+"(...) should be declared with `override` in Activity/Fragment subclasses.")
			},
		})
	}
	{
		r := &UnusedResourcesRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "UnusedResources", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "UnusedResources", Brief: "Unused resources",
			Category: ALCUnknown, ALSeverity: ALSWarning, Priority: 3,
			Origin: "AOSP Android Lint",
		}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression"}, Confidence: r.Confidence(), Implementation: r,
			Check: func(ctx *api.Context) {
				resType, resName, ok := unusedResourceReferenceFlat(ctx.File, ctx.Idx)
				if !ok {
					return
				}
				ctx.EmitAt(ctx.File.FlatRow(ctx.Idx)+1, ctx.File.FlatCol(ctx.Idx)+1,
					"Resource 'R."+resType+"."+resName+"' uses a test/temp naming pattern and may be unused.")
			},
		})
	}
}
