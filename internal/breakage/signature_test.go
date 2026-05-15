package breakage

import "testing"

func TestNormalize(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"  Failed: timeout after 12345ms\t\n", "failed: timeout after <n>ms"},
		{"panic: runtime error at 0xdeadbeef", "panic: runtime error at <addr>"},
		{"\x1b[31mError\x1b[0m: bad", "error: bad"},
		{"", ""},
		{"42", "42"}, // <3 digits: kept
		{"line 100 col 99", "line <n> col 99"},
	}
	for _, tc := range tests {
		got := Normalize(tc.in)
		if got != tc.want {
			t.Errorf("Normalize(%q): got %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestHashIDStable(t *testing.T) {
	a := HashID("test-failure", "x", "abc123", "ci")
	b := HashID("test-failure", "x", "abc123", "ci")
	if a != b {
		t.Fatalf("HashID not stable: %q vs %q", a, b)
	}
	c := HashID("test-failure", "x", "abc123", "local")
	if a == c {
		t.Fatalf("HashID should differ by source: both %q", a)
	}
	if len(a) != 16 {
		t.Fatalf("HashID should be 16 chars, got %d", len(a))
	}
}
