package rules

import (
	"maps"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// buildExcludedSetPerRule is the per-rule scan buildExcludedSet replaced:
// check every rule in every bucket against its configured excludes.
func buildExcludedSetPerRule(d *Dispatcher, filePath string) map[string]bool {
	excluded := make(map[string]bool)
	check := func(r *api.Rule) {
		if excludes := GetRuleExcludes(r.ID); len(excludes) > 0 && IsFileExcluded(filePath, excludes) {
			excluded[r.ID] = true
		}
	}
	for _, bucket := range d.flatTypeRules {
		for _, r := range bucket {
			check(r)
		}
	}
	for _, rs := range [][]*api.Rule{d.allNodeRules, d.lineRules, d.crossFileRules, d.moduleAwareRules, d.manifestRules,
		d.resourceRules, d.resourceSourceRules, d.iconRules, d.gradleRules, d.aggregateRules} {
		for _, r := range rs {
			check(r)
		}
	}
	return excluded
}

func TestBuildExcludedSetMatchesPerRuleScan(t *testing.T) {
	d := NewDispatcher(api.Registry, nil)
	_ = writeKotlinFile(t, "fun f() { g() }\n", "Warm.kt")
	_, _ = d.ensureFlatTypeIndex()
	set := map[string][]string{
		"MagicNumber":         {"**/test/**", "**/*Test.kt"},
		"LongMethod":          {"**/generated/**"},
		"MaxLineLength":       {scanner.IncludePatternPrefix + "**/src/main/**"},
		"NotARegisteredID":    {"**"},
		"PrintlnInProduction": nil,
	}
	for id, pats := range set {
		SetRuleExcludes(id, pats)
	}
	t.Cleanup(func() {
		for id := range set {
			SetRuleExcludes(id, nil)
		}
	})
	for _, path := range []string{
		"/p/app/src/main/kotlin/A.kt",
		"/p/app/src/test/kotlin/ATest.kt",
		"/p/app/build/generated/B.kt",
		"/p/lib/src/commonMain/C.kt",
	} {
		got, want := d.buildExcludedSet(path), buildExcludedSetPerRule(d, path)
		if !maps.Equal(got, want) {
			t.Errorf("%s: got %v, want %v", path, got, want)
		}
	}
}

func TestExcludedSlotsMatchExcludedForFile(t *testing.T) {
	d := NewDispatcher(api.Registry, nil)
	for _, f := range []*scanner.File{
		{Path: "/p/A.kt", Language: scanner.LangKotlin},
		{Path: "/p/build.gradle.kts", Language: scanner.LangKotlin},
		{Path: "/p/A.java", Language: scanner.LangJava},
		{Path: "/p/AndroidManifest.xml", Language: scanner.LangXML},
	} {
		byID := d.excludedForFile(f)
		bits := d.excludedSlotsForFile(f)
		if len(bits) != len(d.slotIDs) {
			t.Fatalf("%s: %d bits for %d slots", f.Path, len(bits), len(d.slotIDs))
		}
		for slot, id := range d.slotIDs {
			if bits[slot] != byID[id] {
				t.Fatalf("%s: slot %d (%s) excluded=%v, map says %v", f.Path, slot, id, bits[slot], byID[id])
			}
		}
		// Callers get a private copy they may mark further.
		bits[0] = !bits[0]
		if again := d.excludedSlotsForFile(f); again[0] == bits[0] {
			t.Fatalf("%s: cached bitmap was modified through a returned copy", f.Path)
		}
	}
}

func TestRuleSlotsCoverEveryIndexedRule(t *testing.T) {
	d := NewDispatcher(api.Registry, nil)
	_ = writeKotlinFile(t, "class A { fun f() { g() } }\n", "Warm.kt")
	rules, slots := d.ensureFlatTypeIndex()
	if len(rules) != len(slots) {
		t.Fatalf("%d rule buckets, %d slot buckets", len(rules), len(slots))
	}
	for typeID, bucket := range rules {
		if len(bucket) != len(slots[typeID]) {
			t.Fatalf("type %d: %d rules, %d slots", typeID, len(bucket), len(slots[typeID]))
		}
		for i, r := range bucket {
			if d.slotIDs[slots[typeID][i]] != r.ID {
				t.Fatalf("type %d: slot of %s names %s", typeID, r.ID, d.slotIDs[slots[typeID][i]])
			}
		}
	}
	for i, r := range d.lineRules {
		if d.slotIDs[d.lineSlots[i]] != r.ID {
			t.Fatalf("line rule %s has slot of %s", r.ID, d.slotIDs[d.lineSlots[i]])
		}
	}
}
