package api

import "testing"

func TestStabilityString(t *testing.T) {
	cases := []struct {
		in   Stability
		want string
	}{
		{StabilityUnset, "unset"},
		{StabilityEvolving, "evolving"},
		{StabilityStable, "stable"},
		{StabilityFrozen, "frozen"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Stability(%d).String() = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestStabilityOrdering(t *testing.T) {
	// Values must be ordered least-committed -> most-committed so
	// callers can filter with comparisons (e.g. < StabilityStable).
	ordered := []Stability{
		StabilityUnset,
		StabilityEvolving,
		StabilityStable,
		StabilityFrozen,
	}
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1] >= ordered[i] {
			t.Fatalf("ordering broken at index %d: %v not before %v", i, ordered[i-1], ordered[i])
		}
	}
}

func TestParseStability(t *testing.T) {
	cases := []struct {
		in   string
		want Stability
		ok   bool
	}{
		{"evolving", StabilityEvolving, true},
		{"stable", StabilityStable, true},
		{"frozen", StabilityFrozen, true},
		{"", StabilityUnset, false},
		{"unset", StabilityUnset, false},
		{"bogus", StabilityUnset, false},
	}
	for _, c := range cases {
		got, ok := ParseStability(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseStability(%q) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
