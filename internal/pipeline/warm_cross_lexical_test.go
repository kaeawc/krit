package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

func TestCanReuseCrossFindingsForLexicallyIrrelevantMisses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "A.kt")
	if err := os.WriteFile(path, []byte("package test\n\npublic fun addedApi(): Int = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := []*api.Rule{
		api.FakeRule("DatabaseQueryOnMainThread", api.WithNeeds(api.NeedsParsedFiles)),
		api.FakeRule("RoomLoadsAllWhereFirstUsed", api.WithNeeds(api.NeedsParsedFiles)),
		api.FakeRule("TestFixtureAccessedFromProduction", api.WithNeeds(api.NeedsCrossFile|api.NeedsParsedFiles)),
	}
	if !canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}) {
		t.Fatal("plain public API addition should be lexically irrelevant to these cross rules")
	}
}

func TestCanReuseCrossFindingsForLexicallyRelevantMisses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "A.kt")
	if err := os.WriteFile(path, []byte("package test\n\n@Dao interface D { @Query(\"select * from t\") fun getAll(): List<T> }\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := []*api.Rule{
		api.FakeRule("DatabaseQueryOnMainThread", api.WithNeeds(api.NeedsParsedFiles)),
		api.FakeRule("RoomLoadsAllWhereFirstUsed", api.WithNeeds(api.NeedsParsedFiles)),
	}
	if canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}) {
		t.Fatal("Room/DB edit should force cross-rule rerun")
	}
}

func TestCanReuseCrossFindingsForUnknownCrossRuleMisses(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "A.kt")
	if err := os.WriteFile(path, []byte("package test\n\npublic fun addedApi(): Int = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := []*api.Rule{api.FakeRule("UnknownCross", api.WithNeeds(api.NeedsCrossFile))}
	if canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}) {
		t.Fatal("unknown cross rule must remain conservative")
	}
}
