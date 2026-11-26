package firchecks

import "strings"

// firPathHeuristicTokens are the substrings krit-fir checkers' path
// heuristics look for in the scan spelling of a file (FirRule.scanPath):
// "/samples/", "-test-utils/", "playground", ".kts" and the like. Rules see
// absolute paths (cli/scan absoluteScanPaths), so a checkout root containing
// one can change verdicts, and the root then stays in the FIR cache key; any
// other root is left out so a restored cache hits at another checkout path.
// The list is deliberately coarse ("test" covers "/test/", "-testing/",
// "/tests/fixtures/", ...): a match only costs relocation.
// TestFirPathHeuristicTokensCoverCheckers keeps it in sync with krit-fir.
var firPathHeuristicTokens = []string{
	"test", "fixture", "sample", "demo", "playground", "sandbox", "benchmark",
	"tooling", "script", "bazel", "debug", "fake", "mock", "stub", "dummy",
	"smoke", "idling", "instrument", "espresso", "maestro", ".idea", ".kts",
}

// rootAffectsVerdicts reports whether a checker path heuristic could match
// inside root: one of firPathHeuristicTokens, or any string a rule is
// configured with (configured markers such as nonProductionPathMarkers are
// matched as substrings), appears in its lowercased path.
func rootAffectsVerdicts(root string, ruleConfigs RuleConfigs) bool {
	lower := strings.ToLower(strings.ReplaceAll(root, "\\", "/")) + "/"
	for _, token := range firPathHeuristicTokens {
		if strings.Contains(lower, token) {
			return true
		}
	}
	found := false
	for _, opts := range ruleConfigs {
		walkConfigStrings(opts, func(s string) {
			if s != "" && strings.Contains(lower, strings.ToLower(s)) {
				found = true
			}
		})
	}
	return found
}

func walkConfigStrings(v any, visit func(string)) {
	switch t := v.(type) {
	case string:
		visit(t)
	case []string:
		for _, s := range t {
			visit(s)
		}
	case []any:
		for _, e := range t {
			walkConfigStrings(e, visit)
		}
	case map[string]any:
		for _, e := range t {
			walkConfigStrings(e, visit)
		}
	case map[any]any:
		for _, e := range t {
			walkConfigStrings(e, visit)
		}
	}
}
