package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *RecyclerViewRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RecyclerView", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The adapter supertype is followed through same-file classes, the resolver and the known ListAdapter, PagingDataAdapter and PagedListAdapter; names are matched textually, so a local that shadows a field is not told apart.",
		}}
}

func (r *FindViewByIDCastRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "FindViewByIdCast", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The surrounding method must be a known generic identity method (Objects.requireNonNull, the Guava and AndroidX checkNotNull, Verify.verifyNotNull) or declared generically in the same file.",
		}}
}

func (r *RemoteViewLayoutRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "RemoteViewLayout", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The layout is looked up in every res directory of the scan by file name; the minimum SDK comes from a literal minSdk in the build script next to the source set.",
		}}
}

func (r *DefaultUncaughtExceptionDelegationRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "DefaultUncaughtExceptionDelegation", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The setter's receiver must be written as Thread or java.lang.Thread; modules are the directories of the nearest build script.",
		}}
}

func (r *WrongGradleMethodRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "WrongGradleMethod", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"Only the patterns Lint checks without a resolved build classpath are ported: a dependencies block inside a build type or product flavor, and firebaseAppDistribution in a .kts script without its import. Lint in the IDE also compares receiver types.",
		}}
}
