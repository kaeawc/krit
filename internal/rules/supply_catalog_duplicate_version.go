package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// VersionCatalogDuplicateVersionRule flags entries in the [versions] table of
// gradle/libs.versions.toml whose literal version strings duplicate one
// another. Two aliases pinned independently to the same value should usually
// be collapsed into a single named version that the libraries reference via
// `version.ref` so upgrades stay in lockstep.
type VersionCatalogDuplicateVersionRule struct {
	BaseRule
}

// Confidence is high — duplicate literal version strings are a
// straightforward textual fact derived from the parsed catalog.
func (r *VersionCatalogDuplicateVersionRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *VersionCatalogDuplicateVersionRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

func (r *VersionCatalogDuplicateVersionRule) check(ctx *api.Context) {
	pmi := ctx.ModuleIndex
	if pmi == nil || pmi.Graph == nil {
		return
	}
	catalogPath := module.FindVersionCatalog(pmi.Graph.RootDir)
	if catalogPath == "" {
		return
	}
	cat, err := module.ParseVersionCatalog(catalogPath)
	if err != nil {
		return
	}

	// Group [versions] aliases by their literal value. Skip rich-version
	// inline tables (starting with '{') and empty values — equivalence on
	// those would require resolving prefer/require/strictly semantics.
	groups := make(map[string][]module.CatalogEntry)
	for _, entry := range cat.Versions {
		if entry.Value == "" || strings.HasPrefix(entry.Value, "{") {
			continue
		}
		groups[entry.Value] = append(groups[entry.Value], entry)
	}

	for value, entries := range groups {
		if len(entries) < 2 {
			continue
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Line < entries[j].Line })
		aliases := make([]string, len(entries))
		for i, e := range entries {
			aliases[i] = e.Alias
		}
		// Emit on the secondary entries so the first declaration is
		// treated as the canonical alias to keep.
		for _, e := range entries[1:] {
			ctx.Emit(scanner.Finding{
				File:       catalogPath,
				Line:       e.Line,
				Col:        1,
				RuleSet:    r.RuleSetName,
				Rule:       r.RuleName,
				Severity:   r.Sev,
				Message:    fmt.Sprintf("Version alias '%s' duplicates literal version %q already declared by '%s' (aliases: %s); collapse to a single alias and use version.ref.", e.Alias, value, entries[0].Alias, strings.Join(aliases, ", ")),
				Confidence: r.Confidence(),
			})
		}
	}
}
