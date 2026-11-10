package rules_test

import (
	"fmt"
	"testing"
)

func TestUnnecessaryAnnotationUseSiteTarget(t *testing.T) {
	const source = `package test

annotation class Ann
annotation class Both

@property:Ann val top = 1
@property:[Ann Both] val grouped = 1
@property:Ann val (first, second) = 1 to 2
@param:Ann val notAParameter = 1

class Primary(@param:Ann val a: Int, @property:Ann val b: Int, @param:Ann c: Int, @field:Ann @param:Ann val d: Int) {
    @property:Ann val member = 1
    @property:Ann private val hidden = 1
    @get:Ann val getter = 1
    @field:Ann val field = 1
    @delegate:Ann val delegated by lazy { 1 }
    constructor(@param:Ann secondary: String) : this(1, 2, 3, 4)

    fun method(@param:Ann p: Int, @property:Ann q: Int) {
        @property:Ann val local = 1
    }
}

interface Contract {
    @property:Ann val declared: Int
}
`
	findings := runRuleByName(t, "UnnecessaryAnnotationUseSiteTarget", source)
	got := make([]string, 0, len(findings))
	for _, f := range findings {
		got = append(got, fmt.Sprintf("%d:%d", f.Line, f.Col))
	}
	want := []string{"6:2", "7:2", "7:2", "11:16", "11:65", "11:95", "12:6", "13:6", "20:10", "25:6"}
	if len(got) != len(want) {
		t.Fatalf("expected findings at %v, got %v", want, got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("finding %d at %s, want %s (got %v)", i, got[i], want[i], got)
		}
	}
}
