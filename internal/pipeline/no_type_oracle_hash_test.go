package pipeline

import "testing"

// Issue #855: --no-type-oracle runs must hash apart from oracle-backed ones,
// and runs without the flag must keep the pre-existing hash.
func TestProjectRuleHashForArgs_SeparatesNoTypeOracle(t *testing.T) {
	base := ProjectArgs{}
	off := ProjectArgs{NoTypeOracle: true}
	if got, want := projectRuleHashForArgs(base, false), projectRuleHash(base.ActiveRules, base.Config); got != want {
		t.Fatalf("default hash drifted: %s != %s", got, want)
	}
	if projectRuleHashForArgs(base, false) == projectRuleHashForArgs(off, false) {
		t.Fatal("no-type-oracle run shares a rule hash with the oracle-backed run")
	}
	if projectRuleHashForArgs(base, true) == projectRuleHashForArgs(off, true) {
		t.Fatal("editorconfig variant shares a rule hash across modes")
	}
}
