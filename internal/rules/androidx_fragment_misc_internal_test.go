package rules

import "testing"

// removeSingleParentheses must match Lint's two regex passes: drop each `(` not followed by `)`, then each `)` not
// preceded by `(`.
func TestRemoveSingleParentheses(t *testing.T) {
	for in, want := range map[string]string{
		"(context)!!":                  "context!!",
		"getContext()!!":               "getContext()!!",
		"((getActivity()))":            "getActivity()",
		"other.context!!":              "other.context!!",
		"requireNonNull(getContext())": "requireNonNullgetContext()",
	} {
		if got := removeSingleParentheses(in); got != want {
			t.Errorf("removeSingleParentheses(%q) = %q, want %q", in, got, want)
		}
	}
}
