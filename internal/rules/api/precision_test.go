package api

import "testing"

func TestPrecisionString(t *testing.T) {
	cases := []struct {
		in   Precision
		want string
	}{
		{PrecisionUnset, "unset"},
		{PrecisionHeuristicTextBacked, "heuristic/text-backed"},
		{PrecisionASTBacked, "ast-backed"},
		{PrecisionProjectStructure, "project-structure-aware"},
		{PrecisionTypeAware, "type-aware"},
		{PrecisionPolicy, "policy"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Precision(%d).String() = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestPrecisionOrdering(t *testing.T) {
	// Values must be ordered noisiest -> cleanest so callers can filter
	// with comparisons (e.g. >= PrecisionASTBacked).
	ordered := []Precision{
		PrecisionUnset,
		PrecisionHeuristicTextBacked,
		PrecisionASTBacked,
		PrecisionProjectStructure,
		PrecisionTypeAware,
		PrecisionPolicy,
	}
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1] >= ordered[i] {
			t.Fatalf("ordering broken at index %d: %v not before %v", i, ordered[i-1], ordered[i])
		}
	}
}

func TestParsePrecision(t *testing.T) {
	cases := []struct {
		in   string
		want Precision
		ok   bool
	}{
		{"heuristic/text-backed", PrecisionHeuristicTextBacked, true},
		{"ast-backed", PrecisionASTBacked, true},
		{"project-structure-aware", PrecisionProjectStructure, true},
		{"type-aware", PrecisionTypeAware, true},
		{"policy", PrecisionPolicy, true},
		{"", PrecisionUnset, false},
		{"bogus", PrecisionUnset, false},
		{"heuristic", PrecisionUnset, false},
		{"ast", PrecisionUnset, false},
	}
	for _, c := range cases {
		got, ok := ParsePrecision(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParsePrecision(%q) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
