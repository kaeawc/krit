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

// registerWorkRule registers a WorkManager Lint port for Kotlin and Java, skipping test sources as Lint does.
func registerWorkRule(rule AndroidRule, impl any, confidence float64, check func(*api.Context), nodeTypes []string) {
	api.Register(&api.Rule{
		ID: rule.RuleName, Category: rule.RuleSetName, Description: rule.Description(), Sev: api.Severity(rule.Sev),
		NodeTypes: nodeTypes, Languages: []scanner.Language{scanner.LangKotlin, scanner.LangJava},
		Confidence: confidence, Implementation: impl, NeedsLibraryFacts: true,
		Check: func(ctx *api.Context) {
			if ctx.File != nil && scanner.IsTestFile(ctx.File.Path) {
				return
			}
			check(ctx)
		},
	})
}
