package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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
	if !canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}, map[string]uint16{path: 0}) {
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
	if canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}, map[string]uint16{path: 0}) {
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
	if canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}, map[string]uint16{path: 0}) {
		t.Fatal("unknown cross rule must remain conservative")
	}
}

// The reused findings were computed from the content savedTags describes, so
// a miss whose old content was relevant forces a rerun even when its new
// content is not: removing @VisibleForTesting drops the only text the rule
// looks for (#1844).
func TestCanReuseCrossFindingsRefusesMissRelevantBeforeTheEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "TestHooks.kt")
	const before = "package p\n\nobject TestHooks {\n    @VisibleForTesting\n    fun reset() = Unit\n}\n"
	if err := os.WriteFile(path, []byte("package p\n\nobject TestHooks {\n    fun reset() = Unit\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := []*api.Rule{api.FakeRule("VisibleForTestingCallerInNonTest", api.WithNeeds(api.NeedsCrossFile))}
	saved := lexicalTags(rules, []*scanner.File{{Path: path, Content: []byte(before)}})
	if canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}, saved) {
		t.Fatal("a miss that mentioned @VisibleForTesting before the edit was reused")
	}
	saved[path] = lexicalTag(path, "package p\n")
	if !canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}, saved) {
		t.Fatal("a miss irrelevant before and after the edit was not reused")
	}
}

// Without tags for a miss, nothing says what the reused findings saw.
func TestCanReuseCrossFindingsRefusesUntaggedMiss(t *testing.T) {
	path := filepath.Join(t.TempDir(), "A.kt")
	if err := os.WriteFile(path, []byte("package test\n\nfun f() = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := []*api.Rule{api.FakeRule("RoomLoadsAllWhereFirstUsed", api.WithNeeds(api.NeedsParsedFiles))}
	if canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}, nil) {
		t.Fatal("findings saved without tags were reused")
	}
	if canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}, map[string]uint16{"/elsewhere/A.kt": 0}) {
		t.Fatal("a miss absent from the saved tags was reused")
	}
}

// Tags are only worth computing when the lexical reuse can apply.
func TestLexicalTagsOnlyForJudgeableRules(t *testing.T) {
	files := []*scanner.File{{Path: "/p/A.kt", Content: []byte("package p\n")}}
	judgeable := []*api.Rule{api.FakeRule("OnClick", api.WithNeeds(api.NeedsResources|api.NeedsParsedFiles))}
	if tags := lexicalTags(judgeable, files); tags == nil || tags["/p/A.kt"] != 0 {
		t.Fatalf("judgeable rules: tags %v, want an entry with no bits", tags)
	}
	withDeadCode := append(judgeable, api.FakeRule("DeadCode", api.WithNeeds(api.NeedsCrossFile)))
	if tags := lexicalTags(withDeadCode, files); tags != nil {
		t.Fatalf("DeadCode active: tags %v, want none", tags)
	}
}

// OnClick reads the handlers of classes extending an *Activity, whose text
// need not mention onClick at all.
func TestOnClickLexicalRelevanceFollowsActivities(t *testing.T) {
	const activity = "class FormActivity : AppCompatActivity() {\n    fun onSubmitClicked(v: View) {}\n}\n"
	if crossRuleLexicallyIrrelevant("OnClick", "/p/FormActivity.kt", activity) {
		t.Fatal("an Activity declaring a handler is relevant to OnClick")
	}
	if !crossRuleLexicallyIrrelevant("OnClick", "/p/Util.kt", "object Util { fun f() = 1 }\n") {
		t.Fatal("a file with no Activity or click handler is irrelevant to OnClick")
	}
}
