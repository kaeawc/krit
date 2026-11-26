package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *WebViewAPIAvailabilityRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WebViewApiAvailability", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The guard must be an enclosing if on Build.VERSION.SDK_INT (then branch of >= / >, else branch of < / <=); like Lint here, early returns and @RequiresApi do not count.",
		}}
}

func (r *EnqueueWorkRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "EnqueueWork", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The continuation is followed through chained calls, locals and lists within the function, as Lint's data-flow analysis does; the WorkManager or WorkContinuation receiver must be visible in source.",
		}}
}

func (r *ReportShortcutUsageRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "ReportShortcutUsage", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"A ShortcutManager receiver for setDynamicShortcuts/addDynamicShortcuts must be a variable declared as one or initialized from getSystemService (any pushDynamicShortcut or reportShortcutUsed call counts as tracking usage); the scan counts as an app unless every module with shortcut calls is a library.",
		}}
}
