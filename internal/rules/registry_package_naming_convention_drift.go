package rules

import (
	"fmt"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"strings"
)

func registerPackageNamingConventionDriftRules() {

	// --- from package_naming_convention_drift.go ---
	{
		r := &PackageNamingConventionDriftRule{BaseRule: BaseRule{RuleName: "PackageNamingConventionDrift", RuleSetName: "architecture", Sev: "info", Desc: "Detects Kotlin source files whose package declaration does not match the directory path under src/main/kotlin."}}
		v2.Register(&v2.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: v2.Severity(r.Sev),
			NodeTypes: []string{"package_header"}, Confidence: 0.95, Implementation: r,
			Check: func(ctx *v2.Context) {
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
