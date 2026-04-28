package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"strconv"
	"strings"
)

func registerAndroidUsabilityRules() {

	// --- from android_usability.go ---
	{
		r := &AppCompatResourceRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "AppCompatResource", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "AppCompatResource", Brief: "Menu namespace collision with AppCompat",
			Category: ALCUsability, ALSeverity: ALSWarning, Priority: 4,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			Needs: v2.NeedsResources, AndroidDeps: uint32(r.AndroidDependencies()), Confidence: r.Confidence(), OriginalV1: r,
			Check: r.check,
		})
	}
	{
		r := &NewApiRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "NewApi", RuleSetName: androidRuleSet, Sev: "error"},
			IssueID:  "NewApi", Brief: "Calling new APIs on older versions",
			Category: ALCUnknown, ALSeverity: ALSError, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"call_expression", "simple_identifier", "user_type", "navigation_expression"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, OriginalV1: r,
			OracleCallTargets:      &v2.OracleCallTargetFilter{CalleeNames: []string{"setElevation", "getSystemService", "setDecorFitsSystemWindows", "requestPermissions", "checkSelfPermission", "getColor", "getDrawable", "setTranslationZ", "setClipToOutline", "createNotificationChannel"}},
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				line := file.FlatRow(idx)
				if nodeHasAncestorTypeFlat(file, idx, "import_header") {
					return
				}
				if newApiNestedAccessHandledByOuterNode(file, idx) {
					return
				}
				if apiGuardedByVersionCheckFlat(file, idx) {
					return
				}
				name := apiNodeNameFlat(file, idx)
				for api, level := range newApiTable {
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
		r := &InlinedApiRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "InlinedApi", RuleSetName: androidRuleSet, Sev: "warning"},
			IssueID:  "InlinedApi", Brief: "Using inlined constants from newer API",
			Category: ALCUnknown, ALSeverity: ALSWarning, Priority: 6,
			Origin: "AOSP Android Lint",
		}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"simple_identifier", "navigation_expression"}, Needs: v2.NeedsTypeInfo, Confidence: 0.75, OriginalV1: r,
			OracleDeclarationNeeds: &v2.OracleDeclarationProfile{},
			Check: func(ctx *v2.Context) {
				idx, file := ctx.Idx, ctx.File
				line := file.FlatRow(idx)
				if nodeHasAncestorTypeFlat(file, idx, "import_header") {
					return
				}
				if apiGuardedByVersionCheckFlat(file, idx) {
					return
				}
				ref := inlinedApiReferenceNameFlat(file, idx)
				for _, entry := range inlinedApiTable {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"function_declaration"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"navigation_expression"}, Confidence: r.Confidence(), OriginalV1: r,
			Check: func(ctx *v2.Context) {
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
