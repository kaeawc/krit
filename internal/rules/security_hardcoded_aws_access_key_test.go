package rules

import "testing"

func TestLooksLikeHardcodedAwsAccessKey(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"AKIA prefix", "AKIA1234567890ABCDEF", true},
		{"ASIA prefix", "ASIAQRSTUVWXYZ012345", true},
		{"AGPA prefix", "AGPA1111222233334444", true},
		{"AIDA prefix", "AIDA1111222233334444", true},
		{"AROA prefix", "AROA1111222233334444", true},
		{"AIPA prefix", "AIPA1111222233334444", true},
		{"ANPA prefix", "ANPA1111222233334444", true},
		{"ANVA prefix", "ANVA1111222233334444", true},
		{"lowercase fail", "akia1234567890abcdef", false},
		{"too short", "AKIA1234567890ABCD", false},
		{"too long", "AKIA1234567890ABCDEFGH", false},
		{"unknown prefix", "ZZZZ1234567890ABCDEF", false},
		{"non-alphanumeric", "AKIA1234567890ABCDE!", false},
		{"AWS docs example placeholder", "AKIAIOSFODNN7EXAMPLE", false},
		{"placeholder marker", "AKIA_PLACEHOLDER_KEY", false},
		{"interpolated literal token", `${"AKIA1234567890ABCDEF"}`, true},
		{"interpolated dynamic value", "${someVariable}", false},
		{"empty", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := looksLikeHardcodedAwsAccessKey(c.body); got != c.want {
				t.Errorf("looksLikeHardcodedAwsAccessKey(%q) = %v, want %v", c.body, got, c.want)
			}
		})
	}
}
