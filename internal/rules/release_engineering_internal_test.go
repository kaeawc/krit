package rules

import "testing"

func TestDebugToastPrefixRegex(t *testing.T) {
	cases := []struct {
		text string
		want bool
	}{
		{`"debug"`, true},
		{`"DEBUG: clicked"`, true},
		{`'test '`, true},
		{`"wip"`, true},
		{`"debug message"`, true},
		{`"debugger"`, false},
		{`"testing-fixture"`, false},
		{`"wipe"`, false},
		{`"Debugger attached"`, false},
		{`"savedMessage"`, false},
	}
	for _, tc := range cases {
		got := debugToastPrefixRe.MatchString(tc.text)
		if got != tc.want {
			t.Errorf("debugToastPrefixRe.MatchString(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
}
