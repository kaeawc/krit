package rules

import (
	"regexp"
	"strings"
)

// simplePatternMatches reports whether name matches a detekt simple pattern, where `*` matches any
// run of characters and `?` matches one character. The whole name must match.
func simplePatternMatches(pattern, name string) bool {
	if !strings.ContainsAny(pattern, "*?") {
		return pattern == name
	}
	var b strings.Builder
	b.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			b.WriteString(".*")
		case '?':
			b.WriteString(".")
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return pattern == name
	}
	return re.MatchString(name)
}
