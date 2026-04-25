package rules

import (
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules/registry"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

var knownRegistryAliases = map[string]string{
	"GradleCompatible":     "GradlePluginCompatibility",
	"GradleDependency":     "NewerVersionAvailable",
	"GradleDynamicVersion": "DynamicVersion",
	"StringShouldBeInt":    "StringInteger",
}

var knownDuplicateRuleIDs = map[string]int{
	// AppCompatResource has two Android lint-compatible registrations that
	// share the same ID but cover different resource contexts.
	"AppCompatResource": 2,
}

func TestV2RegistryIdentityAndMetadataInvariants(t *testing.T) {
	if len(v2.Registry) == 0 {
		t.Fatal("v2 registry is empty")
	}

	seen := make(map[string]int, len(v2.Registry))
	var missingMeta []string
	var metaMismatch []string
	var missingIdentity []string
	var missingCheck []string
	var fixFallbackRules []string
	var fixFromV2, fixFromFallback int

	for _, r := range v2.Registry {
		if r == nil {
			t.Fatal("v2 registry contains nil rule")
		}

		seen[r.ID]++
		if r.ID == "" || r.Category == "" || r.Description == "" || r.Sev == "" {
			missingIdentity = append(missingIdentity, r.ID)
		}
		if r.Check == nil && !r.Needs.Has(v2.NeedsAggregate) {
			missingCheck = append(missingCheck, r.ID)
		}

		meta, ok := MetaForV2Rule(r)
		if !ok {
			if _, isAlias := knownRegistryAliases[r.ID]; !isAlias {
				missingMeta = append(missingMeta, r.ID)
			}
		} else if meta.ID != r.ID {
			metaMismatch = append(metaMismatch, r.ID+" -> "+meta.ID)
		}

		if _, ok := GetV2FixLevel(r); ok {
			if r.Fix != v2.FixNone {
				fixFromV2++
			} else {
				fixFromFallback++
				fixFallbackRules = append(fixFallbackRules, r.ID)
			}
		}
	}

	var duplicateIDs []string
	for id, count := range seen {
		if want, ok := knownDuplicateRuleIDs[id]; ok {
			if count != want {
				duplicateIDs = append(duplicateIDs, id)
			}
			continue
		}
		if count > 1 {
			duplicateIDs = append(duplicateIDs, id)
		}
	}

	sort.Strings(duplicateIDs)
	sort.Strings(missingMeta)
	sort.Strings(metaMismatch)
	sort.Strings(missingIdentity)
	sort.Strings(missingCheck)
	sort.Strings(fixFallbackRules)

	if len(duplicateIDs) > 0 {
		t.Fatalf("unexpected duplicate v2 rule IDs: %s", strings.Join(duplicateIDs, ", "))
	}
	if len(missingIdentity) > 0 {
		t.Fatalf("rules missing required identity fields: %s", strings.Join(missingIdentity, ", "))
	}
	if len(missingCheck) > 0 {
		t.Fatalf("rules missing Check function: %s", strings.Join(missingCheck, ", "))
	}
	if len(missingMeta) > 0 {
		t.Fatalf("rules missing metadata descriptors: %s", strings.Join(missingMeta, ", "))
	}

	if len(metaMismatch) != 0 {
		t.Fatalf("unexpected metadata ID mismatches: %s", strings.Join(metaMismatch, ", "))
	}
	if len(fixFallbackRules) != 0 {
		t.Fatalf("rules still relying on OriginalV1 fix-level fallback: %s", strings.Join(fixFallbackRules, ", "))
	}

	t.Logf("v2 registry: %d rules, %d fix levels on v2.Rule, %d fix levels via transitional fallback",
		len(v2.Registry), fixFromV2, fixFromFallback)
}

func TestV2RegistryDefaultActiveMatchesMetadata(t *testing.T) {
	var mismatches []string
	for _, r := range v2.Registry {
		if _, isAlias := knownRegistryAliases[r.ID]; isAlias {
			continue
		}
		meta, ok := MetaForV2Rule(r)
		if !ok {
			mismatches = append(mismatches, r.ID+" missing metadata")
			continue
		}
		if meta.ID != r.ID {
			continue // Alias defaults are checked below.
		}
		if IsDefaultActive(r.ID) != meta.DefaultActive {
			mismatches = append(mismatches, r.ID)
		}
	}

	for _, alias := range aliasDefaultInactive() {
		if IsDefaultActive(alias) {
			mismatches = append(mismatches, alias+" alias default-active=true")
		}
	}

	sort.Strings(mismatches)
	if len(mismatches) > 0 {
		t.Fatalf("default-active metadata mismatch: %s", strings.Join(mismatches, ", "))
	}
}

func TestV2RegistryDispatcherClassificationHasNoLegacyRules(t *testing.T) {
	dispatcher := NewDispatcherV2(v2.Registry)
	_, _, _, _, _, legacyCount := dispatcher.Stats()
	if legacyCount != 0 {
		t.Fatalf("v2 registry has %d rules classified into legacy fallback", legacyCount)
	}
}

func TestV2RegistryAliasSetIsExplicit(t *testing.T) {
	got := map[string]string{}
	for _, r := range v2.Registry {
		mp, ok := r.OriginalV1.(registry.MetaProvider)
		if !ok {
			continue
		}
		meta := mp.Meta()
		if meta.ID != r.ID {
			got[r.ID] = meta.ID
		}
	}

	if len(got) != len(knownRegistryAliases) {
		t.Fatalf("alias count mismatch: want %d got %d (%v)", len(knownRegistryAliases), len(got), got)
	}
	for alias, primary := range knownRegistryAliases {
		if got[alias] != primary {
			t.Fatalf("alias %s maps to %q, want %q", alias, got[alias], primary)
		}
	}
}
