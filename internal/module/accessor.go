package module

import (
	"strings"
	"unicode"
)

// AccessorToPath converts a typesafe project accessor name to a Gradle module path.
// For example: "circuitRuntime" -> ":circuit-runtime",
// "sentryAndroidCore" -> ":sentry-android-core".
func AccessorToPath(accessor string) string {
	if accessor == "" {
		return ":"
	}
	var parts []string
	start := 0
	for i, r := range accessor {
		if i > start && unicode.IsUpper(r) {
			parts = append(parts, strings.ToLower(accessor[start:i]))
			start = i
		}
	}
	parts = append(parts, strings.ToLower(accessor[start:]))
	return ":" + strings.Join(parts, "-")
}
