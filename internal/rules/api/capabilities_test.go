package api

import (
	"reflect"
	"sort"
	"testing"
)

func TestCapabilitiesList_StableOrder(t *testing.T) {
	caps := NeedsCrossFile | NeedsResolver | NeedsOracleCallTargets
	got := caps.List()
	want := []string{"cross-file", "oracle:call-targets", "resolver"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("List() = %v, want %v", got, want)
	}

	// Repeated calls yield the same order.
	if !reflect.DeepEqual(caps.List(), got) {
		t.Error("List() is not stable across calls")
	}

	// Output is sorted lexicographically.
	if !sort.StringsAreSorted(got) {
		t.Errorf("List() output is not sorted: %v", got)
	}
}

func TestCapabilitiesList_ZeroIsNil(t *testing.T) {
	if got := Capabilities(0).List(); got != nil {
		t.Errorf("zero capabilities should list as nil, got %v", got)
	}
}

func TestCapabilitiesList_RoundTrip(t *testing.T) {
	// Every individual capability bit declared in capabilityLabels must
	// round-trip Capabilities → labels → Capabilities without loss.
	for _, entry := range capabilityLabels {
		labels := entry.Bit.List()
		parsed, unknown := ParseCapabilities(labels)
		if len(unknown) != 0 {
			t.Errorf("bit %q: unexpected unknown labels %v", entry.Label, unknown)
		}
		if parsed != entry.Bit {
			t.Errorf("bit %q: round-trip mismatch, got %b want %b", entry.Label, parsed, entry.Bit)
		}
	}

	// A composite bitfield round-trips too.
	composite := NeedsResolver | NeedsCrossFile | NeedsOracleCallTargets |
		NeedsOracleExprType | NeedsConcurrent
	labels := composite.List()
	parsed, unknown := ParseCapabilities(labels)
	if len(unknown) != 0 {
		t.Errorf("composite: unexpected unknown labels %v", unknown)
	}
	if parsed != composite {
		t.Errorf("composite round-trip mismatch: got %b want %b", parsed, composite)
	}
}

func TestParseCapabilities_OracleGroup(t *testing.T) {
	caps, unknown := ParseCapabilities([]string{"oracle"})
	if len(unknown) != 0 {
		t.Fatalf("unexpected unknown labels: %v", unknown)
	}
	if caps != NeedsOracle {
		t.Errorf("ParseCapabilities([\"oracle\"]) = %b, want %b", caps, NeedsOracle)
	}
}

func TestParseCapabilities_Unknown(t *testing.T) {
	caps, unknown := ParseCapabilities([]string{"resolver", "made-up", "oracle:call-targets"})
	if caps != NeedsResolver|NeedsOracleCallTargets {
		t.Errorf("unexpected caps: %b", caps)
	}
	if !reflect.DeepEqual(unknown, []string{"made-up"}) {
		t.Errorf("unknown = %v, want [made-up]", unknown)
	}
}

func TestCapabilityFilter_RequireExclude(t *testing.T) {
	tests := []struct {
		name   string
		filter CapabilityFilter
		caps   Capabilities
		match  bool
	}{
		{
			name:   "zero filter matches everything",
			filter: CapabilityFilter{},
			caps:   NeedsCrossFile | NeedsOracleCallTargets,
			match:  true,
		},
		{
			name:   "require single bit matches",
			filter: CapabilityFilter{Require: []string{"resolver"}},
			caps:   NeedsResolver | NeedsCrossFile,
			match:  true,
		},
		{
			name:   "require single bit rejects when absent",
			filter: CapabilityFilter{Require: []string{"resolver"}},
			caps:   NeedsCrossFile,
			match:  false,
		},
		{
			name:   "exclude single bit rejects when present",
			filter: CapabilityFilter{Exclude: []string{"resolver"}},
			caps:   NeedsResolver | NeedsCrossFile,
			match:  false,
		},
		{
			name:   "exclude single bit allows when absent",
			filter: CapabilityFilter{Exclude: []string{"resolver"}},
			caps:   NeedsCrossFile,
			match:  true,
		},
		{
			name:   "exclude oracle group rejects any narrow bit",
			filter: CapabilityFilter{Exclude: []string{"oracle"}},
			caps:   NeedsOracleExprType,
			match:  false,
		},
		{
			name:   "exclude oracle group passes non-oracle rules",
			filter: CapabilityFilter{Exclude: []string{"oracle"}},
			caps:   NeedsResolver | NeedsCrossFile,
			match:  true,
		},
		{
			name:   "require oracle group accepts any narrow bit",
			filter: CapabilityFilter{Require: []string{"oracle"}},
			caps:   NeedsOracleSupertypes,
			match:  true,
		},
		{
			name:   "require multiple labels all must match",
			filter: CapabilityFilter{Require: []string{"resolver", "cross-file"}},
			caps:   NeedsResolver | NeedsCrossFile,
			match:  true,
		},
		{
			name:   "require multiple labels rejects partial",
			filter: CapabilityFilter{Require: []string{"resolver", "cross-file"}},
			caps:   NeedsResolver,
			match:  false,
		},
		{
			name:   "exclude with unknown label is a no-op for that label",
			filter: CapabilityFilter{Exclude: []string{"made-up"}},
			caps:   NeedsResolver,
			match:  true,
		},
		{
			name:   "require with unknown label rejects all",
			filter: CapabilityFilter{Require: []string{"made-up"}},
			caps:   NeedsResolver,
			match:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.filter.Match(tt.caps); got != tt.match {
				t.Errorf("Match(%b) = %v, want %v", tt.caps, got, tt.match)
			}
		})
	}
}

func TestCapabilityFilter_MatchRule(t *testing.T) {
	r := &Rule{ID: "x", Needs: NeedsResolver | NeedsOracleCallTargets}
	f := CapabilityFilter{Require: []string{"resolver"}, Exclude: []string{"oracle"}}
	if f.MatchRule(r) {
		t.Error("rule with oracle bit must not pass Exclude oracle")
	}

	f2 := CapabilityFilter{Require: []string{"resolver"}}
	if !f2.MatchRule(r) {
		t.Error("rule with resolver bit must satisfy Require resolver")
	}

	zero := CapabilityFilter{}
	if !zero.MatchRule(nil) {
		t.Error("zero filter must match nil rule")
	}
	require := CapabilityFilter{Require: []string{"resolver"}}
	if require.MatchRule(nil) {
		t.Error("nil rule must not satisfy a non-trivial filter")
	}
}

func TestRule_CapabilitiesList(t *testing.T) {
	r := &Rule{ID: "x", Needs: NeedsCrossFile | NeedsResolver}
	if !reflect.DeepEqual(r.CapabilitiesList(), []string{"cross-file", "resolver"}) {
		t.Errorf("CapabilitiesList = %v", r.CapabilitiesList())
	}

	var _ CapabilityProvider = r // compile-time check

	if (*Rule)(nil).CapabilitiesList() != nil {
		t.Error("nil rule should produce nil list")
	}
}

func TestKnownCapabilityLabels_ContainsAllBitsAndGroups(t *testing.T) {
	labels := KnownCapabilityLabels()
	seen := make(map[string]bool, len(labels))
	for _, l := range labels {
		seen[l] = true
	}
	for _, entry := range capabilityLabels {
		if !seen[entry.Label] {
			t.Errorf("missing bit label %q", entry.Label)
		}
	}
	for label := range capabilityGroups {
		if !seen[label] {
			t.Errorf("missing group label %q", label)
		}
	}
	if !sort.StringsAreSorted(labels) {
		t.Error("KnownCapabilityLabels output not sorted")
	}
}
