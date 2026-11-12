package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

const (
	lintTestAndroidNS = "http://schemas.android.com/apk/res/android"
	lintTestPrefix    = "http://schemas.android.com/apk/res/"
)

// rootDecl runs rule over a layout whose root declares only the given xmlns attribute and returns the messages.
func rootDecl(t *testing.T, rule, name, uri string) []string {
	t.Helper()
	root := &android.View{Type: "FrameLayout", Line: 2, Attributes: map[string]string{
		"xmlns:android": lintTestAndroidNS, name: uri,
	}}
	var msgs []string
	for _, f := range runResourceRule(findResourceRule(t, rule), indexWithLayout("main", "res/layout/main.xml", root)) {
		msgs = append(msgs, f.Message)
	}
	return msgs
}

// The expectations below come from running Android Lint 32.4.1 on layouts that differ only in one root xmlns line.
func TestNamespaceTypoResource_FollowsLint(t *testing.T) {
	cases := []struct {
		name, uri, want string
	}{
		{"xmlns:app", lintTestPrefix + "android/", ""},
		{"xmlns:app", "http://schemas.android.com/apk/res-auto/", ""},
		{"xmlns:app", "schemas.android.com/apk/res-auto", "Suspicious namespace: should start with `http://`"},
		{"xmlns:app", "https://schemas.android.com/apk/res-auto", "Suspicious namespace: should start with `http://`"},
		{"xmlns:app", "HTTP://schemas.android.com/apk/res/foo", "Suspicious namespace: should start with `http://`"},
		{"xmlns:app", "urn:foo:bar", ""},
		{"xmlns:app", "http://schemas.android.com/tools", "Suspicious namespace and prefix combination"},
		{"xmlns:tools", "http://schemas.android.com/tool", ""},
		{"xmlns:tools", "http://schemas.android.com/tools/", ""},
		{"xmlns:tools", lintTestAndroidNS, "Suspicious namespace and prefix combination"},
		{"xmlns:app", lintTestAndroidNS, "Suspicious namespace and prefix combination"},
		{"xmlns:xapp", lintTestAndroidNS, ""},
		{"xmlns:toolsx", lintTestAndroidNS, ""},
		{"xmlns:a", lintTestAndroidNS, ""},
		{"xmlns:a", lintTestPrefix + "androi", "Unexpected namespace URI bound to the `\"android\"` prefix, was `" + lintTestPrefix + "androi`, expected `" + lintTestAndroidNS + "`"},
		{"xmlns:a", "http://example.com/foo", ""},
		{"xmlns:foo", "http://schemas.android.com/ap/res/foo", "Possible typo in URL: was `\"http://schemas.android.com/ap/res/foo\"`, should probably be `\"" + lintTestPrefix + "foo\"`"},
		{"xmlns:foo", "http://schema.android.com/apk/res/foo", "Possible typo in URL: was `\"http://schema.android.com/apk/res/foo\"`, should probably be `\"" + lintTestPrefix + "foo\"`"},
		{"xmlns:foo", "http://schemas.android.com/ak/res/foo", "Possible typo in URL: was `\"http://schemas.android.com/ak/res/foo\"`, should probably be `\"" + lintTestPrefix + "foo\"`"},
		{"xmlns:foo", "http://schemas.android.com/apk/rez/android", ""},
		{"xmlns:foo", "http://schemas.android.com/aaaaaa/res/foo", ""},
		{"xmlns:foo", "http://SCHEMAS.android.com/apk/res/android", ""},
		{"xmlns:foo", lintTestPrefix + "com.example.app", ""},
		{"xmlns:foo", lintTestPrefix + "a", ""},
		{"xmlns:foo", "ftp://schemas.android.com/apk/res/android", "Suspicious namespace: should start with `http://`"},
	}
	for _, tc := range cases {
		got := rootDecl(t, "NamespaceTypoResource", tc.name, tc.uri)
		switch {
		case tc.want == "" && len(got) != 0:
			t.Errorf("%s=%q: Lint is silent, got %v", tc.name, tc.uri, got)
		case tc.want != "" && (len(got) != 1 || got[0] != tc.want):
			t.Errorf("%s=%q: got %v, want %q", tc.name, tc.uri, got, tc.want)
		}
	}
}

func TestNamespaceTypoResource_AndroidPrefixBinding(t *testing.T) {
	run := func(uri string) []string {
		root := &android.View{Type: "FrameLayout", Line: 2, Attributes: map[string]string{"xmlns:android": uri}}
		var msgs []string
		for _, f := range runResourceRule(findResourceRule(t, "NamespaceTypoResource"), indexWithLayout("main", "res/layout/main.xml", root)) {
			msgs = append(msgs, f.Message)
		}
		return msgs
	}
	if got := run(lintTestAndroidNS); len(got) != 0 {
		t.Errorf("the real URI is clean, got %v", got)
	}
	if got := run(lintTestPrefix + "ANDROID"); len(got) != 1 || !strings.HasPrefix(got[0], "URI is case sensitive: was `\"") {
		t.Errorf("wrong case: got %v", got)
	}
	if got := run("http://example.com/other"); len(got) != 1 || !strings.HasPrefix(got[0], "Unexpected namespace URI bound to the `\"android\"` prefix") {
		t.Errorf("unrelated URI on the android prefix is still reported, got %v", got)
	}
}

func TestNamespaceTypoResource_OnlyRootDeclarations(t *testing.T) {
	child := &android.View{Type: "TextView", Line: 4, Attributes: map[string]string{"xmlns:foo": "http://schemas.android.com/ak/res/foo"}}
	root := &android.View{Type: "FrameLayout", Line: 2, Attributes: map[string]string{"xmlns:android": lintTestAndroidNS}, Children: []*android.View{child}}
	if got := runResourceRule(findResourceRule(t, "NamespaceTypoResource"), indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 0 {
		t.Errorf("Lint reads only the root element, got %v", got)
	}
	root.Attributes["tools:ignore"] = "NamespaceTypo"
	root.Attributes["xmlns:foo"] = "http://schemas.android.com/ak/res/foo"
	if got := runResourceRule(findResourceRule(t, "NamespaceTypoResource"), indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 0 {
		t.Errorf("tools:ignore silences the root declaration, got %v", got)
	}
}

func TestResAutoResource_FollowsLint(t *testing.T) {
	const autoMsg = "Suspicious namespace: Did you mean `http://schemas.android.com/apk/res-auto`?"
	const gradleMsg = "In Gradle projects, always use `http://schemas.android.com/apk/res-auto` for custom attributes"
	cases := []struct{ uri, want string }{
		{"http://schemas.android.com/apk/res-auto/", autoMsg},
		{"http://schemas.android.com/apk/res-automatic", autoMsg},
		{"http://schemas.android.com/apk/auto", autoMsg},
		{"http://schemas.android.com/apk/res-aut", ""},
		{"http://example.com/auto", ""},
		{"http://schemas.android.com/apk/res-auto", ""},
		{lintTestPrefix + "com.example.app", gradleMsg},
		{lintTestPrefix + "android/", gradleMsg},
		{lintTestAndroidNS, ""},
	}
	for _, tc := range cases {
		got := rootDecl(t, "ResAutoResource", "xmlns:app", tc.uri)
		switch {
		case tc.want == "" && len(got) != 0:
			t.Errorf("%q: got %v, want none", tc.uri, got)
		case tc.want != "" && (len(got) != 1 || got[0] != tc.want):
			t.Errorf("%q: got %v, want %q", tc.uri, got, tc.want)
		}
	}
	child := &android.View{Type: "TextView", Line: 4, Attributes: map[string]string{"xmlns:foo": lintTestPrefix + "com.example.app"}}
	root := &android.View{Type: "FrameLayout", Line: 2, Attributes: map[string]string{"xmlns:android": lintTestAndroidNS}, Children: []*android.View{child}}
	if got := runResourceRule(findResourceRule(t, "ResAutoResource"), indexWithLayout("main", "res/layout/main.xml", root)); len(got) != 0 {
		t.Errorf("Lint reads only the root element, got %v", got)
	}
}
