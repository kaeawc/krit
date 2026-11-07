package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestLocaleFolder_FlagsDeprecatedLanguageCodes(t *testing.T) {
	resRoot := writeValuesFixture(t, map[string]string{
		"values/strings.xml":    `<resources><string name="x">x</string></resources>`,
		"values-he/strings.xml": `<resources><string name="x">x</string></resources>`,
		"values-id/strings.xml": `<resources><string name="x">x</string></resources>`,
		"values-iw/strings.xml": `<resources><string name="x">x</string></resources>`,
	})
	findings := runLocaleFolderRule(t, resRoot)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d: %+v", len(findings), findings)
	}
	want := "The locale folder \"he\" should be called \"iw\" instead; see the `java.util.Locale` documentation"
	if findings[0].Message != want {
		t.Fatalf("got %q, want %q", findings[0].Message, want)
	}
}

func TestLocaleFolder_AcceptsCorrectFormat(t *testing.T) {
	resRoot := writeValuesFixture(t, map[string]string{
		"values/strings.xml":        `<resources><string name="x">x</string></resources>`,
		"values-en-rUS/strings.xml": `<resources><string name="x">x</string></resources>`,
		"values-en/strings.xml":     `<resources><string name="x">x</string></resources>`,
	})
	findings := runLocaleFolderRule(t, resRoot)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

func TestLocaleFolder_IgnoresNonLocaleQualifiers(t *testing.T) {
	resRoot := writeValuesFixture(t, map[string]string{
		"values/strings.xml":        `<resources><string name="x">x</string></resources>`,
		"values-night/strings.xml":  `<resources><string name="x">x</string></resources>`,
		"values-w600dp/strings.xml": `<resources><string name="x">x</string></resources>`,
	})
	findings := runLocaleFolderRule(t, resRoot)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

func TestUseAlpha2_FlagsKnown3LetterCode(t *testing.T) {
	resRoot := writeValuesFixture(t, map[string]string{
		"values/strings.xml":     `<resources><string name="x">x</string></resources>`,
		"values-eng/strings.xml": `<resources><string name="x">x</string></resources>`,
	})
	findings := runUseAlpha2Rule(t, resRoot)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Message != "For compatibility, should use 2-letter language codes when available; use `en` instead of `eng`" {
		t.Fatalf("expected message to suggest `en`, got: %s", findings[0].Message)
	}
}

func TestUseAlpha2_FlagsJpn(t *testing.T) {
	resRoot := writeValuesFixture(t, map[string]string{
		"values/strings.xml":     `<resources><string name="x">x</string></resources>`,
		"values-jpn/strings.xml": `<resources><string name="x">x</string></resources>`,
	})
	findings := runUseAlpha2Rule(t, resRoot)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "`ja`") {
		t.Fatalf("expected message to suggest `ja`, got: %s", findings[0].Message)
	}
}

func TestUseAlpha2_Accepts2LetterCode(t *testing.T) {
	resRoot := writeValuesFixture(t, map[string]string{
		"values/strings.xml":    `<resources><string name="x">x</string></resources>`,
		"values-en/strings.xml": `<resources><string name="x">x</string></resources>`,
		"values-ja/strings.xml": `<resources><string name="x">x</string></resources>`,
	})
	findings := runUseAlpha2Rule(t, resRoot)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

func TestUseAlpha2_IgnoresUnknown3LetterCode(t *testing.T) {
	resRoot := writeValuesFixture(t, map[string]string{
		"values/strings.xml":     `<resources><string name="x">x</string></resources>`,
		"values-xyz/strings.xml": `<resources><string name="x">x</string></resources>`,
	})
	findings := runUseAlpha2Rule(t, resRoot)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

func TestUseAlpha2_IgnoresMccQualifier(t *testing.T) {
	// `values-mcc310` starts with `mcc` (3 letters) but isn't a locale code
	// and shouldn't be flagged.
	resRoot := writeValuesFixture(t, map[string]string{
		"values/strings.xml":        `<resources><string name="x">x</string></resources>`,
		"values-mcc310/strings.xml": `<resources><string name="x">x</string></resources>`,
	})
	findings := runUseAlpha2Rule(t, resRoot)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

func TestLocaleFolderAndUseAlpha2_HandleMissingResRoot(t *testing.T) {
	// Index seeded with a non-existent file path: os.ReadDir on the derived
	// root returns ENOENT and the rules should silently no-op rather than
	// panicking.
	idx := &android.ResourceIndex{
		StringsLocation: map[string]android.StringLocation{
			"__seed__": {FilePath: "/no/such/path/res/values/strings.xml", Line: 1},
		},
	}
	if findings := dispatchLocaleRule(t, &LocaleFolderRule{}, idx); len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
	if findings := dispatchLocaleRule(t, &UseAlpha2Rule{}, idx); len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d", len(findings))
	}
}

func writeValuesFixture(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	resRoot := filepath.Join(root, "res")
	for rel, content := range files {
		path := filepath.Join(resRoot, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", path, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", path, err)
		}
	}
	return resRoot
}

func runLocaleFolderRule(t *testing.T, resRoot string) []scanner.Finding {
	t.Helper()
	r := &LocaleFolderRule{AndroidRule: AndroidRule{BaseRule: BaseRule{
		RuleName: "LocaleFolder", RuleSetName: "android-lint", Sev: "error",
	}}}
	return dispatchLocaleRule(t, r, indexForResRoot(resRoot))
}

func runUseAlpha2Rule(t *testing.T, resRoot string) []scanner.Finding {
	t.Helper()
	r := &UseAlpha2Rule{AndroidRule: AndroidRule{BaseRule: BaseRule{
		RuleName: "UseAlpha2", RuleSetName: "android-lint", Sev: "warning",
	}}}
	return dispatchLocaleRule(t, r, indexForResRoot(resRoot))
}

type localeRuleImpl interface {
	check(ctx *api.Context)
}

func dispatchLocaleRule(t *testing.T, r localeRuleImpl, idx *android.ResourceIndex) []scanner.Finding {
	t.Helper()
	collector := scanner.NewFindingCollector(0)
	ctx := &api.Context{ResourceIndex: idx, Collector: collector}
	r.check(ctx)
	return api.ContextFindings(ctx)
}

func indexForResRoot(resRoot string) *android.ResourceIndex {
	return &android.ResourceIndex{
		StringsLocation: map[string]android.StringLocation{
			"__seed__": {
				FilePath: filepath.Join(resRoot, "values", "strings.xml"),
				Line:     1,
			},
		},
	}
}

func TestInvalidResourceFolder_FlagsUnknownTypeDirectory(t *testing.T) {
	resRoot := writeValuesFixture(t, map[string]string{
		"values/strings.xml":    `<resources><string name="x">x</string></resources>`,
		"lyout/main.xml":        `<LinearLayout/>`,
		"interpolator/ease.xml": `<accelerateInterpolator/>`,
		"drawable-hdpi/a.xml":   `<vector/>`,
	})
	r := &InvalidResourceFolderResourceRule{AndroidRule: AndroidRule{BaseRule: BaseRule{
		RuleName: "InvalidResourceFolderResource", RuleSetName: "android-lint", Sev: "error",
	}}}
	findings := dispatchLocaleRule(t, r, indexForResRoot(resRoot))
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "lyout") {
		t.Fatalf("expected only the lyout folder to be reported, got %+v", findings)
	}
}
