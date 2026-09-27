package javafacts

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAndLookupReceiverType(t *testing.T) {
	data := []byte(`{"version":1,"calls":[{"file":"/tmp/T.java","line":4,"col":5,"callee":"setJavaScriptEnabled","receiverType":"android.webkit.WebSettings","methodOwner":"android.webkit.WebSettings","element":"setJavaScriptEnabled(boolean)","returnType":"void","annotations":["androidx.annotation.CheckResult"]}],"classes":[{"file":"/tmp/T.java","line":2,"col":1,"name":"Browser","qualifiedName":"test.Browser","supertypes":["android.app.Activity"]}]}`)
	facts, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := facts.ReceiverType("/tmp/T.java", 4, 5); got != "android.webkit.WebSettings" {
		t.Fatalf("ReceiverType = %q", got)
	}
	if got := facts.MethodOwner("/tmp/T.java", 4, 5); got != "android.webkit.WebSettings" {
		t.Fatalf("MethodOwner = %q", got)
	}
	if got := facts.ReturnType("/tmp/T.java", 4, 5); got != "void" {
		t.Fatalf("ReturnType = %q", got)
	}
	if !facts.HasCallAnnotation("/tmp/T.java", 4, 5, "CheckResult") {
		t.Fatal("expected CheckResult annotation")
	}
	if got := facts.ClassSupertypes("/tmp/T.java", 2, 1); len(got) != 1 || got[0] != "android.app.Activity" {
		t.Fatalf("ClassSupertypes = %#v", got)
	}
}

func TestUnavailableWarning(t *testing.T) {
	if got := UnavailableWarning(assertErr("missing javac")); got == "" {
		t.Fatal("expected warning")
	}
}

func TestInvokeMissingJavaFallsBackWithWarning(t *testing.T) {
	facts, warning, err := Invoke(context.Background(), "helper", []string{"Test.java"}, Options{Java: "krit-missing-java"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if facts != nil {
		t.Fatalf("expected no facts when helper is unavailable, got %+v", facts)
	}
	if !strings.Contains(warning, "continuing with conservative source analysis") {
		t.Fatalf("expected conservative fallback warning, got %q", warning)
	}
}

type assertErr string

func (e assertErr) Error() string { return string(e) }

func TestLookupMatchesRelativeAndAbsoluteSpellings(t *testing.T) {
	abs, err := filepath.Abs(filepath.Join("src", "T.java"))
	if err != nil {
		t.Fatal(err)
	}
	facts := &Facts{
		Version: Version,
		Calls: []CallFact{
			{File: "src/./T.java", Line: 3, Col: 7, ReceiverType: "first"},
			{File: abs, Line: 3, Col: 7, ReceiverType: "second"},
			{File: abs, Line: 3, Col: 8, ReceiverType: "other"},
		},
		Classes: []ClassFact{{File: "src/T.java", Line: 1, Col: 1, Supertypes: []string{"Base"}}},
	}
	if got := facts.ReceiverType(abs, 3, 7); got != "first" {
		t.Fatalf("ReceiverType(abs) = %q, want the first matching fact", got)
	}
	if got := facts.ReceiverType("src/T.java", 3, 8); got != "other" {
		t.Fatalf("ReceiverType(relative) = %q", got)
	}
	if _, ok := facts.CallAt(abs, 3, 9); ok {
		t.Fatal("CallAt matched a column with no fact")
	}
	if got := facts.ClassSupertypes(abs, 1, 1); len(got) != 1 || got[0] != "Base" {
		t.Fatalf("ClassSupertypes = %#v", got)
	}
	var nilFacts *Facts
	if _, ok := nilFacts.CallAt(abs, 3, 7); ok {
		t.Fatal("nil Facts returned a call")
	}
}
