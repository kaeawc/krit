package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *WebViewAPIAvailabilityRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WebViewApiAvailability", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Guards that count: an enclosing if, when entry (the lowest level of its conditions, and the else after it), when (SDK_INT) { in N..M -> } or Java ternary on Build.VERSION.SDK_INT, in either operand order; the right side of && (or of || after a failing check); a Boolean local initialized with a check; and a call of a same-file @ChecksSdkIntAtLeast(api = N) or one-line check function, the one declared nearest the call (none when two are equally near). The walk goes through lambdas and anonymous objects.",
			"Not counted: check helpers declared in another file or a library (BuildCompat and the like), @ChecksSdkIntAtLeast's parameter and lambda forms, and, like Lint for this check, early returns and @RequiresApi.",
		}}
}

func (r *EnqueueWorkRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "EnqueueWork", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The continuation is followed as Lint's DataFlowAnalyzer does, within the function: through chained calls, also/apply/let/run/takeIf/takeUnless/with, the locals it initializes or is assigned to until they are assigned again (resolved by declaration, so a shadowing local, loop variable or lambda parameter is another variable), and the value of if/when/elvis/ternary branches. A value passed to any call other than combine, listOf included, is not followed, as in Lint. The WorkManager or WorkContinuation receiver must be visible in source.",
		}}
}

func (r *ReportShortcutUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ReportShortcutUsage", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"A ShortcutManager receiver must be proven in source: getSystemService(ShortcutManager::class.java / .class), getSystemService<ShortcutManager>(), a cast, `!!` of one, or a variable declared as one, initialized with one or delegated to lazy { one }. Any pushDynamicShortcut or reportShortcutUsed call counts as tracking usage.",
			"Each module is checked on its own, as Lint does with checkDependencies off (its default): an application module's calls are reported when that module reports no usage, and a library module is skipped.",
		}}
}
