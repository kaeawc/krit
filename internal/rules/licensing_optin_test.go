package rules_test

import (
	"strings"
	"testing"
)

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

@OptIn(ExperimentalCoroutinesApi::class)
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
	findings := runRuleByName(t, "OptInMarkerExposedPublicly", `
package test

@OptIn(ExperimentalCoroutinesApi::class)
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
