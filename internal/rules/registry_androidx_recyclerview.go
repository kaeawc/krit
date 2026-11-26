package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidXRecyclerViewChecks registers the check from androidx.recyclerview:recyclerview 1.4.0.
func registerAndroidXRecyclerViewChecks() {
	r := &InvalidSetHasFixedSizeRule{AndroidRule: lintAndroidRule("InvalidSetHasFixedSize", recyclerFixedSizeMessage,
		ALCCorrectness, ALSFatal, 5)}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Description(), Sev: api.Severity(r.Sev),
		NodeTypes: lintCallNodeTypes[:2], Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Needs: api.NeedsResources, AndroidDeps: uint32(AndroidDepLayout), Confidence: r.Confidence(), Implementation: r,
		NeedsLibraryFacts: true, Check: r.check,
	})
}
