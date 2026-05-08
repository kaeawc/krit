package lsp

import (
	"strings"
	"testing"
)

func TestBuildJARURIRoundTrip(t *testing.T) {
	ref := JARRef{
		JARPath: "/home/me/.gradle/caches/modules-2/files-2.1/org.jetbrains.kotlinx/kotlinx-coroutines-core/1.7.3/abc/kotlinx-coroutines-core-1.7.3.jar",
		FQN:     "kotlinx.coroutines.CoroutineScope",
	}
	uri := BuildJARURI(ref)
	if !strings.HasPrefix(uri, "krit-jar:///kotlinx-coroutines-core/1.7.3/kotlinx/coroutines/CoroutineScope.kt") {
		t.Fatalf("unexpected uri: %s", uri)
	}
	if !IsJARURI(uri) {
		t.Fatalf("IsJARURI returned false for %s", uri)
	}
	got, err := ParseJARURI(uri)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != ref {
		t.Fatalf("round trip mismatch: got %+v want %+v", got, ref)
	}
}

func TestParseJARURIRejectsOtherSchemes(t *testing.T) {
	if _, err := ParseJARURI("file:///tmp/x.kt"); err == nil {
		t.Fatal("expected error for file:// uri")
	}
	if IsJARURI("file:///tmp/x.kt") {
		t.Fatal("IsJARURI returned true for file uri")
	}
}

func TestParseJARURIMissingParams(t *testing.T) {
	if _, err := ParseJARURI("krit-jar:///foo/1.0/Bar.kt"); err == nil {
		t.Fatal("expected error when jar/fqn query params are missing")
	}
}

func TestSplitJARFilename(t *testing.T) {
	cases := []struct {
		in       string
		artifact string
		version  string
	}{
		{"/cache/kotlinx-coroutines-core-1.7.3.jar", "kotlinx-coroutines-core", "1.7.3"},
		{"kotlin-stdlib-1.9.20.jar", "kotlin-stdlib", "1.9.20"},
		{"weird.jar", "weird", "unknown"},
		{"no-version.jar", "no-version", "unknown"},
		{"", "unknown", "unknown"},
	}
	for _, tc := range cases {
		a, v := splitJARFilename(tc.in)
		if a != tc.artifact || v != tc.version {
			t.Errorf("splitJARFilename(%q) = %q, %q; want %q, %q", tc.in, a, v, tc.artifact, tc.version)
		}
	}
}
