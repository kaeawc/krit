package scan

import "strings"

// parseRuleNameSet parses a comma-separated list of rule names into a
// lookup set. Whitespace around each name is trimmed. Empty input
// returns an empty (non-nil) map so callers can index it without nil
// checks.
//
// This is the input parser shared by --disable-rules and --enable-rules.
// Behavior matches the original inline loop bit-for-bit, including the
// edge case where a trailing comma or whitespace-only token produces an
// empty-string entry in the map (harmless: no real rule has ID "").
func parseRuleNameSet(csv string) map[string]bool {
	out := make(map[string]bool)
	if csv == "" {
		return out
	}
	for _, name := range strings.Split(csv, ",") {
		out[strings.TrimSpace(name)] = true
	}
	return out
}
