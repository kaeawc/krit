package pipeline

import (
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestAffectedSetReplayEnabled_DefaultOff pins the opt-in contract: the
// affected-set replay path is OFF unless KRIT_AFFECTED_SET_REPLAY is set to a
// recognized truthy value.
func TestAffectedSetReplayEnabled_DefaultOff(t *testing.T) {
	t.Setenv("KRIT_AFFECTED_SET_REPLAY", "")
	if affectedSetReplayEnabled() {
		t.Errorf("replay must be OFF when the env var is empty")
	}

	for _, v := range []string{"0", "off", "false", "no", "nope", " "} {
		t.Setenv("KRIT_AFFECTED_SET_REPLAY", v)
		if affectedSetReplayEnabled() {
			t.Errorf("replay must be OFF for %q", v)
		}
	}

	for _, v := range []string{"1", "true", "on", "yes", "TRUE", "On", " yes "} {
		t.Setenv("KRIT_AFFECTED_SET_REPLAY", v)
		if !affectedSetReplayEnabled() {
			t.Errorf("replay must be ON for %q", v)
		}
	}
}

// TestScopeParseResultMulti narrows the parse result to the requested paths,
// preserving language routing and dropping everything else.
func TestScopeParseResultMulti(t *testing.T) {
	kt1 := &scanner.File{Path: "a.kt", Language: scanner.LangKotlin}
	kt2 := &scanner.File{Path: "b.kt", Language: scanner.LangKotlin}
	jv1 := &scanner.File{Path: "C.java", Language: scanner.LangJava}
	jv2 := &scanner.File{Path: "D.java", Language: scanner.LangJava}
	p := ParseResult{
		KotlinFiles: []*scanner.File{kt1, kt2, nil},
		JavaFiles:   []*scanner.File{jv1, jv2},
	}

	scoped := scopeParseResultMulti(p, map[string]bool{"a.kt": true, "D.java": true})

	gotKt := pathsOf(scoped.KotlinFiles)
	if len(gotKt) != 1 || gotKt[0] != "a.kt" {
		t.Errorf("kotlin scoped = %v, want [a.kt]", gotKt)
	}
	gotJv := pathsOf(scoped.JavaFiles)
	if len(gotJv) != 1 || gotJv[0] != "D.java" {
		t.Errorf("java scoped = %v, want [D.java]", gotJv)
	}
}

// TestScopeParseResultMulti_EmptyPaths returns an empty parse result when no
// paths are requested.
func TestScopeParseResultMulti_EmptyPaths(t *testing.T) {
	p := ParseResult{
		KotlinFiles: []*scanner.File{{Path: "a.kt", Language: scanner.LangKotlin}},
		JavaFiles:   []*scanner.File{{Path: "C.java", Language: scanner.LangJava}},
	}
	scoped := scopeParseResultMulti(p, nil)
	if len(scoped.KotlinFiles) != 0 || len(scoped.JavaFiles) != 0 {
		t.Errorf("empty paths must yield no files; got kt=%v java=%v",
			pathsOf(scoped.KotlinFiles), pathsOf(scoped.JavaFiles))
	}
}

// TestAllAffectedFilesParsed is the #608 gate: the affected-set replay path
// must bail unless every affected file is present in the (warm, dirty-only)
// parse result. An affected reverse-dependency that was not re-parsed cannot
// be re-dispatched, so ApplyDelta would drop its prior rows with no
// replacement — losing findings.
func TestAllAffectedFilesParsed(t *testing.T) {
	p := ParseResult{
		KotlinFiles: []*scanner.File{
			{Path: "a.kt", Language: scanner.LangKotlin},
			nil,
		},
		JavaFiles: []*scanner.File{{Path: "C.java", Language: scanner.LangJava}},
	}

	if !allAffectedFilesParsed(p, []string{"a.kt", "C.java"}) {
		t.Errorf("all affected files present must report true")
	}
	if !allAffectedFilesParsed(p, nil) {
		t.Errorf("empty affected set must report true (nothing to dispatch)")
	}
	// A non-dirty dependent (b.kt) that was not re-parsed must fail the gate.
	if allAffectedFilesParsed(p, []string{"a.kt", "b.kt"}) {
		t.Errorf("affected file missing from parse result must report false")
	}
	// An XML referrer is never in Kotlin/Java parse files -> must fail.
	if allAffectedFilesParsed(p, []string{"layout.xml"}) {
		t.Errorf("XML referrer not in parse result must report false")
	}
}

func pathsOf(files []*scanner.File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		if f != nil {
			out = append(out, f.Path)
		}
	}
	sort.Strings(out)
	return out
}
