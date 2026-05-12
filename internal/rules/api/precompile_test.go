package api

import "testing"

func TestCategoryPrecompileConstant(t *testing.T) {
	if CategoryPrecompile != "precompile" {
		t.Errorf("CategoryPrecompile = %q, want %q", CategoryPrecompile, "precompile")
	}
}

func TestRuleLevelString(t *testing.T) {
	cases := []struct {
		level RuleLevel
		want  string
	}{
		{LevelUnset, "unset"},
		{LevelFunction, "function"},
		{LevelFile, "file"},
		{LevelModule, "module"},
		{LevelExternal, "external"},
		{LevelGenerated, "generated"},
		{LevelMeta, "meta"},
	}
	for _, tc := range cases {
		if got := tc.level.String(); got != tc.want {
			t.Errorf("RuleLevel(%d).String() = %q, want %q", tc.level, got, tc.want)
		}
	}
}
