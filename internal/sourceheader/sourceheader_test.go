package sourceheader

import "testing"

func TestFirstSourceLine(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{"empty", "", ""},
		{"single line", "package com.x", "package com.x"},
		{"trailing newline", "package com.x\n", "package com.x"},
		{"leading blank", "\n\npackage com.x", "package com.x"},
		{"leading line comment", "// header\npackage com.x", "package com.x"},
		{"leading block comment opener", "/* doc */\npackage com.x", "package com.x"},
		{"only comments", "// nothing\n// here\n", ""},
		{"trailing comment on subsequent line", "package com.x\n// trailer", "package com.x"},
		{"interior whitespace preserved", "  package   com.x  ", "package   com.x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FirstSourceLine(tc.raw); got != tc.want {
				t.Errorf("FirstSourceLine(%q) = %q, want %q", tc.raw, got, tc.want)
			}
		})
	}
}

func TestFirstHeaderLine(t *testing.T) {
	cases := []struct {
		name    string
		raw     string
		keyword string
		want    string
	}{
		{"kotlin package", "package com.x.y", "package", "com.x.y"},
		{"java package with semicolon", "package com.x.y;", "package", "com.x.y"},
		{"kotlin import alias", "import com.x.Y as Z", "import", "com.x.Y as Z"},
		{"kotlin import wildcard", "import com.x.*", "import", "com.x.*"},
		{"trailing block comment trivia", "package com.x\n/* trailer */", "package", "com.x"},
		{"missing keyword leaves text intact", "com.x.y", "package", "com.x.y"},
		{"empty input", "", "package", ""},
		{"only comments yields empty", "// gone\n", "package", ""},
		{"leading whitespace before keyword", "  package com.x", "package", "com.x"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := FirstHeaderLine(tc.raw, tc.keyword); got != tc.want {
				t.Errorf("FirstHeaderLine(%q, %q) = %q, want %q", tc.raw, tc.keyword, got, tc.want)
			}
		})
	}
}
