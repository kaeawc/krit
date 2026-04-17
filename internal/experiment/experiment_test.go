package experiment

import (
	"reflect"
	"testing"
)

func TestParseCSV(t *testing.T) {
	got := ParseCSV(" b ,a,a,, c ")
	want := []string{"a", "b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseCSV() = %#v, want %#v", got, want)
	}
}

func TestMergeEnabled(t *testing.T) {
	got := MergeEnabled([]string{"a"}, []string{"c", "b"}, []string{"a"})
	want := []string{"b", "c"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MergeEnabled() = %#v, want %#v", got, want)
	}
}

func TestBuildMatrixStrategies(t *testing.T) {
	got, err := BuildMatrix("baseline,singles,pairs", []string{"exp-b", "exp-a"})
	if err != nil {
		t.Fatalf("BuildMatrix() error = %v", err)
	}
	want := []MatrixCase{
		{Name: "baseline"},
		{Name: "exp-a", Enabled: []string{"exp-a"}},
		{Name: "exp-b", Enabled: []string{"exp-b"}},
		{Name: "exp-a+exp-b", Enabled: []string{"exp-a", "exp-b"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildMatrix() = %#v, want %#v", got, want)
	}
}

func TestDefinition_StatusZeroValueEqualsExperimental(t *testing.T) {
	// An entry with Status == "" must NOT be reported as promoted/deprecated,
	// and must not appear in DefaultEnabled().
	saved := knownDefinitions
	defer func() { knownDefinitions = saved }()
	knownDefinitions = []Definition{
		{Name: "zero-value-exp", Description: "zero"},
	}
	if IsPromoted("zero-value-exp") {
		t.Fatalf("zero Status should not be promoted")
	}
	if IsDeprecated("zero-value-exp") {
		t.Fatalf("zero Status should not be deprecated")
	}
	if got := DefaultEnabled(); len(got) != 0 {
		t.Fatalf("DefaultEnabled() = %#v, want empty", got)
	}
}

func TestDefaultEnabledReturnsOnlyPromoted(t *testing.T) {
	saved := knownDefinitions
	defer func() { knownDefinitions = saved }()
	knownDefinitions = []Definition{
		{Name: "exp-a", Description: "a"},
		{Name: "exp-b", Description: "b", Status: StatusPromoted},
		{Name: "exp-c", Description: "c", Status: StatusExperimental},
		{Name: "exp-d", Description: "d", Status: StatusDeprecated},
		{Name: "exp-e", Description: "e", Status: StatusPromoted},
	}
	got := DefaultEnabled()
	want := []string{"exp-b", "exp-e"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultEnabled() = %#v, want %#v", got, want)
	}
	if !IsPromoted("exp-b") {
		t.Fatalf("IsPromoted(exp-b) = false, want true")
	}
	if IsPromoted("exp-a") {
		t.Fatalf("IsPromoted(exp-a) = true, want false")
	}
}

func TestIsDeprecatedDetectsStatus(t *testing.T) {
	saved := knownDefinitions
	defer func() { knownDefinitions = saved }()
	knownDefinitions = []Definition{
		{Name: "exp-live", Description: "live", Status: StatusPromoted},
		{Name: "exp-dead", Description: "dead", Status: StatusDeprecated},
	}
	if !IsDeprecated("exp-dead") {
		t.Fatalf("IsDeprecated(exp-dead) = false, want true")
	}
	if IsDeprecated("exp-live") {
		t.Fatalf("IsDeprecated(exp-live) = true, want false")
	}
	if IsDeprecated("missing") {
		t.Fatalf("IsDeprecated(missing) = true, want false")
	}
}

func TestLookupMissingRemovedExperiments(t *testing.T) {
	for _, name := range []string{
		"no-name-shadowing-scope-pass",
		"function-flow-summary",
		"declaration-summary-extended",
	} {
		if _, ok := Lookup(name); ok {
			t.Fatalf("Lookup(%q) returned a definition, want missing", name)
		}
	}
}

func TestBuildMatrixExplicitCases(t *testing.T) {
	got, err := BuildMatrix("baseline;exp-b+exp-a", nil)
	if err != nil {
		t.Fatalf("BuildMatrix() error = %v", err)
	}
	want := []MatrixCase{
		{Name: "baseline"},
		{Name: "exp-a+exp-b", Enabled: []string{"exp-a", "exp-b"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildMatrix() = %#v, want %#v", got, want)
	}
}
