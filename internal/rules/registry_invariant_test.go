package rules

import (
	"sort"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

var knownRegistryAliases = map[string]string{
	"GradleCompatible":     "GradlePluginCompatibility",
	"GradleDependency":     "NewerVersionAvailable",
	"GradleDynamicVersion": "DynamicVersion",
	"StringShouldBeInt":    "StringInteger",
}

func TestV2RegistryIdentityAndMetadataInvariants(t *testing.T) {
	if len(api.Registry) == 0 {
		t.Fatal("rule registry is empty")
	}

	seen := make(map[string]int, len(api.Registry))
	var missingMeta []string
	var metaMismatch []string
	var missingIdentity []string
	var missingCheck []string
	var missingImplementation []string
	var fixFromV2 int

	for _, r := range api.Registry {
		if r == nil {
			t.Fatal("rule registry contains nil rule")
		}

		seen[r.ID]++
		if r.ID == "" || r.Category == "" || r.Description == "" || r.Sev == "" {
			missingIdentity = append(missingIdentity, r.ID)
		}
		if r.Check == nil && !r.Needs.Has(api.NeedsAggregate) {
			missingCheck = append(missingCheck, r.ID)
		}
		if !HasV2Implementation(r) {
			missingImplementation = append(missingImplementation, r.ID)
		}

		meta, ok := MetaForRule(r)
		if !ok {
			if _, isAlias := knownRegistryAliases[r.ID]; !isAlias {
				missingMeta = append(missingMeta, r.ID)
			}
		} else if meta.ID != r.ID {
			metaMismatch = append(metaMismatch, r.ID+" -> "+meta.ID)
		}

		if _, ok := GetV2FixLevel(r); ok {
			if r.Fix != api.FixNone {
				fixFromV2++
			}
		}
	}

	var duplicateIDs []string
	for id, count := range seen {
		if count > 1 {
			duplicateIDs = append(duplicateIDs, id)
		}
	}

	sort.Strings(duplicateIDs)
	sort.Strings(missingMeta)
	sort.Strings(metaMismatch)
	sort.Strings(missingIdentity)
	sort.Strings(missingCheck)
	sort.Strings(missingImplementation)

	if len(duplicateIDs) > 0 {
		t.Fatalf("unexpected duplicate rule IDs: %s", strings.Join(duplicateIDs, ", "))
	}
	if len(missingIdentity) > 0 {
		t.Fatalf("rules missing required identity fields: %s", strings.Join(missingIdentity, ", "))
	}
	if len(missingCheck) > 0 {
		t.Fatalf("rules missing Check function: %s", strings.Join(missingCheck, ", "))
	}
	if len(missingImplementation) > 0 {
		t.Fatalf("rules missing runnable v2 implementations: %s", strings.Join(missingImplementation, ", "))
	}
	if len(missingMeta) > 0 {
		t.Fatalf("rules missing metadata descriptors: %s", strings.Join(missingMeta, ", "))
	}

	if len(metaMismatch) != 0 {
		t.Fatalf("unexpected metadata ID mismatches: %s", strings.Join(metaMismatch, ", "))
	}
	t.Logf("rule registry: %d rules, %d fix levels on api.Rule",
		len(api.Registry), fixFromV2)
}

func TestV2RegistryDefaultActiveMatchesMetadata(t *testing.T) {
	var mismatches []string
	for _, r := range api.Registry {
		if _, isAlias := knownRegistryAliases[r.ID]; isAlias {
			continue
		}
		meta, ok := MetaForRule(r)
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

func TestV2RegistryHasNoDuplicateRuleIDs(t *testing.T) {
	if !registryHasNoDuplicateIDs() {
		t.Fatal("rule registry contains duplicate rule IDs")
	}
}

func registryHasNoDuplicateIDs() bool {
	seen := make(map[string]bool, len(api.Registry))
	for _, r := range api.Registry {
		if seen[r.ID] {
			return false
		}
		seen[r.ID] = true
	}
	return true
}

func TestV2RegistryAliasSetIsExplicit(t *testing.T) {
	got := map[string]string{}
	for _, r := range api.Registry {
		mp, ok := r.Implementation.(api.MetaProvider)
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
