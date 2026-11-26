package rules

import (
	"github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// registerAndroidXWorkCallsiteChecks registers the call-site checks from androidx.work:work-runtime 2.12.0.
func registerAndroidXWorkCallsiteChecks() {
	calls := lintCallNodeTypes[:2]
	idle := &IdleBatteryChargingConstraintsRule{AndroidRule: lintAndroidRule("IdleBatteryChargingConstraints",
		"Constraints may not be met for some devices", ALCCorrectness, ALSWarning, 5)}
	registerWorkRule(idle.AndroidRule, idle, idle.Confidence(), idle.check, calls)

	interval := &InvalidPeriodicWorkRequestIntervalRule{AndroidRule: lintAndroidRule("InvalidPeriodicWorkRequestInterval",
		"Invalid interval duration", ALCCorrectness, ALSFatal, 5)}
	registerWorkRule(interval.AndroidRule, interval, interval.Confidence(), interval.check,
		[]string{"call_expression", "object_creation_expression"})

	enqueue := &BadPeriodicWorkRequestEnqueueRule{AndroidRule: lintAndroidRule("BadPeriodicWorkRequestEnqueue",
		"Use `enqueueUniquePeriodicWork()` instead of `enqueue()`", ALCCorrectness, ALSWarning, 5)}
	registerWorkRule(enqueue.AndroidRule, enqueue, enqueue.Confidence(), enqueue.check, calls)

	rx := &UseRxSetProgress2Rule{AndroidRule: lintAndroidRule("UseRxSetProgress2",
		"`setProgress` is deprecated. Use `setCompletableProgress` instead.", ALCCorrectness, ALSFatal, 5)}
	api.Register(&api.Rule{
		ID: rx.RuleName, Category: rx.RuleSetName, Description: rx.Description(), Sev: api.Severity(rx.Sev),
		NodeTypes: calls, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: rx.Confidence(), Implementation: rx, Fix: api.FixIdiomatic, NeedsLibraryFacts: true, OptionalResolver: true,
		Check: rx.check,
	})
}

// registerAndroidXWorkProjectChecks registers the project-wide checks from androidx.work:work-runtime 2.12.0.
func registerAndroidXWorkProjectChecks() {
	config := &BadConfigurationProviderRule{AndroidRule: lintAndroidRule("BadConfigurationProvider",
		"Invalid WorkManager Configuration Provider", ALCCorrectness, ALSFatal, 5)}
	registerWorkProjectRule(config.AndroidRule, config, config.Confidence(), config.check)

	jobs := &SpecifyJobSchedulerIDRangeRule{AndroidRule: lintAndroidRule("SpecifyJobSchedulerIdRange",
		"Specify a range of JobScheduler ids", ALCCorrectness, ALSWarning, 5)}
	registerWorkProjectRule(jobs.AndroidRule, jobs, jobs.Confidence(), jobs.check)

	workers := &WorkerHasAPublicModifierRule{AndroidRule: lintAndroidRule("WorkerHasAPublicModifier",
		"ListenableWorkers constructed using the default WorkerFactories need to be public", ALCCorrectness, ALSFatal, 5)}
	registerWorkProjectRule(workers.AndroidRule, workers, workers.Confidence(), workers.check)
}

// registerAndroidXWorkManifestChecks registers the androidx.work:work-runtime 2.12.0 checks that join source with
// the manifests.
func registerAndroidXWorkManifestChecks() {
	initializer := &RemoveWorkManagerInitializerRule{AndroidRule: lintAndroidRule("RemoveWorkManagerInitializer",
		"Remove androidx.work.WorkManagerInitializer from your AndroidManifest.xml when using on-demand initialization.",
		ALCCorrectness, ALSFatal, 5)}
	registerWorkManifestRule(initializer.AndroidRule, initializer, initializer.Confidence(), initializer.check)

	foreground := &SpecifyForegroundServiceTypeRule{AndroidRule: lintAndroidRule("SpecifyForegroundServiceType",
		"Specify foreground service type", ALCCorrectness, ALSFatal, 5)}
	registerWorkManifestRule(foreground.AndroidRule, foreground, foreground.Confidence(), foreground.check)
}

// registerWorkManifestRule registers a WorkManager Lint port in the manifest-source scope.
func registerWorkManifestRule(rule AndroidRule, impl any, confidence float64, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		Needs: api.NeedsManifest | api.NeedsParsedFiles, AndroidDeps: uint32(AndroidDepManifest),
		Languages:  []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: confidence, Implementation: impl, NeedsLibraryFacts: true, Check: check,
	})
}

// registerWorkProjectRule registers a WorkManager Lint port that runs once over every parsed Kotlin and Java file.
func registerWorkProjectRule(rule AndroidRule, impl any, confidence float64, check func(*api.Context)) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		Needs: api.NeedsParsedFiles, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: confidence, Implementation: impl, NeedsLibraryFacts: true, Check: check,
	})
}

// registerWorkRule registers a WorkManager Lint port for Kotlin and Java, skipping test sources as Lint does.
func registerWorkRule(rule AndroidRule, impl any, confidence float64, check func(*api.Context), nodeTypes []string) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		NodeTypes: nodeTypes, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: confidence, Implementation: impl, NeedsLibraryFacts: true,
		Check: func(ctx *api.Context) {
			if ctx.File != nil && scanner.IsTestFile(projectPath(ctx, ctx.File.Path)) {
				return
			}
			check(ctx)
		},
	})
}
