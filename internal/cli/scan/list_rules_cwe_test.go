package scan

import (
	"bytes"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
)

// TestCWEFilter verifies that --list-rules --cwe filters the output to
// rules whose Security taxonomy lists the requested ID.
func TestCWEFilter(t *testing.T) {
	matcher := api.TaxonomyMatcher{IDs: []string{"CWE-89"}}
	hasMatch := false
	for _, r := range api.Registry {
		if matcher.Matches(r.Security) {
			hasMatch = true
			break
		}
	}
	if !hasMatch {
		t.Skip("no CWE-89 rule registered; skipping filter test")
	}

	var buf bytes.Buffer
	printListRules(&buf, false, "", "CWE-89", nil, nil)
	out := buf.String()

	if !strings.Contains(out, "SqlInjectionRawQuery") {
		t.Fatalf("expected SqlInjectionRawQuery in --cwe CWE-89 output, got:\n%s", out)
	}
	if strings.Contains(out, "TrailingWhitespace") {
		t.Fatalf("non-security rule leaked into CWE-89 filter:\n%s", out)
	}
}
