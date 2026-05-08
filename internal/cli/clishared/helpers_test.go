package clishared

import "testing"

func TestSplitPositional(t *testing.T) {
	cases := []struct {
		name           string
		args           []string
		max            int
		wantPositional []string
		wantRest       []string
	}{
		{"empty", nil, 1, nil, []string{}},
		{"one positional", []string{"foo"}, 1, []string{"foo"}, []string{}},
		{"positional then flag", []string{"foo", "--bar"}, 1, []string{"foo"}, []string{"--bar"}},
		{"flag interleaved with extras", []string{"foo", "--flag", "bar"}, 1, []string{"foo"}, []string{"--flag", "bar"}},
		{"max two", []string{"a", "b", "c"}, 2, []string{"a", "b"}, []string{"c"}},
		{"flag first", []string{"--bar", "foo"}, 1, []string{"foo"}, []string{"--bar"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotPos, gotRest := SplitPositional(tc.args, tc.max)
			if !equalSlice(gotPos, tc.wantPositional) {
				t.Errorf("positional = %v, want %v", gotPos, tc.wantPositional)
			}
			if !equalSlice(gotRest, tc.wantRest) {
				t.Errorf("rest = %v, want %v", gotRest, tc.wantRest)
			}
		})
	}
}

func TestSimpleName(t *testing.T) {
	cases := map[string]string{
		"com.acme.Foo.bar":      "bar",
		"com.acme.Foo":          "Foo",
		"bar":                   "bar",
		"com.acme.Foo.bar(Int)": "bar",
		"":                      "",
	}
	for in, want := range cases {
		if got := SimpleName(in); got != want {
			t.Errorf("SimpleName(%q) = %q, want %q", in, got, want)
		}
	}
}

func equalSlice(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
