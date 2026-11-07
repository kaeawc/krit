package android

import "strings"

// ToolsIgnoreTokens splits a tools:ignore attribute value into its tokens.
//
// Android Lint splits on commas and trims each token (a value without a comma
// is used as is). Krit also splits on whitespace, so "A B" lists two ids; this
// accepts a superset of Lint's spellings.
func ToolsIgnoreTokens(raw string) []string {
	return strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ' ' || r == '\t' || r == '\n' || r == '\r'
	})
}
