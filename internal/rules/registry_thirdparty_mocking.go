package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// mockIssues lists the ported MockDetector issues with Lint's brief description. Findings carry the
// checker's own reason, as in Lint.
var mockIssues = []struct {
	id    string
	brief string
}{
	{mockDoNotMockAnything, "Do not add new mocks."},
	{mockDoNotMockDataClass, "data classes represent pure data classes, so mocking them should not be necessary."},
	{mockDoNotMockObjectClass, "object classes are singletons, so mocking them should not be necessary"},
	{mockDoNotMockPlatformTypes, "platform types should not be mocked"},
}

// MockDetectorRule is one MockDetector issue. Every rule reads the same
// project pass (mockAnalysisFor) and emits its own findings.
type MockDetectorRule struct {
	BaseRule
}

// Confidence reports a high base confidence: the pass reports a mock only when
// the source shows the mocked type and its declaration.
func (r *MockDetectorRule) Confidence() float64 { return api.ConfidenceHigh }

func registerThirdPartyMockingRules() {
	for _, issue := range mockIssues {
		r := &MockDetectorRule{BaseRule: BaseRule{RuleName: issue.id, RuleSetName: moshiRuleSet, Sev: string(api.SeverityError), Desc: issue.brief}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.SeverityError,
			Needs:          api.NeedsCrossFile,
			Languages:      []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence:     r.Confidence(),
			Implementation: r,
			Check:          emitMockFindings,
			LintIssue:      true,
			DefaultActive:  false,
			OptInReason:    api.OptInReasonDomainSpecific,
			Maturity:       api.MaturityExperimental,
		})
	}
}
