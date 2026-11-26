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

func (r *BadConfigurationProviderRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "BadConfigurationProvider", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Class hierarchies are followed through the project's Kotlin and Java sources and a few library classes (MultiDexApplication); an Application whose Configuration.Provider comes from another library base class is not recognized.",
			"Lint reports the whole file of the last Application subclass it visits; krit anchors the finding at that file's first line. Lint runs per root project, krit over every module of the scan.",
		}}
}

func (r *SpecifyJobSchedulerIDRangeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SpecifyJobSchedulerIdRange", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Needs evidence that the project uses WorkManager (an androidx.work dependency or import), because a JobService alone does not bring the work lint.jar.",
			"Any setJobSchedulerJobIdRange call counts, without proving its receiver is a Configuration.Builder.",
		}}
}

func (r *WorkerHasAPublicModifierRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WorkerHasAPublicModifier", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Any setWorkerFactory call in the project counts as a custom WorkerFactory, without proving its receiver is a Configuration.Builder.",
			"Worker hierarchies are followed through project sources and the androidx.work worker base classes.",
		}}
}

func (r *RemoveWorkManagerInitializerRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RemoveWorkManagerInitializer", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"A tools:node=\"remove\" in any manifest of the scan counts; the location comes from the manifests of the Application's own module (nearest build file).",
			"Class hierarchies are followed through project sources; a Configuration.Provider inherited from another library's base class is not seen.",
		}}
}

func (r *SpecifyForegroundServiceTypeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "SpecifyForegroundServiceType", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Service types are collected from every manifest of the scan, and nothing is reported when the scan has no manifest.",
			"The type argument must fold to a constant from literals, ServiceInfo constants and same-file constants.",
		}}
}
