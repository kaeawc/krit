package rules

import (
	"strings"

	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// Port of GradleDebugConfigurationDetector from androidx.compose.ui:ui-test-manifest 1.12.1.

// TestManifestGradleConfigurationRule ports TestManifestGradleConfiguration: ui-test-manifest only needs to be on
// the debug classpath, so it should be declared with debugImplementation.
type TestManifestGradleConfigurationRule struct {
	GradleBase
	BaseRule
}

// Confidence reports a high base confidence: the check reads literal coordinates and configuration names.
func (r *TestManifestGradleConfigurationRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *TestManifestGradleConfigurationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "TestManifestGradleConfiguration", RuleSet: "compose", DefaultActive: true,
		KnownLimitations: []string{
			"As in Lint, only a bare string coordinate is read: version-catalog accessors, platform(...), map " +
				"notation and declarations with a configuration closure are not checked.",
		}}
}

const acuiTestManifestCoordinate = "androidx.compose.ui:ui-test-manifest"

// acuiTestManifestConfigurations are the configuration names (lowercased) Lint looks for in a statement's name.
var acuiTestManifestConfigurations = []string{
	"implementation", "api", "compileonly", "runtimeonly", "annotationprocessor", "lintchecks", "lintpublish",
}

func registerAndroidxComposeUITestManifestRules() {
	r := &TestManifestGradleConfigurationRule{BaseRule: BaseRule{RuleName: "TestManifestGradleConfiguration",
		RuleSetName: "compose", Sev: "warning",
		Desc: "Detects androidx.compose.ui:ui-test-manifest declared on a configuration other than debugImplementation."}}
	api.Register(&api.Rule{
		ID: r.RuleName, Category: r.RuleSetName, Description: r.Desc, Sev: api.Severity(r.Sev),
		Needs: api.NeedsGradle, AndroidDeps: uint32(AndroidDepGradle), Confidence: r.Confidence(), Fix: api.FixSemantic,
		Scripts: api.ScriptsGradle, Implementation: r, Check: r.check, LintIssue: true,
	})
}

// check mirrors GradleDebugConfigurationDetector.checkDslPropertyAssignment: any statement (Lint does not look at
// the enclosing block) whose value is a bare string starting with the ui-test-manifest coordinate, and whose name,
// lowercased, contains a configuration name but not debugimplementation and starts with a configuration name,
// `android` or `test`. So testImplementation and androidTestImplementation are reported, and so is an `ext`
// assignment such as `implementationOfManifest = '...'`; releaseImplementation and testDebugImplementation are not.
func (r *TestManifestGradleConfigurationRule) check(ctx *api.Context) {
	content := ctx.GradleContent
	if !strings.Contains(content, acuiTestManifestCoordinate) {
		return
	}
	lineStarts := acuiLineStarts(content)
	for _, st := range lintGradleStatements(content) {
		coord, _, ok := lintQuotedLiteral(st.Value)
		if !ok || !strings.HasPrefix(coord, acuiTestManifestCoordinate) || !acuiTestManifestConfigurationName(st.Name) {
			continue
		}
		f := baseFindingAt(ctx.GradlePath, st.Line, st.Col, r.BaseRule, "Please use debugImplementation.")
		if st.Line >= 1 && st.Line <= len(lineStarts) {
			start := lineStarts[st.Line-1] + st.Col - 1
			if start >= 0 && start+len(st.Name) <= len(content) && content[start:start+len(st.Name)] == st.Name {
				f.Fix = &scanner.Fix{ByteMode: true, StartByte: start, EndByte: start + len(st.Name),
					Replacement: "debugImplementation"}
			}
		}
		ctx.Emit(f)
	}
}

// acuiTestManifestConfigurationName applies Lint's configuration-name test to a statement name.
func acuiTestManifestConfigurationName(name string) bool {
	lower := strings.ToLower(name)
	if strings.Contains(lower, "debugimplementation") {
		return false
	}
	contains, prefixed := false, strings.HasPrefix(lower, "android") || strings.HasPrefix(lower, "test")
	for _, c := range acuiTestManifestConfigurations {
		if strings.Contains(lower, c) {
			contains = true
		}
		if strings.HasPrefix(lower, c) {
			prefixed = true
		}
	}
	return contains && prefixed
}

// acuiLineStarts returns the byte offset at which each line of content starts.
func acuiLineStarts(content string) []int {
	starts := []int{0}
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return starts
}
