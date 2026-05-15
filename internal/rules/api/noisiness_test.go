package api

import "testing"

func TestNoisinessString(t *testing.T) {
	cases := []struct {
		in   Noisiness
		want string
	}{
		{NoisinessUnset, "unset"},
		{NoisinessQuiet, "quiet"},
		{NoisinessNormal, "normal"},
		{NoisinessNoisy, "noisy"},
	}
	for _, c := range cases {
		if got := c.in.String(); got != c.want {
			t.Errorf("Noisiness(%d).String() = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNoisinessOrdering(t *testing.T) {
	// Values must be ordered cleanest -> noisiest so callers can filter
	// with comparisons (e.g. <= NoisinessNormal excludes NoisinessNoisy).
	ordered := []Noisiness{NoisinessUnset, NoisinessQuiet, NoisinessNormal, NoisinessNoisy}
	for i := 1; i < len(ordered); i++ {
		if ordered[i-1] >= ordered[i] {
			t.Fatalf("ordering broken at index %d: %v not before %v", i, ordered[i-1], ordered[i])
		}
	}
}

func TestParseNoisiness(t *testing.T) {
	cases := []struct {
		in   string
		want Noisiness
		ok   bool
	}{
		{"quiet", NoisinessQuiet, true},
		{"normal", NoisinessNormal, true},
		{"noisy", NoisinessNoisy, true},
		{"", NoisinessUnset, false},
		{"bogus", NoisinessUnset, false},
	}
	for _, c := range cases {
		got, ok := ParseNoisiness(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("ParseNoisiness(%q) = (%v, %v), want (%v, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}
