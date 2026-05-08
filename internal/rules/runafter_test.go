package rules

import (
	"reflect"
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
