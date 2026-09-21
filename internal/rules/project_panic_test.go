package rules

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestDispatcher_ProjectRulePanicsAreQueryable(t *testing.T) {
	rule := &api.Rule{
		ID: "GradlePanic", Category: "test", Description: "test", Scope: api.ScopeGradle, Needs: api.NeedsGradle,
		Check: func(*api.Context) { panic("boom") },
	}
	dispatcher := NewDispatcher([]*api.Rule{rule}, nil)
	_, cacheable := dispatcher.RunGradle(&scanner.File{Path: "build.gradle.kts", Language: scanner.LangGradle}, nil)
	if cacheable {
		t.Fatal("RunGradle cacheable = true after recovered panic, want false")
	}

	stats := dispatcher.ProjectRuleStats()
	if len(stats.Errors) != 1 {
		t.Fatalf("ProjectRuleStats.Errors = %+v, want one error", stats.Errors)
	}
	got := stats.Errors[0]
	if got.RuleName != "GradlePanic" || got.FilePath != "build.gradle.kts" || got.PanicValue != "boom" {
		t.Errorf("ProjectRuleStats.Errors[0] = %+v, want recovered GradlePanic", got)
	}
}
