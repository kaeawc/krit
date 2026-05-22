package rules

import (
	"fmt"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func registerPackageNamingConventionDriftRules() {

	// --- from package_naming_convention_drift.go ---
	{
		r := &PackageNamingConventionDriftRule{BaseRule: BaseRule{RuleName: "PackageNamingConventionDrift", RuleSetName: "architecture", Sev: "info", Desc: "Detects Kotlin source files whose package declaration does not match the directory path under src/main/kotlin."}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
			NodeTypes: []string{"package_header"}, Confidence: api.ConfidenceVeryHigh, Implementation: r,
			Check: func(ctx *api.Context) {
				idx, file := ctx.Idx, ctx.File
				pkg := packageHeaderNameFlat(file, idx)
				if pkg == "" {
					return
				}
				expectedPrefix := packageNamingConventionExpectedPrefix(file.Path)
				if expectedPrefix == "" {
					return
				}
				if pkg == expectedPrefix || strings.HasPrefix(pkg, expectedPrefix+".") {
					return
				}
				ctx.EmitAt(file.FlatRow(idx)+1, 1, fmt.Sprintf("Package declaration '%s' drifts from source path; expected prefix '%s'.", pkg, expectedPrefix))
			},
		})
	}
}
