package scan

import (
	"reflect"
	"testing"

	"github.com/kaeawc/krit/internal/gradlemodel"
	"github.com/kaeawc/krit/internal/oracle"
)

// The runner hands the freshness gate the scope the oracle run is keyed on;
// an empty scope would read the settings-free entries instead.
func TestRunnerOracleExpansionScopeCarriesTheModelScope(t *testing.T) {
	kt := gradlemodel.KotlinSettings{LanguageVersion: "2.2", APIVersion: "2.2"}
	r := &runner{f: &scanFlags{
		modelKotlin:    kt,
		modelClasspath: []string{"/m/a.jar"},
		modelJvmTarget: "17",
	}}
	got := r.oracleExpansionScope()
	want := oracle.ExpansionScope{Kotlin: kt, Classpath: []string{"/m/a.jar"}, JvmTarget: "17"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("scope = %+v, want %+v", got, want)
	}
}
