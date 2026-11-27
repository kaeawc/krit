package pipeline

import (
	"os"
	"path/filepath"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// lexicalReuseRules are the cross-file rules crossRuleLexicallyIrrelevant
// can judge. Bit i of a file's lexical tag is set when the file's content is
// relevant to lexicalReuseRules[i]. Append only: the bits are stored with the
// cross findings.
var lexicalReuseRules = []string{
	"OnClick",
	"RoomConflictStrategyReplaceOnFk",
	"RoomRelationWithoutIndex",
	"AnvilMergeComponentEmptyScope",
	"VisibleForTestingCallerInNonTest",
	"TestFixtureAccessedFromProduction",
	"DatabaseQueryOnMainThread",
	"RoomLoadsAllWhereFirstUsed",
}

// lexicalReuseBit returns ruleID's bit in a lexical tag, or 0 when
// crossRuleLexicallyIrrelevant cannot judge the rule.
func lexicalReuseBit(ruleID string) uint16 {
	for i, id := range lexicalReuseRules {
		if id == ruleID {
			return 1 << i
		}
	}
	return 0
}

// lexicalReuseApplies reports whether every active cross-file and
// parsed-file rule is one crossRuleLexicallyIrrelevant can judge, the only
// case canReuseCrossFindingsForLexicallyIrrelevantMisses can reuse.
func lexicalReuseApplies(activeRules []*api.Rule) bool {
	found := false
	for _, r := range activeRules {
		if r == nil || (!r.Needs.Has(api.NeedsCrossFile) && !r.Needs.Has(api.NeedsParsedFiles)) {
			continue
		}
		if lexicalReuseBit(r.ID) == 0 {
			return false
		}
		found = true
	}
	return found
}

// lexicalTag returns the bits of the lexicalReuseRules path's text is
// relevant to.
func lexicalTag(path, text string) uint16 {
	var tag uint16
	for i, id := range lexicalReuseRules {
		if !crossRuleLexicallyIrrelevant(id, path, text) {
			tag |= 1 << i
		}
	}
	return tag
}

// lexicalTags tags every file the cross-file rules read, for saving with
// their findings, or returns nil when the active rules rule out the
// lexical reuse.
func lexicalTags(activeRules []*api.Rule, files ...[]*scanner.File) map[string]uint16 {
	if !lexicalReuseApplies(activeRules) {
		return nil
	}
	n := 0
	for _, set := range files {
		n += len(set)
	}
	tags := make(map[string]uint16, n)
	for _, set := range files {
		for _, f := range set {
			if f != nil {
				tags[f.Path] = lexicalTag(f.Path, string(f.Content))
			}
		}
	}
	return tags
}

// canReuseCrossFindingsForLexicallyIrrelevantMisses reports whether the
// cross findings saved with savedTags still hold although missPaths changed:
// for every active cross-file rule, each miss was irrelevant to it both in
// the content the findings were computed from (savedTags) and now. Checking
// only the current content let an edit that removed the relevant text, such
// as an @VisibleForTesting annotation, reuse the old findings (#1844).
func canReuseCrossFindingsForLexicallyIrrelevantMisses(activeRules []*api.Rule, missPaths []string, savedTags map[string]uint16) bool {
	if len(missPaths) == 0 || len(missPaths) > 4 || savedTags == nil || !lexicalReuseApplies(activeRules) {
		return false
	}
	var activeBits uint16
	for _, r := range activeRules {
		if r != nil && (r.Needs.Has(api.NeedsCrossFile) || r.Needs.Has(api.NeedsParsedFiles)) {
			activeBits |= lexicalReuseBit(r.ID)
		}
	}
	for _, path := range missPaths {
		saved, ok := savedTags[path]
		if !ok || saved&activeBits != 0 {
			return false
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return false
		}
		if lexicalTag(path, string(data))&activeBits != 0 {
			return false
		}
	}
	return true
}

func crossRuleLexicallyIrrelevant(ruleID, path, text string) bool {
	lowerPath := strings.ToLower(filepath.ToSlash(path))
	lowerText := strings.ToLower(text)
	switch ruleID {
	case "OnClick":
		// The rule reads the handlers of classes extending an *Activity.
		return !containsAny(lowerText, "activity", "onclick", "setonclicklistener")
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
