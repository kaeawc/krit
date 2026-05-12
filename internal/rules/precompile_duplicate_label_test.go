package rules

import "testing"

func TestNormalizeIntegerLiteral(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"1", "1"},
		{"1_000", "1000"},
		{"0x01", "1"},
		{"0X0a", "10"},
		{"0b0001", "1"},
		{"0B1010", "10"},
		{"1000", "1000"},
		{"1L", "1L"},
		{"0x01L", "1L"},
		{"1u", "1u"},
		{"1uL", "1uL"},
	}
	for _, c := range cases {
		if got := normalizeIntegerLiteral(c.in); got != c.want {
			t.Errorf("normalizeIntegerLiteral(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeIntegerLiteralEquivalenceClasses(t *testing.T) {
	groups := [][]string{
		{"1", "0x01", "0X1", "0b1", "001"},
		{"1000", "1_000", "0x3E8", "0b1111101000"},
		{"1L", "0x01L", "1_L"},
	}
	for _, g := range groups {
		first := normalizeIntegerLiteral(g[0])
		for _, s := range g[1:] {
			if got := normalizeIntegerLiteral(s); got != first {
				t.Errorf("expected %q and %q to normalize identically; got %q vs %q", g[0], s, first, got)
			}
		}
	}
}
