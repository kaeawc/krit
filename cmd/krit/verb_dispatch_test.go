package main

import (
	"os"
	"testing"
)

func TestClassifyVerb(t *testing.T) {
	cases := []struct {
		arg  string
		want subcommandVerb
	}{
		{"cache", verbCache},
		{"serve", verbServe},
		{"harvest", verbHarvest},
		{"rename", verbRename},
		{"init", verbInit},
		{"api-snapshot", verbAPISnapshot},
		{"api-diff", verbAPIDiff},
		{"abi-hash", verbABIHash},
		{"impact", verbImpact},
		{"dead-code", verbDeadCode},
		{"used-symbols", verbUsedSymbols},
		{"test-coverage", verbTestCoverage},
		{"select-tests", verbSelectTests},
		{"mocks", verbMocks},
		{"transform", verbTransform},
		{"migrate", verbMigrate},
		{"metrics", verbMetrics},
		{"score", verbScore},
		{"scorecard", verbScorecard},
		{"risk-map", verbRiskMap},
		{"blast-radius", verbBlastRadius},
		{"baseline-audit", verbBaselineAudit},
		{"suggest-reviewers", verbSuggestReviewers},
		{"editorconfig-drift", verbEditorConfigDrift},
		{"gen", verbGen},
		{"graph", verbGraph},
		{"", verbNone},
		{"scan", verbNone},
		{"--help", verbNone},
		{"src/", verbNone},
		{"Cache", verbNone},
		{"CACHE", verbNone},
		{"baselineaudit", verbNone},
	}
	for _, tc := range cases {
		t.Run(tc.arg, func(t *testing.T) {
			if got := classifyVerb(tc.arg); got != tc.want {
				t.Fatalf("classifyVerb(%q) = %d, want %d", tc.arg, got, tc.want)
			}
		})
	}
}

func TestDispatchSubcommandNoArgs(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()

	os.Args = []string{"krit"}
	if dispatchSubcommand() {
		t.Fatal("dispatchSubcommand() = true, want false when no verb is present")
	}
}

func TestDispatchSubcommandNonVerb(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()

	os.Args = []string{"krit", "src/", "--all-rules"}
	if dispatchSubcommand() {
		t.Fatal("dispatchSubcommand() = true, want false for plain scan args")
	}
	if got, want := os.Args, []string{"krit", "src/", "--all-rules"}; !equalSlices(got, want) {
		t.Fatalf("os.Args mutated: got %v, want %v", got, want)
	}
}

func TestDispatchSubcommandBaselineAuditRewritesArgs(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()

	os.Args = []string{"krit", "baseline-audit", "--baseline", "b.xml", "src/"}
	if !dispatchSubcommand() {
		t.Fatal("dispatchSubcommand() = false, want true for baseline-audit verb")
	}
	want := []string{"krit", "--baseline", "b.xml", "src/"}
	if !equalSlices(os.Args, want) {
		t.Fatalf("os.Args = %v, want %v (verb should be removed)", os.Args, want)
	}
}

func TestDispatchSubcommandBaselineAuditOnlyVerb(t *testing.T) {
	saved := os.Args
	defer func() { os.Args = saved }()

	os.Args = []string{"krit", "baseline-audit"}
	if !dispatchSubcommand() {
		t.Fatal("dispatchSubcommand() = false, want true")
	}
	want := []string{"krit"}
	if !equalSlices(os.Args, want) {
		t.Fatalf("os.Args = %v, want %v", os.Args, want)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
