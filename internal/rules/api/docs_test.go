package api

import "testing"

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
			rule:     &Rule{ID: "MagicNumber"},
			want:     "https://krit.dev/rules/MagicNumber",
		},
		{
			name:     "custom base URL",
			resolver: DefaultDocsResolver{BaseURL: "https://docs.example.com/r/"},
			rule:     &Rule{ID: "MagicNumber"},
			want:     "https://docs.example.com/r/MagicNumber",
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

	got := RuleDocsURL(&Rule{ID: "R1"})
	if want := "https://fake.test/rules/R1"; got != want {
		t.Errorf("RuleDocsURL() = %q, want %q", got, want)
	}
}
