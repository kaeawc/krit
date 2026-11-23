package rules

import (
	"fmt"
	"math/rand/v2"
	"reflect"
	"sort"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func runAfterRule(id string, after ...string) *api.Rule {
	return &api.Rule{
		ID:          id,
		Description: "fake",
		RunAfter:    after,
		Check:       func(*api.Context) {},
	}
}

func runAfterIDs(rs []*api.Rule) []string {
	out := make([]string, len(rs))
	for i, r := range rs {
		out[i] = r.ID
	}
	return out
}

func TestSortByRunAfter_NoConstraintsKeepsOrder(t *testing.T) {
	in := []*api.Rule{runAfterRule("A"), runAfterRule("B"), runAfterRule("C")}
	got := runAfterIDs(SortByRunAfter(in))
	want := []string{"A", "B", "C"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestSortByRunAfter_DepRunsFirst(t *testing.T) {
	// B depends on A; both in registration order [B, A] → A must come first.
	in := []*api.Rule{runAfterRule("B", "A"), runAfterRule("A")}
	got := runAfterIDs(SortByRunAfter(in))
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestSortByRunAfter_StableForUnrelated(t *testing.T) {
	// X has no relationship with the A→B chain; it must keep its
	// original position relative to other unrelated rules.
	in := []*api.Rule{
		runAfterRule("X"),
		runAfterRule("B", "A"),
		runAfterRule("Y"),
		runAfterRule("A"),
	}
	got := runAfterIDs(SortByRunAfter(in))
	// A must precede B; X and Y keep registry order; A is allowed to
	// move earlier than its registration position to satisfy B's
	// constraint, but X (which has no dependency) must still appear
	// before Y because that was the registry order.
	if posOf(got, "A") >= posOf(got, "B") {
		t.Errorf("A must run before B; got %v", got)
	}
	if posOf(got, "X") >= posOf(got, "Y") {
		t.Errorf("X must keep its order before Y; got %v", got)
	}
}

func TestSortByRunAfter_MissingDepIsIgnored(t *testing.T) {
	// B depends on a rule that is not in the active set; the
	// constraint becomes a no-op and original order is preserved.
	in := []*api.Rule{runAfterRule("B", "GhostRule"), runAfterRule("A")}
	got := runAfterIDs(SortByRunAfter(in))
	want := []string{"B", "A"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v (missing deps must be ignored)", got, want)
	}
}

func TestSortByRunAfter_SelfEdgeIsIgnored(t *testing.T) {
	// A self-edge would otherwise make in-degree non-zero forever.
	in := []*api.Rule{runAfterRule("A", "A"), runAfterRule("B")}
	got := runAfterIDs(SortByRunAfter(in))
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestSortByRunAfter_DropsNilEntries(t *testing.T) {
	in := []*api.Rule{nil, runAfterRule("A"), nil, runAfterRule("B", "A")}
	got := runAfterIDs(SortByRunAfter(in))
	want := []string{"A", "B"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func TestSortByRunAfter_CyclePanics(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on cycle")
		}
		msg, ok := r.(string)
		if !ok || !strings.Contains(msg, "RunAfter cycle") {
			t.Fatalf("expected RunAfter cycle message, got %v", r)
		}
		if !strings.Contains(msg, "A") || !strings.Contains(msg, "B") {
			t.Fatalf("cycle message should name both rules, got %q", msg)
		}
	}()
	in := []*api.Rule{
		runAfterRule("A", "B"),
		runAfterRule("B", "A"),
	}
	SortByRunAfter(in)
}

func TestSortByRunAfter_TransitiveChain(t *testing.T) {
	// C depends on B; B depends on A. Output must be A, B, C even when
	// inputs are reversed.
	in := []*api.Rule{
		runAfterRule("C", "B"),
		runAfterRule("B", "A"),
		runAfterRule("A"),
	}
	got := runAfterIDs(SortByRunAfter(in))
	want := []string{"A", "B", "C"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("order = %v, want %v", got, want)
	}
}

func posOf(ss []string, s string) int {
	for i, v := range ss {
		if v == s {
			return i
		}
	}
	return -1
}

func BenchmarkSortByRunAfterRegistry(b *testing.B) {
	rules := append([]*api.Rule(nil), api.Registry...)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		SortByRunAfter(rules)
	}
}

// referenceSortByRunAfter is the original re-sort-per-pop implementation,
// kept as an oracle for the heap-based SortByRunAfter.
func referenceSortByRunAfter(rules []*api.Rule) []string {
	pos := map[string]int{}
	byID := map[string]*api.Rule{}
	var order []*api.Rule
	for i, r := range rules {
		if _, dup := byID[r.ID]; dup {
			continue
		}
		byID[r.ID], pos[r.ID] = r, i
		order = append(order, r)
	}
	indeg := map[string]int{}
	dependents := map[string][]string{}
	for _, r := range order {
		for _, dep := range r.RunAfter {
			if _, ok := byID[dep]; !ok || dep == r.ID {
				continue
			}
			indeg[r.ID]++
			dependents[dep] = append(dependents[dep], r.ID)
		}
	}
	var ready, out []string
	for _, r := range order {
		if indeg[r.ID] == 0 {
			ready = append(ready, r.ID)
		}
	}
	for len(ready) > 0 {
		id := ready[0]
		ready = ready[1:]
		out = append(out, id)
		for _, dep := range dependents[id] {
			indeg[dep]--
			if indeg[dep] == 0 {
				ready = append(ready, dep)
			}
		}
		sort.SliceStable(ready, func(i, j int) bool { return pos[ready[i]] < pos[ready[j]] })
	}
	return out
}

func TestSortByRunAfter_MatchesReferenceOnRandomDAGs(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	for trial := 0; trial < 200; trial++ {
		n := 1 + rng.IntN(40)
		// Edges only point from a higher rank to a lower one, so the graph is acyclic;
		// the input order is a random permutation of the ranks.
		perm := rng.Perm(n)
		rules := make([]*api.Rule, n)
		for i, rank := range perm {
			r := &api.Rule{ID: fmt.Sprintf("R%02d", rank)}
			for dep := 0; dep < rank; dep++ {
				if rng.IntN(6) == 0 {
					r.RunAfter = append(r.RunAfter, fmt.Sprintf("R%02d", dep))
				}
			}
			if rng.IntN(10) == 0 {
				r.RunAfter = append(r.RunAfter, "Missing", r.ID)
			}
			rules[i] = r
		}
		if rng.IntN(5) == 0 && n > 1 {
			rules = append(rules, &api.Rule{ID: rules[0].ID}) // duplicate ID: first one wins
		}
		want := referenceSortByRunAfter(rules)
		var got []string
		for _, r := range SortByRunAfter(rules) {
			got = append(got, r.ID)
		}
		if strings.Join(got, ",") != strings.Join(want, ",") {
			t.Fatalf("trial %d:\n got %v\nwant %v", trial, got, want)
		}
	}
}
