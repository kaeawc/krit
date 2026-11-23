package rules

import (
	"context"
	"io/fs"
	"math/rand"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestDispatcherSkipsRuleWhenFileGateFails(t *testing.T) {
	file := parseInlineKt(t, "package p\n\nfun f() { g() }\n")
	for _, open := range []bool{false, true} {
		calls, gates := 0, 0
		rule := api.FakeRule("GateProbe",
			api.WithNodeTypes("call_expression"),
			api.WithCheck(func(*api.Context) { calls++ }))
		rule.FileGate = func(*scanner.File) bool { gates++; return open }
		NewDispatcher([]*api.Rule{rule}, nil).Run(file)
		if gates != 1 {
			t.Fatalf("open=%v: gate evaluated %d times, want once per file", open, gates)
		}
		if open != (calls > 0) {
			t.Fatalf("open=%v: rule ran %d times", open, calls)
		}
	}
}

func TestSourceMayNameAny(t *testing.T) {
	cases := []struct {
		lang scanner.Language
		src  string
		want bool
	}{
		{scanner.LangKotlin, "val v = AllowAllHostnameVerifier()", true},
		{scanner.LangKotlin, "import org.apache.http.conn.ssl.AllowAllHostnameVerifier as A\nval v = A()", true},
		{scanner.LangKotlin, "val v = SSLSocketFactory.ALLOW_ALL_HOSTNAME_VERIFIER", true},
		{scanner.LangKotlin, "val v = StrictHostnameVerifier()", false},
		{scanner.LangKotlin, "val v = `AllowAll``HostnameVerifier`()", true},
		{scanner.LangKotlin, "/* license */ val v = 1", false},
		{scanner.LangJava, "/* license */ class A {}", true},
		{scanner.LangJava, "class A {}", false},
	}
	for _, c := range cases {
		f := &scanner.File{Language: c.lang, Content: []byte(c.src)}
		if got := sourceMayNameAny(f, "AllowAllHostnameVerifier", "ALLOW_ALL_HOSTNAME_VERIFIER"); got != c.want {
			t.Errorf("%q: got %v, want %v", c.src, got, c.want)
		}
	}
}

// TestCompactTextHasSuffixMatchesFieldsJoin compares against the
// strings.Fields based text the rules build.
func TestCompactTextHasSuffixMatchesFieldsJoin(t *testing.T) {
	pieces := []string{"System", ".", "gc", "()", "(", ")", "runFinalization", " ", "\n", "\t", "\v", "\f", "\r",
		" ", "\u0085", " ", "　", "é", "\xff", "\xe2\x80", "g", "c", "Runtime.getRuntime()", "java.lang."}
	suffixes := []string{"gc()", "runFinalization()", "c()", ")", ""}
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20000; i++ {
		var b strings.Builder
		for n := rng.Intn(8); n >= 0; n-- {
			b.WriteString(pieces[rng.Intn(len(pieces))])
		}
		text := b.String()
		compact := strings.Join(strings.Fields(text), "")
		for _, s := range suffixes {
			if got, want := compactTextHasSuffix(text, s), strings.HasSuffix(compact, s); got != want {
				t.Fatalf("compactTextHasSuffix(%q, %q) = %v, want %v", text, s, got, want)
			}
		}
	}
}

// TestFileGatesNeverHideFixtureFindings runs every gated rule without its
// gate over the fixture corpus and fails when a file the gate rejects would
// have produced a finding.
func TestFileGatesNeverHideFixtureFindings(t *testing.T) {
	var gated []*api.Rule
	for _, r := range api.Registry {
		if r.FileGate != nil {
			gated = append(gated, r)
		}
	}
	if len(gated) == 0 {
		t.Skip("no gated rules")
	}
	var files []*scanner.File
	root := filepath.Join("..", "..", "tests", "fixtures")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || (!strings.HasSuffix(path, ".kt") && !strings.HasSuffix(path, ".java")) {
			return nil
		}
		if f, perr := scanner.ParseFile(context.Background(), path); perr == nil && f != nil {
			files = append(files, f)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range gated {
		ungated := *r
		ungated.FileGate = nil
		d := NewDispatcher([]*api.Rule{&ungated}, nil)
		rejected, withFindings := 0, 0
		for _, f := range files {
			if r.FileGate(f) {
				continue
			}
			rejected++
			if cols := d.Run(f); cols.Len() > 0 {
				t.Errorf("%s: gate rejects %s, which has %d findings", r.ID, f.Path, cols.Len())
			}
		}
		for _, f := range files {
			if !r.FileGate(f) {
				continue
			}
			if cols := d.Run(f); cols.Len() > 0 {
				withFindings++
			}
		}
		if rejected == 0 {
			t.Errorf("%s: gate rejected no fixture files", r.ID)
		}
		t.Logf("%s: gate rejected %d of %d fixture files; %d passing files have findings", r.ID, rejected, len(files), withFindings)
	}
}
