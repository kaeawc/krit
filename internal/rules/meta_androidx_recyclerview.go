package rules

import "github.com/kaeawc/krit/internal/rules/api"

func (r *InvalidSetHasFixedSizeRule) Meta() api.RuleDescriptor {
	return api.RuleDescriptor{ID: "InvalidSetHasFixedSize", RuleSet: "android-lint", DefaultActive: true,
		KnownLimitations: []string{
			"The receiver must be traced to findViewById(R.id.x) through an initializer, a cast, a type argument or a single same-file assignment, and proven to be a RecyclerView; view-binding receivers are not checked (Lint does not check them either).",
			"Lint reports every setHasFixedSize(true) on a RecyclerView in a scope that also looks up such an id, even on another view, and once per matching lookup; krit reports a call once and only for its own receiver.",
		}}
}
