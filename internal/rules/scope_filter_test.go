package rules

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules/api"
)

func TestParseScopeSet(t *testing.T) {
	set, err := ParseScopeSet(" File , cross-file")
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range append(append([]api.Scope{}, fileScopes...), api.ScopeCrossFile) {
		if !set[s] {
			t.Errorf("set is missing %s", s)
		}
	}
	if set[api.ScopeGradle] {
		t.Error("set unexpectedly contains gradle")
	}

	if set, err := ParseScopeSet(""); err != nil || set != nil {
		t.Errorf("ParseScopeSet(\"\") = (%v, %v), want (nil, nil)", set, err)
	}
	if _, err := ParseScopeSet("file,per-module"); err == nil || !strings.Contains(err.Error(), "per-module") {
		t.Errorf("unknown token error = %v, want it to name the token", err)
	}
}

// Every dispatcher scope belongs to exactly one group, so --only-scope=file
// and --only-scope=project shards together run every rule once.
func TestScopeGroupsPartitionEveryScope(t *testing.T) {
	file, _ := ParseScopeSet(ScopeGroupFile)
	project, _ := ParseScopeSet(ScopeGroupProject)
	for s := api.ScopeUnset; s <= api.ScopeGradle; s++ {
		if file[s] == project[s] {
			t.Errorf("scope %s: file=%v project=%v, want exactly one", s, file[s], project[s])
		}
	}
}

func TestFilterByScopePartitionsRegistry(t *testing.T) {
	file, _ := ParseScopeSet(ScopeGroupFile)
	project, _ := ParseScopeSet(ScopeGroupProject)
	fileRules := FilterByScope(api.Registry, file, nil)
	projectRules := FilterByScope(api.Registry, project, nil)
	if len(fileRules) == 0 || len(projectRules) == 0 {
		t.Fatalf("file=%d project=%d rules, want both non-empty", len(fileRules), len(projectRules))
	}
	if len(fileRules)+len(projectRules) != len(api.Registry) {
		t.Errorf("file %d + project %d != registry %d", len(fileRules), len(projectRules), len(api.Registry))
	}
	for _, r := range fileRules {
		if ScopeGroup(r) != ScopeGroupFile {
			t.Errorf("%s selected as file but grouped %s", r.ID, ScopeGroup(r))
		}
		if r.Needs.Has(api.NeedsCrossFile) || r.Needs.Has(api.NeedsModuleIndex) || r.Needs.Has(api.NeedsParsedFiles) {
			t.Errorf("%s needs project data but is in the file group", r.ID)
		}
	}
	excluded := FilterByScope(api.Registry, nil, project)
	if len(excluded) != len(fileRules) {
		t.Errorf("--exclude-scope=project kept %d rules, want the %d file rules", len(excluded), len(fileRules))
	}
}

func TestFilterByScopeOnlyAndExclude(t *testing.T) {
	line := &api.Rule{ID: "Line", Needs: api.NeedsLinePass}
	node := &api.Rule{ID: "Node", NodeTypes: []string{"call_expression"}}
	cross := &api.Rule{ID: "Cross", Needs: api.NeedsCrossFile}
	all := []*api.Rule{line, node, cross}

	if got := FilterByScope(all, nil, nil); len(got) != 3 {
		t.Errorf("no filter kept %d rules, want 3", len(got))
	}
	only, _ := ParseScopeSet("file")
	exclude, _ := ParseScopeSet("line-pass")
	got := FilterByScope(all, only, exclude)
	if len(got) != 1 || got[0] != node {
		t.Errorf("--only-scope=file --exclude-scope=line-pass = %v, want [Node]", ruleIDs(got))
	}
}
