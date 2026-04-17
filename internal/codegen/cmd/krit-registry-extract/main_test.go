package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestRun_EmitsRegistryFile parses a small synthetic rules directory
// and verifies the generator:
//   - extracts every v2.Register(...) call across init() bodies,
//   - preserves embedded struct literal text verbatim,
//   - handles alias registrations (a rule registered under two IDs),
//   - rewrites source files to drop the extracted statements, leaving
//     other init() content (e.g. map initialization) intact.
func TestRun_EmitsRegistryFile(t *testing.T) {
	tmp := t.TempDir()
	rulesDir := filepath.Join(tmp, "rules")
	if err := os.MkdirAll(rulesDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Synthetic rule source mimicking the real shapes:
	//   - WrapAsV2 with struct literal
	//   - AdaptFlatDispatch with a var-declared struct + adapter options
	//   - alias (rule registered under TWO IDs in the same init)
	//   - a non-registration init (map setup) that must be preserved.
	aFile := `package rules

import (
	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

var lookup = map[string]string{
	"foo": "bar",
}

func init() {
	lookup["baz"] = "qux"
}

func init() {
	v2.Register(WrapAsV2(&FooRule{
		BaseRule: BaseRule{RuleName: "Foo", RuleSetName: "demo", Sev: "warning"},
	}))

	{
		r := &BarRule{AndroidRule: AndroidRule{
			BaseRule: BaseRule{RuleName: "Bar", RuleSetName: "demo", Sev: "info"},
			Brief:    "Barring things",
		}}
		v2.Register(v2.AdaptFlatDispatch(r.RuleName, r.RuleSetName, r.Description(), v2.Severity(r.Sev), r.NodeTypes(), r.CheckFlatNode, v2.AdaptWithConfidence(0.9)))
	}

	// Alias: register BazRule under two IDs.
	v2.Register(WrapAsV2(&BazRule{BaseRule: BaseRule{RuleName: "Baz", RuleSetName: "demo", Sev: "warning"}}))
	v2.Register(WrapAsV2(&BazRule{BaseRule: BaseRule{RuleName: "BazAlias", RuleSetName: "demo", Sev: "warning"}}))
}
`
	if err := os.WriteFile(filepath.Join(rulesDir, "a.go"), []byte(aFile), 0o644); err != nil {
		t.Fatalf("write a.go: %v", err)
	}

	outPath := filepath.Join(tmp, "zz_registry_gen.go")
	var stdout, stderr bytes.Buffer
	err := run([]string{
		"-rules", rulesDir,
		"-out", outPath,
		"-rewrite",
	}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v\nstderr: %s", err, stderr.String())
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out: %v", err)
	}
	out := string(data)

	// Must contain all four registrations verbatim.
	for _, want := range []string{
		`RuleName: "Foo"`,
		`RuleName: "Bar"`,
		`RuleName: "Baz"`,
		`RuleName: "BazAlias"`,
		`v2.AdaptFlatDispatch`,
		`v2.AdaptWithConfidence(0.9)`,
		`func init()`,
		`registerAllRules`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("registry missing %q\n---\n%s", want, out)
		}
	}

	// Source file must have had its rule-registration init stripped, but
	// the map-setup init preserved.
	rewritten, err := os.ReadFile(filepath.Join(rulesDir, "a.go"))
	if err != nil {
		t.Fatalf("read rewritten a.go: %v", err)
	}
	rs := string(rewritten)
	if !strings.Contains(rs, `lookup["baz"] = "qux"`) {
		t.Errorf("map-setup init body was deleted:\n%s", rs)
	}
	if strings.Contains(rs, "v2.Register(") {
		t.Errorf("rewritten source still contains v2.Register:\n%s", rs)
	}
	if strings.Contains(rs, `"github.com/kaeawc/krit/internal/rules/v2"`) {
		t.Errorf("rewritten source still imports v2 (unused):\n%s", rs)
	}
	// Only one init() should remain (the map-setup).
	if got := strings.Count(rs, "\nfunc init()"); got != 1 {
		t.Errorf("expected 1 remaining init(), got %d:\n%s", got, rs)
	}
}

// helper: satisfy the imports.
var _ io.Reader = (*bytes.Buffer)(nil)
