package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
		api.FakeRule("RoomLoadsAllWhereFirstUsed", api.WithNeeds(api.NeedsParsedFiles)),
		api.FakeRule("RoomRelationWithoutIndex", api.WithNeeds(api.NeedsCrossFile)),
		api.FakeRule("AnvilMergeComponentEmptyScope", api.WithNeeds(api.NeedsCrossFile|api.NeedsParsedFiles)),
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
	rules := []*api.Rule{api.FakeRule("RoomLoadsAllWhereFirstUsed", api.WithNeeds(api.NeedsParsedFiles))}
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
// content is not: removing @Relation drops the only text the rule looks for
// (#1844).
func TestCanReuseCrossFindingsRefusesMissRelevantBeforeTheEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "UserWithPets.kt")
	const before = "package p\n\ndata class UserWithPets(\n    @Relation(parentColumn = \"id\", entityColumn = \"ownerId\")\n    val pets: List<Pet>,\n)\n"
	if err := os.WriteFile(path, []byte("package p\n\ndata class UserWithPets(\n    val pets: List<Pet>,\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := []*api.Rule{api.FakeRule("RoomRelationWithoutIndex", api.WithNeeds(api.NeedsCrossFile))}
	saved := lexicalTags(rules, []*scanner.File{{Path: path, Content: []byte(before)}})
	if canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}, saved) {
		t.Fatal("a miss that mentioned @Relation before the edit was reused")
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

// These rules report on callers that need not name the annotation, fixture
// or query they flag, so no needle in the edited file can clear an edit:
// changing a caller of a @VisibleForTesting function changed the findings
// unseen (#1893). They are never judged.
func TestLexicalReuseNeverJudgesCallerSideRules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "Production.kt")
	if err := os.WriteFile(path, []byte("package p\n\nfun production() {\n    println(\"x\")\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"VisibleForTestingCallerInNonTest", "TestFixtureAccessedFromProduction", "DatabaseQueryOnMainThread"} {
		rules := []*api.Rule{api.FakeRule(id, api.WithNeeds(api.NeedsCrossFile|api.NeedsParsedFiles))}
		if lexicalReuseApplies(rules) {
			t.Errorf("%s: lexical reuse applies to a rule that reports on callers", id)
		}
		if canReuseCrossFindingsForLexicallyIrrelevantMisses(rules, []string{path}, map[string]uint16{path: 0}) {
			t.Errorf("%s: a caller edit reused the cross findings", id)
		}
	}
}

// The reuse key ties saved tags to the rules and the needle meaning.
func TestLexicalReuseKeyCoversRulesAndNeedleVersion(t *testing.T) {
	if lexicalReuseKey("rules-a") == lexicalReuseKey("rules-b") {
		t.Fatal("the reuse key ignores the rule hash")
	}
	if !strings.Contains(lexicalReuseKey("rules-a"), fmt.Sprintf("v%d", lexicalNeedleVersion)) {
		t.Fatal("the reuse key ignores the needle version")
	}
}

// The last-value cross-findings slot is reused without its key. Switching
// the only cross-file rule made every file a miss, irrelevant to the new
// rule, so the old rule's findings were replayed (#1893).
func TestWarmCrossFindingsLexicalReuseRefusesOtherRules(t *testing.T) {
	h := newWarmCrossHarness(t, "RoomLoadsAllWhereFirstUsed", map[string]string{
		"A.kt": "package test\n\nclass A\n",
		"B.kt": "package test\n\nclass B\n",
	})
	h.wantFindings("first rule, cold", 2, false)
	h.rule = api.FakeRule("AnvilMergeComponentEmptyScope", api.WithNeeds(api.NeedsCrossFile), api.WithCheck(func(*api.Context) {}))
	if got := h.run().FindingsCount; got != 0 {
		t.Fatalf("after switching to a rule that reports nothing: %d findings, want 0 (the first rule's were replayed)", got)
	}
}
