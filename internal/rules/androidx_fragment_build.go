package rules

// Ports of the build and layout checks bundled in androidx.fragment 1.9.1: GradleConfigurationDetector from
// fragment-testing-manifest-lint and fragment-testing-lint (both register FragmentGradleConfiguration), and
// FragmentTagDetector from fragment-lint.

import (
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/librarymodel"
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// FragmentGradleConfigurationRule mirrors FragmentGradleConfiguration: fragment-testing or fragment-testing-manifest
// declared on a configuration other than debugImplementation.
type FragmentGradleConfigurationRule struct {
	GradleBase
	AndroidRule
}

// FragmentTagUsageRule mirrors FragmentTagUsage: a <fragment> tag in a layout.
type FragmentTagUsageRule struct {
	LayoutResourceBase
	AndroidRule
}

// Confidence reports a high base confidence: the check reads literal dependency coordinates.
func (r *FragmentGradleConfigurationRule) Confidence() float64 { return api.ConfidenceHigh }

// Confidence reports a high base confidence: the check reads literal layout structure.
func (r *FragmentTagUsageRule) Confidence() float64 { return api.ConfidenceHigh }

func registerAndroidXFragmentBuildChecks() {
	gradle := &FragmentGradleConfigurationRule{AndroidRule: lintAndroidRule("FragmentGradleConfiguration",
		"Include the fragment-testing-manifest library using the debugImplementation configuration.", ALCCorrectness, ALSError, 5)}
	registerLintGradleRule(gradle.AndroidRule, gradle, gradle.Confidence(), gradle.check)
	tag := &FragmentTagUsageRule{AndroidRule: lintAndroidRule("FragmentTagUsage",
		"Use FragmentContainerView instead of the <fragment> tag", ALCCorrectness, ALSWarning, 5)}
	api.Register(&api.Rule{
		ID: tag.RuleName, Category: tag.RuleSetName, Description: tag.Description(), Sev: api.Severity(tag.Sev),
		Needs: api.NeedsResources, AndroidDeps: uint32(tag.AndroidDependencies()), Confidence: tag.Confidence(),
		Implementation: tag, NeedsLibraryFacts: true, Check: tag.check,
	})
}

func (r *FragmentGradleConfigurationRule) check(ctx *api.Context) {
	if !strings.Contains(ctx.GradleContent, "androidx.fragment:fragment-testing") {
		return
	}
	for _, st := range lintGradleStatements(ctx.GradleContent) {
		// Lint reports any assignment of such a string, ext constants and local variables included; krit only reads
		// dependency declarations, where the configuration is the statement's name.
		if st.Parent != "dependencies" || st.Name == "debugImplementation" {
			continue
		}
		// Lint reads only a bare string literal, so platform(...), map notation and catalog accessors are skipped.
		coord, _, ok := lintQuotedLiteral(st.Value)
		if !ok || !strings.HasPrefix(coord, "androidx.fragment:fragment-testing") {
			continue
		}
		f := baseFinding(ctx.GradlePath, st.Line, r.BaseRule, "Replace with debugImplementation.")
		f.Col = st.Col
		ctx.Emit(f)
	}
}

// fragmentCoordinates lists artifacts that bring androidx.fragment, whose AAR ships fragment-lint.
var fragmentCoordinates = []librarymodel.Coordinate{
	{Group: "androidx.fragment", Name: "fragment"},
	{Group: "androidx.fragment", Name: "fragment-ktx"},
	{Group: "androidx.fragment", Name: "fragment-compose"},
	{Group: "androidx.appcompat", Name: "appcompat"},
	{Group: "com.google.android.material", Name: "material"},
	{Group: "androidx.navigation", Name: "navigation-fragment"},
	{Group: "androidx.navigation", Name: "navigation-fragment-ktx"},
	{Group: "androidx.preference", Name: "preference"},
	{Group: "androidx.preference", Name: "preference-ktx"},
}

func (r *FragmentTagUsageRule) check(ctx *api.Context) {
	if ctx.ResourceIndex == nil || !librarymodel.EnsureFacts(ctx.LibraryFacts).Profile.HasAnyDependency(fragmentCoordinates...) {
		return
	}
	for _, layout := range ctx.ResourceIndex.AllLayouts() {
		walkViewsUnlessIgnored(layout.RootView, []string{r.RuleName}, func(v *android.View) {
			if v.Type == "fragment" {
				// Lint reports at the tag name, one column after `<`.
				ctx.Emit(scanner.Finding{File: layout.FilePath, Line: v.Line, Col: v.Col + 1,
					Message: "Replace the <fragment> tag with FragmentContainerView."})
			}
		})
	}
}
