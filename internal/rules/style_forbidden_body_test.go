package rules

import "testing"

func TestForbiddenCommentBody(t *testing.T) {
	cases := []struct {
		name string
		text string
		want string
	}{
		{name: "line", text: "// TODO: fix", want: "TODO: fix"},
		{name: "block", text: "/* TODO: fix */", want: "TODO: fix"},
		{name: "kdoc", text: "/**\n * TODO: fix\n */", want: "TODO: fix"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := forbiddenCommentBody(tc.text); got != tc.want {
				t.Fatalf("forbiddenCommentBody(%q) = %q, want %q", tc.text, got, tc.want)
			}
		})
	}
}
