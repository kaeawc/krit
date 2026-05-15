package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// TestSARIFTaxa exercises the security-taxonomy wiring end-to-end:
// when a security rule fires, the SARIF output carries taxa, the
// driver advertises supportedTaxonomies, and the result's properties
// expose the CWE/OWASP/CERT IDs that GitHub code-scanning expects.
func TestSARIFTaxa(t *testing.T) {
	// Pick any registered security rule with a CWE mapping; the global
	// init() already attached taxonomies to the registry.
	var ruleID, ruleSet string
	for _, r := range api.Registry {
		if r.Category == "security" && r.Security != nil && len(r.Security.CWE) > 0 {
			ruleID = r.ID
			ruleSet = r.Category
			break
		}
	}
	if ruleID == "" {
		t.Fatal("expected at least one security rule with CWE taxonomy in registry")
	}

	cc := scanner.NewFindingCollector(1)
	cc.Append(scanner.Finding{
		File:       "app/Foo.kt",
		Line:       10,
		Col:        4,
		Severity:   "error",
		RuleSet:    ruleSet,
		Rule:       ruleID,
		Message:    "boom",
		Confidence: 0.9,
	})
	cols := cc.Columns()

	var buf bytes.Buffer
	if err := FormatSARIFColumns(&buf, cols, "test"); err != nil {
		t.Fatalf("FormatSARIFColumns: %v", err)
	}

	var log map[string]any
	if err := json.Unmarshal(buf.Bytes(), &log); err != nil {
		t.Fatalf("SARIF output is not valid JSON: %v\n%s", err, buf.String())
	}

	runs, _ := log["runs"].([]any)
	if len(runs) != 1 {
		t.Fatalf("expected 1 run, got %d", len(runs))
	}
	run := runs[0].(map[string]any)

	taxonomies, _ := run["taxonomies"].([]any)
	if len(taxonomies) == 0 {
		t.Fatalf("expected SARIF run to carry taxonomies block; got: %s", buf.String())
	}
	hasCWE := false
	for _, tx := range taxonomies {
		m := tx.(map[string]any)
		if m["name"] == "CWE" {
			taxa, _ := m["taxa"].([]any)
			if len(taxa) > 0 {
				hasCWE = true
			}
		}
	}
	if !hasCWE {
		t.Fatalf("expected CWE taxonomy with at least one taxon: %s", buf.String())
	}

	driver := run["tool"].(map[string]any)["driver"].(map[string]any)
	supported, _ := driver["supportedTaxonomies"].([]any)
	if len(supported) == 0 {
		t.Fatalf("expected driver.supportedTaxonomies to be populated: %s", buf.String())
	}

	results := run["results"].([]any)
	res := results[0].(map[string]any)
	props, ok := res["properties"].(map[string]any)
	if !ok {
		t.Fatalf("expected result.properties: %s", buf.String())
	}
	cwe, _ := props["cwe"].([]any)
	if len(cwe) == 0 {
		t.Fatalf("expected result.properties.cwe: %s", buf.String())
	}
	if !strings.HasPrefix(cwe[0].(string), "CWE-") {
		t.Fatalf("expected CWE-prefixed ID, got %v", cwe[0])
	}
}
