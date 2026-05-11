package api

import "testing"

func TestPrecompileIDPattern(t *testing.T) {
	good := []string{
		"K0101-UnreachableCode",
		"K0201-UnresolvedReference",
		"K0306-TypeAliasResolutionFailure",
		"K9001-OracleBudgetExceeded",
		"K0001-Reserved",
	}
	for _, id := range good {
		if !IsPrecompileID(id) {
			t.Errorf("IsPrecompileID(%q) = false, want true", id)
		}
	}
	bad := []string{
		"",
		"UnreachableCode",
		"K101-UnreachableCode",
		"K01010-UnreachableCode",
		"K0101_UnreachableCode",
		"K0101-unreachableCode",
		"K0101-",
		"k0101-UnreachableCode",
	}
	for _, id := range bad {
		if IsPrecompileID(id) {
			t.Errorf("IsPrecompileID(%q) = true, want false", id)
		}
	}
}

func TestPrecompileMetaIDPattern(t *testing.T) {
	if !IsPrecompileMetaID("K9001-OracleBudgetExceeded") {
		t.Errorf("K9001 should be classified as meta")
	}
	if IsPrecompileMetaID("K0101-UnreachableCode") {
		t.Errorf("K0101 must not be classified as meta")
	}
	if IsPrecompileMetaID("K8999-Whatever") {
		t.Errorf("K8999 must not be classified as meta (out of K9### band)")
	}
}

func TestCategoryPrecompileConstant(t *testing.T) {
	if CategoryPrecompile != "precompile" {
		t.Errorf("CategoryPrecompile = %q, want %q", CategoryPrecompile, "precompile")
	}
}
