package rules

import "testing"

func TestIsTypeParameter(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{name: "T", want: true},
		{name: "R", want: true},
		{name: "String", want: false},
		{name: "t", want: false},
		{name: "", want: false},
	}
	for _, tc := range cases {
		if got := isTypeParameter(tc.name); got != tc.want {
			t.Fatalf("isTypeParameter(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
