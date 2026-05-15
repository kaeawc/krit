package api

import "testing"

func TestEffortString(t *testing.T) {
	cases := []struct {
		in   Effort
		want string
	}{
		{EffortUnset, "unset"},
		{EffortTrivial, "trivial"},
		{EffortLocal, "local"},
		{EffortRefactor, "refactor"},
		{EffortArchitectural, "architectural"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Effort(%d).String() = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestEffortOrdering(t *testing.T) {
	// Values must be ordered easiest -> hardest so `triage --max-effort`
	// can filter with >= comparisons.
	ordered := []Effort{
		EffortUnset,
		EffortTrivial,
		EffortLocal,
		EffortRefactor,
		EffortArchitectural,
	}
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1] >= ordered[i] {
			t.Fatalf("ordering broken at %d: %v not before %v", i, ordered[i-1], ordered[i])
		}
	}
}

func TestParseEffort(t *testing.T) {
	cases := []struct {
		in   string
		want Effort
		ok   bool
	}{
		{"trivial", EffortTrivial, true},
		{"local", EffortLocal, true},
		{"refactor", EffortRefactor, true},
		{"architectural", EffortArchitectural, true},
		{"", EffortUnset, false},
		{"unset", EffortUnset, false},
		{"bogus", EffortUnset, false},
	}
	for _, c := range cases {
		got, ok := ParseEffort(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseEffort(%q) = (%v,%v), want (%v,%v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
