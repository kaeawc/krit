package rules

import "testing"

// importAliases takes `package.R as Alias` entries; Lint's import-aliases is one comma-separated string, which is
// split the same way.
func TestResourceImportAliasesParsing(t *testing.T) {
	r := &ResourceImportAliasRule{ImportAliases: []string{"slack.l10n.R as L10nR, slack.other.R as OtherR", " a.b.R as AbR ", "bad entry"}}
	got := r.aliases()
	want := map[string]string{"slack.l10n.R": "L10nR", "slack.other.R": "OtherR", "a.b.R": "AbR"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %q, want %q", k, got[k], v)
		}
	}
}
