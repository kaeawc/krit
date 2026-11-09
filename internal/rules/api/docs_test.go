package api

import (
	"strings"
	"testing"
)

func TestDocsURLDerivation(t *testing.T) {
	tests := []struct {
		name     string
		resolver DocsResolver
		rule     *Rule
		want     string
	}{
		{
			name:     "explicit DocsURL wins",
			resolver: DefaultDocsResolver{},
			rule:     &Rule{ID: "MagicNumber", DocsURL: "https://example.com/magic"},
			want:     "https://example.com/magic",
		},
		{
			name:     "derived from default base",
			resolver: DefaultDocsResolver{},
			rule:     &Rule{ID: "MagicNumber", Category: "style"},
			want:     "https://kaeawc.github.io/krit/rules/style/magic-number/",
		},
		{
			name:     "custom base URL",
			resolver: DefaultDocsResolver{BaseURL: "https://docs.example.com/r/"},
			rule:     &Rule{ID: "MagicNumber"},
			want:     "https://docs.example.com/r/other/magic-number/",
		},
		{
			name:     "nil rule returns empty",
			resolver: DefaultDocsResolver{},
			rule:     nil,
			want:     "",
		},
		{
			name:     "rule without ID or DocsURL returns empty",
			resolver: DefaultDocsResolver{},
			rule:     &Rule{},
			want:     "",
		},
		{
			name:     "DocsURL still wins when ID is empty",
			resolver: DefaultDocsResolver{},
			rule:     &Rule{DocsURL: "https://example.com/x"},
			want:     "https://example.com/x",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.resolver.DocsURL(tt.rule); got != tt.want {
				t.Errorf("DocsURL() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuleDocsURL_UsesPackageResolver(t *testing.T) {
	prev := SetDocsResolver(DefaultDocsResolver{BaseURL: "https://fake.test/rules/"})
	t.Cleanup(func() { SetDocsResolver(prev) })

	got := RuleDocsURL(&Rule{ID: "R1", Category: "style"})
	if want := "https://fake.test/rules/style/r1/"; got != want {
		t.Errorf("RuleDocsURL() = %q, want %q", got, want)
	}
}

func TestDocsSlug(t *testing.T) {
	for in, want := range map[string]string{
		"MagicNumber":         "magic-number",
		"XMLParser":           "xml-parser",
		"UnusedImport":        "unused-import",
		"untested_public_api": "untested-public-api",
		"android-lint":        "android-lint",
		"Utf8Handling":        "utf8-handling",
		"ID":                  "id",
	} {
		if got := DocsSlug(in); got != want {
			t.Errorf("DocsSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRuleDocsURLFor(t *testing.T) {
	got := RuleDocsURLFor("potential-bugs", "UnsafeCast")
	if want := "https://kaeawc.github.io/krit/rules/potential-bugs/unsafe-cast/"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if RuleDocsURLFor("style", "") != "" {
		t.Error("empty ID must give empty URL")
	}
}

func TestRuleDocsURLByID(t *testing.T) {
	saved := Registry
	t.Cleanup(func() { Registry = saved })
	Registry = []*Rule{{ID: "NewName", Category: "naming", Aliases: []string{"OldName"}}}
	want := "https://kaeawc.github.io/krit/rules/naming/new-name/"
	for _, id := range []string{"NewName", "OldName"} {
		if got := RuleDocsURLByID(id); got != want {
			t.Errorf("RuleDocsURLByID(%q) = %q, want %q", id, got, want)
		}
	}
	if got := RuleDocsURLByID("Nope"); got != "" {
		t.Errorf("unknown ID gave %q", got)
	}
}

func TestRegistryDocsURLsUnique(t *testing.T) {
	seen := map[string]string{}
	for _, r := range Registry {
		u := strings.ToLower(RuleDocsURL(r))
		if prev, ok := seen[u]; ok {
			t.Errorf("%s and %s share docs URL %s", prev, r.ID, u)
		}
		seen[u] = r.ID
	}
}
