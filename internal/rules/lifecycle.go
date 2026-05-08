package rules

import (
	api "github.com/kaeawc/krit/internal/rules/api"
)

// DeprecationFor returns the deprecation metadata for a rule, or
// (nil, false) if the rule is not deprecated or is unknown.
//
// Consumers (output formatters, CI gates, the docs generator) call this
// to surface ReplacedBy / Reason guidance when a deprecated rule is
// active.
func DeprecationFor(ruleID string) (*api.Deprecation, bool) {
	if ruleID == "" {
		return nil, false
	}
	for _, r := range api.Registry {
		if r == nil || r.ID != ruleID {
			continue
		}
		if r.Deprecated == nil {
			return nil, false
		}
		return r.Deprecated, true
	}
	return nil, false
}

// AllDeprecations returns a snapshot of every registered rule's
// Deprecation metadata, keyed by rule ID. Rules without a Deprecation
// are omitted; the result is nil when no rule is deprecated.
//
// The returned map and its values are not shared with the registry —
// the deprecation pointer is copied so callers can read it without
// holding a reference to a registered rule.
func AllDeprecations() map[string]api.Deprecation {
	var out map[string]api.Deprecation
	for _, r := range api.Registry {
		if r == nil || r.Deprecated == nil {
			continue
		}
		if out == nil {
			out = make(map[string]api.Deprecation)
		}
		out[r.ID] = *r.Deprecated
	}
	return out
}
