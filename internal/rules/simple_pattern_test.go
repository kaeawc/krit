package rules

import "testing"

func TestSimplePatternMatches(t *testing.T) {
	cases := []struct {
		pattern, name string
		want          bool
	}{
		{"equals", "equals", true},
		{"equals", "equalsTo", false},
		{"to*", "toString", true},
		{"*Name", "firstName", true},
		{"get?", "getX", true},
		{"get?", "getXY", false},
		{"a.b", "aXb", false},
	}
	for _, c := range cases {
		if got := simplePatternMatches(c.pattern, c.name); got != c.want {
			t.Errorf("simplePatternMatches(%q, %q) = %v, want %v", c.pattern, c.name, got, c.want)
		}
	}
}
