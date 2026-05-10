package di

import "testing"

// TestPackageNameFlat_StripsTrailingTrivia is a regression test for #114:
// tree-sitter Kotlin sometimes attaches trailing comments and whitespace
// to the package_header node. Before the sourceheader migration,
// packageNameFlat returned the package name with the trivia still
// attached, which broke downstream FQN matching for any file with a
// trailing comment on the package line.
func TestPackageNameFlat_StripsTrailingTrivia(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "trailing block comment",
			content: "package com.example\n/* TODO: split this module */\nclass A",
			want:    "com.example",
		},
		{
			name:    "leading line comment",
			content: "// header banner\npackage com.example\nclass A",
			want:    "com.example",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := parseDIFile(t, tc.content)
			if got := packageNameFlat(file); got != tc.want {
				t.Errorf("packageNameFlat = %q, want %q", got, tc.want)
			}
		})
	}
}
