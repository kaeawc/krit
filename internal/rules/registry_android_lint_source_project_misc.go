package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidLintSourceProjectMiscChecks registers the Android Lint 32.4.1 RecyclerView, FindViewByIdCast,
// RemoteViewLayout, DefaultUncaughtExceptionDelegation and WrongGradleMethod checks.
func registerAndroidLintSourceProjectMiscChecks() {
	recycler := &RecyclerViewRule{AndroidRule: lintAndroidRule("RecyclerView", "RecyclerView Problems", ALCCorrectness, ALSError, 8)}
	registerLintRule(recycler.AndroidRule, recycler, recycler.Confidence(), recycler.check, []string{"class_declaration"}, 0, nil)

	cast := &FindViewByIDCastRule{AndroidRule: lintAndroidRule("FindViewByIdCast", "Add Explicit Cast", ALCCorrectness, ALSWarning, 9)}
	api.Register(&api.Rule{
		ID: cast.RuleName, Category: cast.RuleSetName, Description: cast.Description(), Sev: api.Severity(cast.Sev),
		NodeTypes: []string{"method_invocation"}, Languages: []scanner.Language{scanner.LangJava},
		Confidence: cast.Confidence(), Implementation: cast, Fix: api.FixIdiomatic, Check: lintSkipTestSources(cast.check),
	})

	remote := &RemoteViewLayoutRule{AndroidRule: lintAndroidRule("RemoteViewLayout", "Unsupported View in RemoteView",
		ALCCorrectness, ALSError, 6)}
	api.Register(&api.Rule{
		ID: remote.RuleName, Category: remote.RuleSetName, Description: remote.Description(), Sev: api.Severity(remote.Sev),
		NodeTypes: []string{"call_expression", "object_creation_expression"}, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Needs: api.NeedsResources, AndroidDeps: uint32(AndroidDepLayout), Confidence: remote.Confidence(), Implementation: remote,
		Check: lintSkipTestSources(remote.check),
	})

	uncaught := &DefaultUncaughtExceptionDelegationRule{AndroidRule: lintAndroidRule("DefaultUncaughtExceptionDelegation",
		"Missing default uncaught exception handler delegation", ALCCorrectness, ALSWarning, 5)}
	api.Register(&api.Rule{
		ID: uncaught.RuleName, Category: uncaught.RuleSetName, Description: uncaught.Description(), Sev: api.Severity(uncaught.Sev),
		Needs: api.NeedsParsedFiles, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: uncaught.Confidence(), Implementation: uncaught, Check: uncaught.check,
	})

	gradle := &WrongGradleMethodRule{AndroidRule: lintAndroidRule("WrongGradleMethod", "Wrong Gradle method invoked",
		ALCCorrectness, ALSError, 6)}
	registerLintGradleRule(gradle.AndroidRule, gradle, gradle.Confidence(), gradle.check)
}
