package scanner

import "testing"

func TestParsePackageHeaderText(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "simple",
			raw:  "package com.example",
			want: "com.example",
		},
		{
			name: "with semicolon",
			raw:  "package com.example;",
			want: "com.example",
		},
		{
			name: "with trailing comments",
			raw: "package com.example.utils\n" +
				"\n" +
				"// keep me\n" +
				"// and me",
			want: "com.example.utils",
		},
		{
			name: "leading whitespace and trailing newline",
			raw:  "  package   com.example.foo  \n",
			want: "com.example.foo",
		},
		{
			name: "empty",
			raw:  "",
			want: "",
		},
		{
			name: "comments only",
			raw:  "// just a comment\n/* and another */",
			want: "",
		},
		{
			name: "leading blank lines and comments",
			raw:  "\n\n// a comment\n\npackage com.example\n",
			want: "com.example",
		},
		{
			name: "block comment then package",
			raw:  "/* license header */\npackage com.example.bar\n",
			want: "com.example.bar",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parsePackageHeaderText(tc.raw); got != tc.want {
				t.Errorf("parsePackageHeaderText = %q, want %q", got, tc.want)
			}
		})
	}
}
