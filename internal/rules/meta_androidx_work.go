package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *IdleBatteryChargingConstraintsRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "IdleBatteryChargingConstraints", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Both calls need an explicit receiver proven to be a Constraints.Builder by a constructor, a builder-setter chain, or a declaration in the same file.",
		}}
}

func (r *InvalidPeriodicWorkRequestIntervalRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidPeriodicWorkRequestInterval", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Follows the detector source: the Android Lint CLI (9.4) running the 2.12.0 lint.jar against compiled WorkManager reported none of these intervals, so Lint-side parity could not be observed.",
			"Like the detector, a Java int argument (`10` instead of `10L`) is not evaluated, and PeriodicWorkRequestBuilder<W>(...) is left to PeriodicWorkRequestLessThan15Min.",
		}}
}

func (r *BadPeriodicWorkRequestEnqueueRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "BadPeriodicWorkRequestEnqueue", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Reported at the enqueue() call; Lint 2.12.0 reports the location of WorkManager.enqueue in the library jar.",
			"A Kotlin List<WorkRequest> argument is not reported, though Lint's List<? extends WorkRequest> match reports it even for one-time requests.",
		}}
}

func (r *UseRxSetProgress2Rule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "UseRxSetProgress2", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"An explicit receiver must be declared in the same file as androidx.work.RxWorker or a same-file subclass of it.",
		}}
}
