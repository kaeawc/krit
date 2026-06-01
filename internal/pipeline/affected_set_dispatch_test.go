package pipeline

import (
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/scanner"
)

// TestAffectedSetReplayEnabled_DefaultOn pins the opt-out contract: the
// affected-set replay path is ON by default; KRIT_AFFECTED_SET_REPLAY is a
// kill switch that only disables on a recognized falsy value.
func TestAffectedSetReplayEnabled_DefaultOn(t *testing.T) {
	t.Setenv("KRIT_AFFECTED_SET_REPLAY", "")
	if !affectedSetReplayEnabled() {
		t.Errorf("replay must be ON when the env var is empty (default on)")
	}

	for _, v := range []string{"0", "off", "false", "no", "OFF", "False", " off "} {
		t.Setenv("KRIT_AFFECTED_SET_REPLAY", v)
		if affectedSetReplayEnabled() {
			t.Errorf("replay must be OFF for kill-switch value %q", v)
		}
	}

	for _, v := range []string{"1", "true", "on", "yes", "maybe", " "} {
		t.Setenv("KRIT_AFFECTED_SET_REPLAY", v)
		if !affectedSetReplayEnabled() {
			t.Errorf("replay must be ON for %q (only explicit falsy disables)", v)
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
// collects the paths of a parse result's Kotlin and Java files, skipping nils.
// It backs invariant 2 of tryAffectedSetDispatch (deciding which affected
// files still need to be materialized for re-dispatch).
func TestParsedPathSet(t *testing.T) {
	p := ParseResult{
		KotlinFiles: []*scanner.File{
			{Path: "a.kt", Language: scanner.LangKotlin},
			nil,
		},
		JavaFiles: []*scanner.File{{Path: "C.java", Language: scanner.LangJava}},
	}

	got := parsedPathSet(p)
	if !got["a.kt"] || !got["C.java"] {
		t.Errorf("parsed path set must contain a.kt and C.java; got %v", got)
	}
	if got["b.kt"] {
		t.Errorf("parsed path set must not contain unparsed b.kt; got %v", got)
	}
	if len(got) != 2 {
		t.Errorf("nil entries must be skipped; got %d paths %v", len(got), got)
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
