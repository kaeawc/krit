package api

// ResolveActiveRulesAtDepth returns the input slice unchanged when
// thorough is false or no rule opts in via ThoroughOnlyNeeds. Otherwise
// it returns a new slice where opted-in rules are shallow copies with
// `Needs |= ThoroughOnlyNeeds` and `ThoroughOnlyNeeds = 0` (so the
// projection is idempotent); other entries share the original pointer.
//
// The clone-vs-mutate split exists because the global Registry is
// shared across scans — daemon mode runs balanced and thorough requests
// against the same Rule pointers and must not leak extra Needs bits
// across calls.
func ResolveActiveRulesAtDepth(rules []*Rule, thorough bool) []*Rule {
	if !thorough || len(rules) == 0 {
		return rules
	}
	var out []*Rule
	for i, r := range rules {
		if r == nil || r.ThoroughOnlyNeeds == 0 {
			continue
		}
		if out == nil {
			out = make([]*Rule, len(rules))
			copy(out, rules)
		}
		cp := *r
		cp.Needs |= r.ThoroughOnlyNeeds
		cp.ThoroughOnlyNeeds = 0
		out[i] = &cp
	}
	if out == nil {
		return rules
	}
	return out
}
