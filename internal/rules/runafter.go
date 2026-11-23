package rules

import (
	"container/heap"
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
	order := make([]*api.Rule, 0, n)
	for _, r := range rules {
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
		order = append(order, r)
	}

	// Work on input indices so the queue is a cheap int min-heap rather
	// than a string slice re-sorted after every pop.
	idx := make(map[string]int, len(order))
	for i, r := range order {
		idx[r.ID] = i
	}
	indeg := make([]int, len(order))
	dependents := make([][]int, len(order))
	edges := 0
	for i, r := range order {
		for _, dep := range r.RunAfter {
			j, ok := idx[dep]
			if !ok || j == i {
				continue // unknown deps and self-edges are a no-op
			}
			indeg[i]++
			dependents[j] = append(dependents[j], i)
			edges++
		}
	}
	if edges == 0 {
		return order
	}

	// Kahn's algorithm with a stable tiebreaker: the ready entry with the
	// lowest original-input position is always processed next, so
	// unrelated rules keep their registry sequence. order is already in
	// input order, so its index doubles as the position key.
	ready := &intMinHeap{}
	for i := range order {
		if indeg[i] == 0 {
			heap.Push(ready, i)
		}
	}

	out := make([]*api.Rule, 0, len(order))
	processed := make([]bool, len(order))
	for ready.Len() > 0 {
		i := heap.Pop(ready).(int)
		if processed[i] {
			continue
		}
		processed[i] = true
		out = append(out, order[i])
		for _, dep := range dependents[i] {
			indeg[dep]--
			if indeg[dep] == 0 {
				heap.Push(ready, dep)
			}
		}
	}

	if len(out) != len(order) {
		var cycle []string
		for i, r := range order {
			if !processed[i] {
				cycle = append(cycle, r.ID)
			}
		}
		sort.Strings(cycle)
		panic("rules: RunAfter cycle detected among: " + strings.Join(cycle, ", "))
	}
	return out
}

type intMinHeap []int

func (h intMinHeap) Len() int           { return len(h) }
func (h intMinHeap) Less(i, j int) bool { return h[i] < h[j] }
func (h intMinHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }
func (h *intMinHeap) Push(x any)        { *h = append(*h, x.(int)) }
func (h *intMinHeap) Pop() any {
	old := *h
	x := old[len(old)-1]
	*h = old[:len(old)-1]
	return x
}
