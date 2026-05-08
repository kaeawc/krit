package rules

import (
	"sort"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// SortByRunAfter returns rules reordered so each rule's RunAfter
// dependencies precede it. The sort is stable: rules that are unrelated
// to any dependency keep their original relative order. Nil entries are
// dropped (the dispatcher already filters them).
//
// Dependencies that are not present in rules are silently ignored — a
// rule can declare RunAfter on a constraint that only applies in some
// rule sets without becoming a hard requirement.
//
// A cycle among active rules is a programmer error. SortByRunAfter
// panics with the cycle's rule IDs so the offending registration shows
// up in tests rather than as a silent runtime drift.
func SortByRunAfter(rules []*api.Rule) []*api.Rule {
	n := 0
	for _, r := range rules {
		if r != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}

	byID := make(map[string]*api.Rule, n)
	pos := make(map[string]int, n)
	order := make([]*api.Rule, 0, n)
	for i, r := range rules {
		if r == nil {
			continue
		}
		if _, dup := byID[r.ID]; dup {
			// Duplicate IDs in the registry are a separate bug; preserve
			// the first occurrence and drop the rest so the topo sort
			// has a deterministic input.
			continue
		}
		byID[r.ID] = r
		pos[r.ID] = i
		order = append(order, r)
	}

	indeg := make(map[string]int, len(order))
	dependents := make(map[string][]string, len(order))
	for _, r := range order {
		for _, dep := range r.RunAfter {
			if _, ok := byID[dep]; !ok {
				continue
			}
			if dep == r.ID {
				continue // self-edges are a no-op
			}
			indeg[r.ID]++
			dependents[dep] = append(dependents[dep], r.ID)
		}
	}

	// Kahn's algorithm with a stable tiebreaker: ready entries are
	// processed in original-input order so unrelated rules keep their
	// registry sequence.
	ready := make([]string, 0, len(order))
	for _, r := range order {
		if indeg[r.ID] == 0 {
			ready = append(ready, r.ID)
		}
	}

	out := make([]*api.Rule, 0, len(order))
	processed := make(map[string]bool, len(order))
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		if processed[id] {
			continue
		}
		processed[id] = true
		out = append(out, byID[id])

		deps := dependents[id]
		sort.SliceStable(deps, func(i, j int) bool {
			return pos[deps[i]] < pos[deps[j]]
		})
		newReady := make([]string, 0, len(deps))
		for _, dep := range deps {
			indeg[dep]--
			if indeg[dep] == 0 {
				newReady = append(newReady, dep)
			}
		}
		// Insert new-ready entries at the head in original-input order
		// so they queue before any already-pending entries with later
		// positions. This matches the stable ordering guarantee.
		ready = append(newReady, ready...)
		sort.SliceStable(ready, func(i, j int) bool {
			return pos[ready[i]] < pos[ready[j]]
		})
	}

	if len(out) != len(order) {
		var cycle []string
		for _, r := range order {
			if !processed[r.ID] {
				cycle = append(cycle, r.ID)
			}
		}
		sort.Strings(cycle)
		panic("rules: RunAfter cycle detected among: " + strings.Join(cycle, ", "))
	}
	return out
}
