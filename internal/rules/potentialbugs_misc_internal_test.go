package rules

import "testing"

func TestExtractDeprecationLevel(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple warning",
			in:   `@Deprecated("old", level = DeprecationLevel.WARNING)`,
			want: "WARNING",
		},
		{
			name: "simple error",
			in:   `@Deprecated("old", level = DeprecationLevel.ERROR)`,
			want: "ERROR",
		},
		{
			name: "simple hidden",
			in:   `@Deprecated("old", level = DeprecationLevel.HIDDEN)`,
			want: "HIDDEN",
		},
		{
			name: "unqualified value",
			in:   `@Deprecated("old", level = ERROR)`,
			want: "ERROR",
		},
		{
			name: "level before message containing another level word",
			in:   `@Deprecated("see ERROR cases", level = DeprecationLevel.WARNING)`,
			want: "WARNING",
		},
		{
			name: "level followed by replaceWith referencing another level keyword",
			in:   `@Deprecated("old", level = DeprecationLevel.HIDDEN, replaceWith = ReplaceWith("newApi() // WARNING-style cleanup"))`,
			want: "HIDDEN",
		},
		{
			name: "level followed by replaceWith with ERROR substring",
			in:   `@Deprecated("old", level = DeprecationLevel.WARNING, replaceWith = ReplaceWith("handleERRORLater()"))`,
			want: "WARNING",
		},
		{
			name: "no level argument",
			in:   `@Deprecated("old")`,
			want: "",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := extractDeprecationLevel(tc.in)
			if got != tc.want {
				t.Fatalf("extractDeprecationLevel(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}
