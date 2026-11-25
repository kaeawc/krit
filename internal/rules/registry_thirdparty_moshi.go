package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// moshiIssue describes one MoshiUsageDetector issue: its Lint ID, severity,
// and brief description, which Lint also reports as the message.
type moshiIssue struct {
	id    string
	sev   api.Severity
	brief string
}

// moshiIssues lists the ported MoshiUsageDetector issues, alphabetically as in the detector's issues().
// NonMoshiClassInternal and NonMoshiClassExternal are not ported: they split on whether the type is in
// Slack's own `slack.*` packages.
var moshiIssues = []moshiIssue{
	{"MoshiUsageBlankGenerator", api.SeverityError, "Don't use blank JsonClass.generator values."},
	{"MoshiUsageBlankTypeLabel", api.SeverityError, "Moshi-sealed requires a type label specified after the 'sealed:' prefix."},
	{"MoshiUsageClassVisibility", api.SeverityError, "@JsonClass-annotated types must be public, package-private, or internal."},
	{"MoshiUsageDoubleClassAnnotation", api.SeverityError, "Only use one of @AdaptedBy or @JsonClass."},
	{"MoshiUsageDoubleTypeLabel", api.SeverityError, "Only use one of @TypeLabel or @DefaultObject."},
	{"MoshiUsageEnumAnnotatedUnknown", api.SeverityError, "UNKNOWN members in @JsonClass-annotated enums should not be annotated with @Json"},
	{"MoshiUsageEnumCasing", api.SeverityWarning, "Consider using @Json(name = ...) rather than lower casing."},
	{"MoshiUsageEnumJsonClassGenerated", api.SeverityError, "Enums annotated with @JsonClass must not set generateAdapter to true."},
	{"MoshiUsageEnumMissingJsonClass", api.SeverityError, "Enums serialized with Moshi should be annotated with @JsonClass."},
	{"MoshiUsageEnumMissingUnknown", api.SeverityError, "Enums serialized with Moshi must reserve the first member as UNKNOWN."},
	{"MoshiUsageGenerateAdapterShouldBeTrue", api.SeverityError, "JsonClass.generateAdapter must be true in order for Moshi code gen to run."},
	{"MoshiUsageGenericSealedSubtype", api.SeverityError, "Sealed subtypes used with moshi-sealed cannot be generic."},
	{"MoshiUsageInappropriateTypeLabel", api.SeverityError, "Inappropriate @TypeLabel or @DefaultObject annotation."},
	{"MoshiUsageMissingPrimary", api.SeverityError, "@JsonClass-annotated types must have a primary constructor or be sealed."},
	{"MoshiUsageMissingTypeLabel", api.SeverityError, "Sealed Moshi subtypes must be annotated with @TypeLabel or @DefaultObject."},
	{"MoshiUsageObject", api.SeverityError, "Object types cannot be annotated with @JsonClass."},
	{"MoshiUsageParamNeedsInit", api.SeverityError, "Constructor non-property parameters in Moshi classes must have default values."},
	{"MoshiUsagePrivateConstructor", api.SeverityError, "Constructors in Moshi classes cannot be private."},
	{"MoshiUsagePrivateConstructorProperty", api.SeverityError, "Constructor parameter properties in Moshi classes cannot be private."},
	{"MoshiUsageSealedMustBeSealed", api.SeverityError, "Moshi-sealed can only be applied to 'sealed' types."},
	{"MoshiUsageTransientNeedsInit", api.SeverityError, "Transient constructor properties must have default values."},
	{"MoshiUsageUnsupportedType", api.SeverityError, "This type cannot be annotated with @JsonClass."},
	{"MoshiUsageUseData", api.SeverityError, "Model classes should be immutable data classes."},
	{"MoshiUsageVarProperty", api.SeverityWarning, "Moshi properties should be immutable."},
}

// moshiIssueMessage returns the Lint message for a MoshiUsage issue ID.
func moshiIssueMessage(id string) string {
	for _, issue := range moshiIssues {
		if issue.id == id {
			return issue.brief
		}
	}
	return ""
}

// MoshiUsageRule is one MoshiUsageDetector issue. Every MoshiUsage rule reads
// the same shared project pass (moshiAnalysisFor) and emits its own findings.
type MoshiUsageRule struct {
	BaseRule
}

// Confidence reports a high base confidence: the shared pass reports only
// what the source proves and abstains otherwise.
func (r *MoshiUsageRule) Confidence() float64 { return api.ConfidenceHigh }

func registerThirdPartyMoshiRules() {
	for _, issue := range moshiIssues {
		r := &MoshiUsageRule{BaseRule: BaseRule{RuleName: issue.id, RuleSetName: moshiRuleSet, Sev: string(issue.sev), Desc: issue.brief}}
		api.Register(&api.Rule{
			ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: issue.sev,
			Needs:          api.NeedsParsedFiles,
			Languages:      []scanner.Language{scanner.LangKotlin, scanner.LangJava},
			Confidence:     r.Confidence(),
			Implementation: r,
			Check:          emitMoshiFindings,
			LintIssue:      true,
			DefaultActive:  false,
			OptInReason:    api.OptInReasonDomainSpecific,
			Maturity:       api.MaturityExperimental,
		})
	}
}
