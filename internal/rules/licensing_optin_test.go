package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestOptInMarkerExposedPublicly_AdditionalMarkers verifies a project can
// register its own propagating opt-in markers (custom @RequiresOptIn classes)
// via `additionalMarkers`, so exposing one on a public declaration is flagged.
// The embedded well-known set only covers library markers, so an unconfigured
// project marker must NOT flag.
func TestOptInMarkerExposedPublicly_AdditionalMarkers(t *testing.T) {
	var rule *rules.OptInMarkerExposedPubliclyRule
	for _, c := range api.Registry {
		if c.ID == "OptInMarkerExposedPublicly" {
			rule, _ = c.Implementation.(*rules.OptInMarkerExposedPubliclyRule)
			break
		}
	}
	if rule == nil {
		t.Fatal("OptInMarkerExposedPublicly not registered")
	}
	original := rule.AdditionalMarkers
	defer func() { rule.AdditionalMarkers = original }()

	src := "package test\n@RequiresOptIn\nannotation class InternalMyApi\n@InternalMyApi\nfun exposed() {}\n"
	countRule := func(fs []scanner.Finding) int {
		n := 0
		for _, f := range fs {
			if f.Rule == "OptInMarkerExposedPublicly" {
				n++
			}
		}
		return n
	}

	rule.AdditionalMarkers = nil
	if n := countRule(runRuleByName(t, "OptInMarkerExposedPublicly", src)); n != 0 {
		t.Fatalf("baseline: an unconfigured project marker should not be flagged; got %d", n)
	}

	rules.ApplyConfig(loadTempConfig(t, "licensing:\n  OptInMarkerExposedPublicly:\n    additionalMarkers:\n      - InternalMyApi\n"))
	if n := countRule(runRuleByName(t, "OptInMarkerExposedPublicly", src)); n == 0 {
		t.Fatal("with additionalMarkers configured, exposing the project marker on a public declaration should be flagged; got 0")
	}
}

func TestOptInMarkerNotRecognised_Positive(t *testing.T) {
	findings := runRuleByName(t, "OptInMarkerNotRecognised", `
package test

@OptIn(RemovedExperimentalApi::class)
fun staleMarker() {
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unrecognised OptIn marker")
	}
}

func TestOptInMarkerNotRecognised_Negative(t *testing.T) {
	findings := runRuleByName(t, "OptInMarkerNotRecognised", `
package test

@OptIn(ExperimentalCoroutinesApi::class)
fun knownMarker() {
}
`)
	for _, f := range findings {
		if f.Rule == "OptInMarkerNotRecognised" {
			t.Fatalf("did not expect finding: %s", f.Message)
		}
	}
}

func TestOptInMarkerNotRecognised_FullyQualifiedKnownMarker(t *testing.T) {
	findings := runRuleByName(t, "OptInMarkerNotRecognised", `
package test

@OptIn(kotlinx.coroutines.ExperimentalCoroutinesApi::class)
fun knownMarker() {
}
`)
	for _, f := range findings {
		if f.Rule == "OptInMarkerNotRecognised" {
			t.Fatalf("did not expect finding: %s", f.Message)
		}
	}
}

func TestOptInMarkerNotRecognised_MultipleMarkersFlagsOnlyUnknown(t *testing.T) {
	findings := runRuleByName(t, "OptInMarkerNotRecognised", `
package test

@OptIn(ExperimentalCoroutinesApi::class, RemovedExperimentalApi::class)
fun mixed() {
}
`)
	count := 0
	for _, f := range findings {
		if f.Rule == "OptInMarkerNotRecognised" {
			count++
			if !strings.Contains(f.Message, "RemovedExperimentalApi") {
				t.Errorf("expected message to reference RemovedExperimentalApi, got %q", f.Message)
			}
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 finding, got %d", count)
	}
}

func TestOptInMarkerExposedPublicly_SkipsTestSources(t *testing.T) {
	file := parseInline(t, `
package test

@ExperimentalCoroutinesApi
class CoroutineRobot
`)
	file.Path = "/repo/app/src/androidTest/kotlin/com/example/CoroutineRobot.kt"
	findings := runRuleByNameOnFile(t, "OptInMarkerExposedPublicly", file)
	for _, f := range findings {
		if f.Rule == "OptInMarkerExposedPublicly" {
			t.Fatalf("did not expect finding in test source: %s", f.Message)
		}
	}
}

func TestOptInMarkerExposedPublicly_FlagsProductionPublicApi(t *testing.T) {
	// A public declaration carrying a propagating opt-in marker DIRECTLY
	// exposes the opt-in requirement to callers.
	findings := runRuleByName(t, "OptInMarkerExposedPublicly", `
package test

@ExperimentalCoroutinesApi
class PublicApi
`)
	count := 0
	for _, f := range findings {
		if f.Rule == "OptInMarkerExposedPublicly" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected 1 OptInMarkerExposedPublicly finding, got %d", count)
	}
}

func TestOptInMarkerExposedPublicly_DoesNotFlagOptInConsumer(t *testing.T) {
	// Regression: @OptIn(Foo::class) CONSUMES the requirement locally and does
	// NOT propagate to callers, so it must never be flagged.
	findings := runRuleByName(t, "OptInMarkerExposedPublicly", `
package test

@OptIn(ExperimentalCoroutinesApi::class)
fun publicApi() {
}
`)
	for _, f := range findings {
		if f.Rule == "OptInMarkerExposedPublicly" {
			t.Fatalf("did not expect finding for @OptIn consumer: %s", f.Message)
		}
	}
}

func TestOptInMarkerExposedPublicly_SkipsNonPublicDirectMarker(t *testing.T) {
	findings := runRuleByName(t, "OptInMarkerExposedPublicly", `
package test

@ExperimentalCoroutinesApi
internal fun internalApi() {
}

@ExperimentalCoroutinesApi
private fun privateApi() {
}
`)
	for _, f := range findings {
		if f.Rule == "OptInMarkerExposedPublicly" {
			t.Fatalf("did not expect finding for non-public declaration: %s", f.Message)
		}
	}
}
