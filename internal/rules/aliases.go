package rules

import (
	"sort"
	"strings"
	"sync"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// AllSuppressionAliases returns a snapshot of every rule's suppression
// aliases, keyed by canonical rule ID. The pipeline Parse phase passes
// this into scanner.SuppressionFilter.WithRuleAliases so an
// @Suppress("LegacyName") (or `// krit:ignore[LegacyName]`) silences
// findings emitted under the canonical ID.
//
// Returns a nil map when no registered rule declares any aliases. The
// scanner side treats nil and empty equivalently, so a nil result is a
// safe drop-in.
func AllSuppressionAliases() map[string][]string {
	var out map[string][]string
	detektNames := detektNamesByRule()
	for _, r := range api.Registry {
		if r == nil {
			continue
		}
		aliases := append(append([]string(nil), r.Aliases...), LintSuppressionAliases(r)...)
		for _, name := range detektNames[r.ID] {
			if !containsString(aliases, name) {
				aliases = append(aliases, name)
			}
		}
		if len(aliases) == 0 {
			continue
		}
		if out == nil {
			out = make(map[string][]string)
		}
		// Defensive copy: rules treat Aliases as immutable, but the
		// scanner caches the slice by reference and we don't want a
		// later registry mutation to bleed in.
		out[r.ID] = aliases
	}
	return out
}

// detektNamesByRule inverts the detekt rename table so @Suppress("DetektName") silences the krit rule.
func detektNamesByRule() map[string][]string {
	out := make(map[string][]string)
	for detekt, targets := range config.DetektRuleRenames() {
		for _, target := range targets {
			out[target] = append(out[target], detekt)
		}
	}
	for _, names := range out {
		sort.Strings(names)
	}
	return out
}

func containsString(list []string, s string) bool {
	for _, have := range list {
		if have == s {
			return true
		}
	}
	return false
}

// extraLintAliases lists Android lint issue IDs that name a rule whose ID cannot be derived.
var extraLintAliases = map[string][]string{
	"IgnoredReturnValue":             {"CheckResult"},
	"RecyclerAdapterWithoutDiffUtil": {"NotifyDataSetChanged"},
	"ExportedWithoutPermission":      {"ExportedActivity"},
	"ServiceExportedManifest":        {"ExportedService"},
	// Android lint reports calls to @RequiresApi members under NewApi.
	"RequiresApiViolation": {"NewApi"},
}

// LintSuppressionAliases returns the Android lint issue IDs that also suppress the rule: the ID without
// its Resource/Manifest suffix when that is a known lint ID, the AOSP issue ID, and any extra mapped ID.
func LintSuppressionAliases(r *api.Rule) []string {
	if r == nil {
		return nil
	}
	var out []string
	add := func(id string) {
		if id == "" || id == r.ID {
			return
		}
		for _, have := range out {
			if have == id {
				return
			}
		}
		out = append(out, id)
	}
	for _, suffix := range []string{"Resource", "Manifest"} {
		if trimmed := strings.TrimSuffix(r.ID, suffix); trimmed != r.ID && knownLintIssueIDs[trimmed] {
			add(trimmed)
		}
	}
	if withIssueID, ok := r.Implementation.(interface{ LintIssueID() string }); ok {
		add(withIssueID.LintIssueID())
	}
	// The kotlinc diagnostic name (@Suppress("UNREACHABLE_CODE")) silences the matching rule too.
	add(r.KotlincAnalog)
	for _, id := range extraLintAliases[r.ID] {
		if id != r.ID {
			out = append(out, id)
		}
	}
	return out
}

var (
	lintAliasesOnce sync.Once
	lintAliasesByID map[string][]string
)

// lintAliasesForRuleID returns LintSuppressionAliases for a registered rule ID.
func lintAliasesForRuleID(id string) []string {
	lintAliasesOnce.Do(func() {
		lintAliasesByID = make(map[string][]string, len(api.Registry))
		for _, r := range api.Registry {
			if r == nil {
				continue
			}
			// The Rule.Aliases (retired IDs) silence the rule from tools:ignore the same way Android lint IDs do.
			aliases := append(LintSuppressionAliases(r), r.Aliases...)
			if len(aliases) > 0 {
				lintAliasesByID[r.ID] = aliases
			}
		}
	})
	return lintAliasesByID[id]
}

var (
	registeredLintIDsOnce sync.Once
	registeredLintIDs     map[string]bool
)

// isKnownLintIssueID reports whether id is a built-in lint issue ID or names, by ID or alias,
// a registered Android lint rule.
func isKnownLintIssueID(id string) bool {
	if knownLintIssueIDs[id] {
		return true
	}
	registeredLintIDsOnce.Do(func() {
		registeredLintIDs = make(map[string]bool)
		for _, r := range api.Registry {
			if r == nil || r.Category != androidRuleSet {
				continue
			}
			registeredLintIDs[r.ID] = true
			for _, alias := range lintAliasesForRuleID(r.ID) {
				registeredLintIDs[alias] = true
			}
		}
	})
	return registeredLintIDs[id]
}
