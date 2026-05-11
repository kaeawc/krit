package pipeline

import (
	"os"
	"path/filepath"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func canReuseCrossFindingsForLexicallyIrrelevantMisses(activeRules []*api.Rule, missPaths []string) bool {
	if len(missPaths) == 0 || len(missPaths) > 4 {
		return false
	}
	for _, path := range missPaths {
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		text := string(data)
		for _, r := range activeRules {
			if r == nil || (!r.Needs.Has(api.NeedsCrossFile) && !r.Needs.Has(api.NeedsParsedFiles)) {
				continue
			}
			if !crossRuleLexicallyIrrelevant(r.ID, path, text) {
				return false
			}
		}
	}
	return true
}

func crossRuleLexicallyIrrelevant(ruleID, path, text string) bool {
	lowerPath := strings.ToLower(filepath.ToSlash(path))
	lowerText := strings.ToLower(text)
	switch ruleID {
	case "OnClick":
		return !containsAny(lowerText, "onclick", "setonclicklistener")
	case "RoomConflictStrategyReplaceOnFk":
		return !containsAny(lowerText, "onconflictstrategy", "foreignkey", "@entity", "room")
	case "RoomRelationWithoutIndex":
		return !containsAny(lowerText, "@relation", "foreignkey", "@entity", "room")
	case "AnvilMergeComponentEmptyScope":
		return !containsAny(lowerText, "mergecomponent", "contributesto", "anvil", "scope::class")
	case "VisibleForTestingCallerInNonTest":
		return !containsAny(lowerText, "visiblefortesting", "fortesting")
	case "TestFixtureAccessedFromProduction":
		if containsAny(lowerPath, "testfixtures", "/test/", "/androidtest/") {
			return false
		}
		return !containsAny(lowerText, "testfixtures", "testfixture")
	case "DatabaseQueryOnMainThread":
		return !containsAny(lowerText,
			"sqlite", "rawquery", ".query(", "@query", "@dao", "roomdatabase",
			"dispatchers.main", "mainthread", "lifecycle", "onclick", "setonclicklistener",
		)
	case "RoomLoadsAllWhereFirstUsed":
		return !containsAny(lowerText, "@query", "@dao", "room", ".first(", ".firstornull(", ".single(", ".singleornull(", "getall")
	default:
		return false
	}
}

func containsAny(s string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(s, needle) {
			return true
		}
	}
	return false
}
