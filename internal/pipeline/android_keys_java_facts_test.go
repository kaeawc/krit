package pipeline

import (
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
)

// TestAndroidKeys_StableAcrossJavaSemanticFactsFP pins the contract
// that XML-only Android cache keys (resource, manifest, icon, plus
// their bundle variants) MUST NOT change when only
// JavaSemanticFactsFP rotates. Folding semantic-facts into these
// keys would invalidate the entire Android cache on every .java
// edit even though the underlying rules can't observe Java semantic
// facts (their dispatch language is LangXML and the only consumer
// of ctx.JavaSemanticFacts hard-gates on LangJava). The full path
// breakdown lives next to each key constructor in android_keys.go.
func TestAndroidKeys_StableAcrossJavaSemanticFactsFP(t *testing.T) {
	mkInput := func(javaFP string) AndroidInput {
		return AndroidInput{
			RuleHash:            "rh",
			LibraryFactsFP:      "lf",
			JavaSemanticFactsFP: javaFP,
		}
	}
	a := mkInput("java-v1")
	b := mkInput("java-v2")

	type stableCase struct {
		name string
		fn   func(in AndroidInput) string
	}
	cases := []stableCase{
		{"resourceKey", func(in AndroidInput) string {
			return in.resourceKey("/res", "fp", rules.AndroidDepLayout, android.ValuesScanStrings)
		}},
		{"resourceBundleKey", func(in AndroidInput) string {
			return in.resourceBundleKey("merged", rules.AndroidDepLayout, android.ValuesScanStrings)
		}},
		{"manifestKey", func(in AndroidInput) string {
			return in.manifestKey("content", "bundle")
		}},
		{"manifestBundleKey", func(in AndroidInput) string {
			return in.manifestBundleKey("bundle")
		}},
		{"iconKey", func(in AndroidInput) string {
			return in.iconKey("/res", "fp")
		}},
		{"iconBundleKey", func(in AndroidInput) string {
			return in.iconBundleKey("merged")
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got, want := c.fn(a), c.fn(b); got != want {
				t.Errorf("%s changed when only JavaSemanticFactsFP rotated\n  java-v1: %s\n  java-v2: %s", c.name, got, want)
			}
		})
	}
}

// TestAndroidKeys_RuleHashStillInvalidates is the regression-critical
// counterpart: dropping JavaSemanticFactsFP must NOT also break the
// other key contributions. RuleHash changes (e.g. toggling a rule
// on/off via config) must still rotate every key so a different
// rule set never serves another rule set's cached findings.
func TestAndroidKeys_RuleHashStillInvalidates(t *testing.T) {
	a := AndroidInput{RuleHash: "rh-1", LibraryFactsFP: "lf"}
	b := AndroidInput{RuleHash: "rh-2", LibraryFactsFP: "lf"}
	if a.resourceKey("/res", "fp", rules.AndroidDepLayout, android.ValuesScanNone) ==
		b.resourceKey("/res", "fp", rules.AndroidDepLayout, android.ValuesScanNone) {
		t.Error("resourceKey did not change when RuleHash rotated")
	}
	if a.manifestKey("content", "bundle") == b.manifestKey("content", "bundle") {
		t.Error("manifestKey did not change when RuleHash rotated")
	}
	if a.iconKey("/res", "fp") == b.iconKey("/res", "fp") {
		t.Error("iconKey did not change when RuleHash rotated")
	}
}

// TestAndroidKeys_LibraryFactsStillInvalidates: library facts (Gradle
// SDK versions, dependency closure) can legitimately affect resource
// rule output (e.g. a rule that gates on minSdk). Dropping
// JavaSemanticFactsFP must not also un-gate this contribution.
func TestAndroidKeys_LibraryFactsStillInvalidates(t *testing.T) {
	a := AndroidInput{RuleHash: "rh", LibraryFactsFP: "lf-1"}
	b := AndroidInput{RuleHash: "rh", LibraryFactsFP: "lf-2"}
	if a.resourceKey("/res", "fp", rules.AndroidDepLayout, android.ValuesScanNone) ==
		b.resourceKey("/res", "fp", rules.AndroidDepLayout, android.ValuesScanNone) {
		t.Error("resourceKey did not change when LibraryFactsFP rotated")
	}
}
