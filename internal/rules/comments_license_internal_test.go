package rules

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func parseLicenseFixture(t *testing.T, code string) *scanner.File {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.kt")
	if err := os.WriteFile(path, []byte(code), 0644); err != nil {
		t.Fatal(err)
	}
	f, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func runLicenseRule(t *testing.T, r *AbsentOrWrongFileLicenseRule, file *scanner.File) []scanner.Finding {
	t.Helper()
	collector := scanner.NewFindingCollector(0)
	rule := &api.Rule{
		ID:             r.RuleName,
		Category:       r.RuleSetName,
		Description:    r.Desc,
		Sev:            api.Severity(r.Sev),
		Implementation: r,
	}
	ctx := &api.Context{File: file, Rule: rule, Collector: collector}
	r.check(ctx)
	return api.ContextFindings(ctx)
}

// Regression: when LicenseTemplate is a regex pattern, the autofix must not
// dump the raw pattern into the source file. Earlier versions of this rule
// wrapped the regex in "/* ... */" as a fake "license header"; the fix is to
// emit the finding without an autofix in regex mode.
func TestAbsentOrWrongFileLicense_RegexModeOmitsAutofix_NoComment(t *testing.T) {
	file := parseLicenseFixture(t, `package test

fun foo() {}
`)
	r := &AbsentOrWrongFileLicenseRule{
		BaseRule:        BaseRule{RuleName: "AbsentOrWrongFileLicense", RuleSetName: "comments", Sev: "warning"},
		LicenseTemplate: `(?i)copyright\s+\d{4}.*`,
		IsRegex:         true,
	}
	findings := runLicenseRule(t, r, file)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix != nil {
		t.Fatalf("regex-mode rule must not attach an autofix; got Fix with replacement %q",
			findings[0].Fix.Replacement)
	}
}

func TestAbsentOrWrongFileLicense_RegexModeOmitsAutofix_WrongComment(t *testing.T) {
	file := parseLicenseFixture(t, `/* some unrelated header */
package test

fun foo() {}
`)
	r := &AbsentOrWrongFileLicenseRule{
		BaseRule:        BaseRule{RuleName: "AbsentOrWrongFileLicense", RuleSetName: "comments", Sev: "warning"},
		LicenseTemplate: `(?i)copyright\s+\d{4}.*`,
		IsRegex:         true,
	}
	findings := runLicenseRule(t, r, file)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Fix != nil {
		t.Fatalf("regex-mode rule must not attach an autofix; got Fix with replacement %q",
			findings[0].Fix.Replacement)
	}
}

// Literal-mode autofix still works — sanity check that the regex guard didn't
// over-correct and disable the autofix for normal users.
func TestAbsentOrWrongFileLicense_LiteralModeKeepsAutofix(t *testing.T) {
	file := parseLicenseFixture(t, `package test

fun foo() {}
`)
	r := &AbsentOrWrongFileLicenseRule{
		BaseRule:        BaseRule{RuleName: "AbsentOrWrongFileLicense", RuleSetName: "comments", Sev: "warning"},
		LicenseTemplate: "Copyright 2026 Acme",
	}
	findings := runLicenseRule(t, r, file)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	fix := findings[0].Fix
	if fix == nil {
		t.Fatal("literal-mode rule must attach an autofix")
	}
	if !strings.Contains(fix.Replacement, "Copyright 2026 Acme") {
		t.Fatalf("expected literal template in replacement, got %q", fix.Replacement)
	}
}
