package javafacts

import (
	"context"
	"strings"
	"testing"
)

func TestParseAndLookupReceiverType(t *testing.T) {
	data := []byte(`{"version":1,"calls":[{"file":"/tmp/T.java","line":4,"col":5,"callee":"setJavaScriptEnabled","receiverType":"android.webkit.WebSettings","element":"setJavaScriptEnabled(boolean)","returnType":"void"}],"classes":[]}`)
	facts, err := Parse(data)
	if err != nil {
		t.Fatal(err)
	}
	if got := facts.ReceiverType("/tmp/T.java", 4, 5); got != "android.webkit.WebSettings" {
		t.Fatalf("ReceiverType = %q", got)
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
