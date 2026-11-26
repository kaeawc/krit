package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// daggerIssues lists the ported DaggerIssuesDetector issues with Lint's
// severity and brief description, which Lint also reports as the message.
var daggerIssues = []struct {
	id    string
	brief string
}{
	{daggerBindingReceiverParameter, "@Binds/@Provides functions cannot be extensions"},
	{daggerBindingReturnType, "@Binds/@Provides must have a return type"},
	{daggerBindsMustBeAbstract, "@Binds functions must be abstract"},
	{daggerMustBeInModule, "@Binds/@Provides functions must be in modules"},
	{daggerProvidesMustNotBeAbstract, "@Provides functions cannot be abstract"},
}

// DaggerIssuesRule is one DaggerIssuesDetector issue. Every rule evaluates the
// same per-method verdict (daggerIssueOf) and emits only its own ID.
type DaggerIssuesRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a high base confidence: the verdict needs only the
// method's annotations, modifiers and enclosing class, and abstains on an
// expression body whose type the source does not show.
func (r *DaggerIssuesRule) Confidence() float64 { return api.ConfidenceHigh }

// LintIssueID maps Lint's BindsWrongParameterCount onto the existing rule, so
// @SuppressLint, tools:ignore and lint.xml entries for the Lint ID apply.
func (r *BindsMismatchedArityRule) LintIssueID() string { return daggerBindsWrongParameterCount }

// LintIssueID maps Lint's RedundantBinds onto the existing rule.
func (r *BindsReturnTypeMatchesParamRule) LintIssueID() string { return "RedundantBinds" }

func registerThirdPartyDaggerRules() {
	for _, issue := range daggerIssues {
		r := &DaggerIssuesRule{BaseRule: BaseRule{RuleName: issue.id, RuleSetName: moshiRuleSet, Sev: string(api.SeverityError), Desc: issue.brief}}
		id, brief := issue.id, issue.brief
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.SeverityError,
			NodeTypes:      []string{"function_declaration", "method_declaration"},
			Languages:      []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			FileGate:       daggerFileGate,
			Confidence:     r.Confidence(),
			Implementation: r,
			Check: func(ctx *api.Context) {
				if verdict, ok := daggerIssueOf(ctx.File, ctx.Idx); ok && verdict.id == id {
					ctx.EmitAtNode(verdict.node, brief)
				}
			},
			LintIssue:     true,
			DefaultActive: false,
			OptInReason:   api.OptInReasonDomainSpecific,
			Maturity:      api.MaturityExperimental,
		})
	}
}
