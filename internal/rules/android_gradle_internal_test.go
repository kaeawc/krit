package rules

import "testing"

func TestStripGradleLineComment(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "no comment",
			in:   `val s = "hello"`,
			want: `val s = "hello"`,
		},
		{
			name: "trailing line comment",
			in:   `minSdk = 24 // trailing`,
			want: `minSdk = 24 `,
		},
		{
			name: "double slash inside double-quoted string preserved",
			in:   `val url = "http://example.com"`,
			want: `val url = "http://example.com"`,
		},
		{
			name: "double slash inside single-quoted string preserved",
			in:   `val url = 'http://example.com'`,
			want: `val url = 'http://example.com'`,
		},
		{
			name: "triple-quoted string with double-slash preserved",
			in:   `val s = """foo://bar"""`,
			want: `val s = """foo://bar"""`,
		},
		{
			name: "triple-quoted with embedded double-quote then double-slash preserved",
			in:   `val s = """abc"//def"""`,
			want: `val s = """abc"//def"""`,
		},
		{
			name: "triple-quoted string followed by real comment",
			in:   `val s = """foo://bar""" // real`,
			want: `val s = """foo://bar""" `,
		},
		{
			name: "triple-single-quoted string with double-slash preserved",
			in:   `val s = '''foo://bar'''`,
			want: `val s = '''foo://bar'''`,
		},
		{
			name: "escaped quote in regular string",
			in:   `val s = "a\"//b"`,
			want: `val s = "a\"//b"`,
		},
		{
			name: "comment after closed string",
			in:   `val s = "abc" // tail`,
			want: `val s = "abc" `,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := stripGradleLineComment(tc.in)
			if got != tc.want {
				t.Fatalf("stripGradleLineComment(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
