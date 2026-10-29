package rules

import "strings"

// simpleQualifiedName returns the last dot-separated segment of s.
func simpleQualifiedName(s string) string {
	s = strings.TrimSpace(s)
	if idx := strings.LastIndexByte(s, '.'); idx >= 0 && idx < len(s)-1 {
		return s[idx+1:]
	}
	return s
}

func isIdentStartByte(b byte) bool {
	return b == '_' || (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z')
}

func isIdentByte(b byte) bool {
	return isIdentStartByte(b) || (b >= '0' && b <= '9')
}
