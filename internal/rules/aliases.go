package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

// AllSuppressionAliases returns a snapshot of every rule's suppression
// aliases, keyed by canonical rule ID. The pipeline Parse phase passes
// this into scanner.SuppressionFilter.WithRuleAliases so an
// @Suppress("LegacyName") (or `// krit:ignore[LegacyName]`) silences
// findings emitted under the canonical ID.
//
// Returns a nil map when no registered rule declares any aliases. The
// scanner side treats nil and empty equivalently, so a nil result is a
// safe drop-in.
func AllSuppressionAliases() map[string][]string {
	var out map[string][]string
	for _, r := range api.Registry {
		if r == nil || len(r.Aliases) == 0 {
			continue
		}
		if out == nil {
			out = make(map[string][]string)
		}
		// Defensive copy: rules treat Aliases as immutable, but the
		// scanner caches the slice by reference and we don't want a
		// later registry mutation to bleed in.
		clone := make([]string, len(r.Aliases))
		copy(clone, r.Aliases)
		out[r.ID] = clone
	}
	return out
}
