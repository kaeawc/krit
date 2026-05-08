package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

func TestMissingQuantityResource(t *testing.T) {
	r := findResourceRule(t, "MissingQuantityResource")

	t.Run("missing other triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one": "%d apple",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Rule != "MissingQuantityResource" {
			t.Fatalf("expected rule MissingQuantityResource, got %s", findings[0].Rule)
		}
	})

	t.Run("has other is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one":   "%d apple",
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty plurals is clean", func(t *testing.T) {
		idx := emptyIndex()
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// UnusedQuantityResource
// ---------------------------------------------------------------------------

func TestUnusedQuantityResource(t *testing.T) {
	r := findResourceRule(t, "UnusedQuantityResource")

	t.Run("unused zero triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"zero":  "no apples",
			"one":   "%d apple",
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("unused two few many triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "%d item",
			"two":   "%d items",
			"few":   "%d items",
			"many":  "%d items",
			"other": "%d items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %d", len(findings))
		}
	})

	t.Run("only one and other is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one":   "%d apple",
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ImpliedQuantityResource
// ---------------------------------------------------------------------------

func TestImpliedQuantityResource(t *testing.T) {
	r := findResourceRule(t, "ImpliedQuantityResource")

	t.Run("one without %d triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one":   "One apple",
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("one with %d is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one":   "%d apple",
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no one quantity is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"other": "%d apples",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringFormatTrivialResource
// ---------------------------------------------------------------------------

func TestStringFormatTrivialResource(t *testing.T) {
	r := findResourceRule(t, "StringFormatTrivialResource")

	t.Run("whole value %s triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greeting"] = "%s"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("whole value %s uses parsed source location", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		valuesDir := filepath.Join(resDir, "values")
		if err := os.MkdirAll(valuesDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", valuesDir, err)
		}
		stringsPath := filepath.Join(valuesDir, "strings.xml")
		if err := os.WriteFile(stringsPath, []byte("<resources>\n    <string name=\"greeting\">%s</string>\n</resources>\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", stringsPath, err)
		}
		idx, err := android.ScanResourceDir(resDir)
		if err != nil {
			t.Fatalf("ScanResourceDir(%s): %v", resDir, err)
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].File != stringsPath {
			t.Fatalf("expected finding file %q, got %q", stringsPath, findings[0].File)
		}
		if findings[0].Line != 2 {
			t.Fatalf("expected finding line 2, got %d", findings[0].Line)
		}
	})

	t.Run("relative res dir resolves parsed source location to absolute path", func(t *testing.T) {
		root := t.TempDir()
		runnerDir := filepath.Join(root, "runner")
		resDir := filepath.Join(root, "project", "src", "main", "res")
		valuesDir := filepath.Join(resDir, "values")
		if err := os.MkdirAll(runnerDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", runnerDir, err)
		}
		if err := os.MkdirAll(valuesDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", valuesDir, err)
		}
		stringsPath := filepath.Join(valuesDir, "strings.xml")
		if err := os.WriteFile(stringsPath, []byte("<resources>\n    <string name=\"greeting\">%s</string>\n</resources>\n"), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", stringsPath, err)
		}
		relResDir, err := filepath.Rel(runnerDir, resDir)
		if err != nil {
			t.Fatalf("Rel(%s, %s): %v", runnerDir, resDir, err)
		}
		oldWD, err := os.Getwd()
		if err != nil {
			t.Fatalf("Getwd: %v", err)
		}
		if err := os.Chdir(runnerDir); err != nil {
			t.Fatalf("Chdir(%s): %v", runnerDir, err)
		}
		t.Cleanup(func() {
			if err := os.Chdir(oldWD); err != nil {
				t.Fatalf("restore cwd: %v", err)
			}
		})
		expectedStringsPath, err := filepath.Abs(filepath.Join(relResDir, "values", "strings.xml"))
		if err != nil {
			t.Fatalf("Abs(%s): %v", stringsPath, err)
		}

		idx, err := android.ScanResourceDir(relResDir)
		if err != nil {
			t.Fatalf("ScanResourceDir(%s): %v", relResDir, err)
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].File != expectedStringsPath {
			t.Fatalf("expected absolute finding file %q, got %q", expectedStringsPath, findings[0].File)
		}
	})

	t.Run("multiple specifiers is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greeting"] = "Hello, %s! You have %d messages."
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("single %s inside localized text is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greeting"] = "Hello, %s!"
		idx.Strings["scheduled"] = "Today at %s"
		idx.Strings["count"] = "%s items"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("whole value positional string specifier triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["value"] = "%1$s"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("single %d is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["count"] = "You have %d items"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no specifiers is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["hello"] = "Hello World"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("escaped %% is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["percent"] = "100%% complete"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringNotLocalizableResource
// ---------------------------------------------------------------------------

func TestStringNotLocalizableResource(t *testing.T) {
	r := findResourceRule(t, "StringNotLocalizableResource")

	t.Run("URL triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["website"] = "https://example.com"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("http URL triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["api"] = "http://api.example.com/v1"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("email triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["support_email"] = "support@example.com"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("all uppercase triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["api_key_label"] = "API_KEY"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("normal string is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greeting"] = "Hello World"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty string is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["empty"] = ""
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("mixed case is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["title"] = "My Application"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("single char uppercase is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["single"] = "A"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// GoogleApiKeyInResources
// ---------------------------------------------------------------------------

func TestWrongCaseResource(t *testing.T) {
	r := findResourceRule(t, "WrongCaseResource")

	t.Run("Textview triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "Textview",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "TextView") {
			t.Fatalf("expected suggestion for TextView, got %q", findings[0].Message)
		}
	})

	t.Run("linearlayout triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "linearlayout",
			Line:       1,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "LinearLayout") {
			t.Fatalf("expected suggestion for LinearLayout, got %q", findings[0].Message)
		}
	})

	t.Run("correct casing is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "TextView",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("fully qualified name is skipped", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "com.example.CustomView",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringTrailingWhitespace
// ---------------------------------------------------------------------------

func TestStringTrailingWhitespace(t *testing.T) {
	r := findResourceRule(t, "StringTrailingWhitespace")

	t.Run("trailing space triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["label"] = "Label"
		idx.StringsTrailingWS = map[string]bool{"label": true}
		idx.StringsLocation["label"] = android.StringLocation{
			FilePath: "app/src/main/res/values/strings.xml",
			Line:     5,
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "trailing whitespace") {
			t.Fatalf("expected message about trailing whitespace, got %q", findings[0].Message)
		}
		if findings[0].File != "app/src/main/res/values/strings.xml" || findings[0].Line != 5 {
			t.Fatalf("unexpected location %s:%d", findings[0].File, findings[0].Line)
		}
	})

	t.Run("translatable=false is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["label"] = "Label"
		idx.StringsTrailingWS = map[string]bool{"label": true}
		idx.StringsNonTranslate = map[string]bool{"label": true}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no trailing whitespace is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["label"] = "Label"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("end-to-end via XML scan flags trailing space and respects translatable=false", func(t *testing.T) {
		dir := t.TempDir()
		valuesDir := filepath.Join(dir, "res", "values")
		if err := os.MkdirAll(valuesDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		content := `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="label">Label </string>
    <string name="label_with_space" translatable="false">Label </string>
    <string name="clean">Label</string>
</resources>
`
		if err := os.WriteFile(filepath.Join(valuesDir, "strings.xml"), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		idx, err := android.ScanResourceDir(filepath.Join(dir, "res"))
		if err != nil {
			t.Fatalf("ScanResourceDir: %v", err)
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "`label`") {
			t.Fatalf("expected finding for `label`, got %q", findings[0].Message)
		}
	})
}

// ---------------------------------------------------------------------------
// StringResourceMissingPositional
// ---------------------------------------------------------------------------

func TestStringResourceMissingPositional(t *testing.T) {
	r := findResourceRule(t, "StringResourceMissingPositional")

	t.Run("two bare %s triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greet"] = "%s meets %s"
		idx.StringsLocation["greet"] = android.StringLocation{
			FilePath: "app/src/main/res/values/strings.xml",
			Line:     7,
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "positional") {
			t.Fatalf("expected message about positional, got %q", findings[0].Message)
		}
		if findings[0].File != "app/src/main/res/values/strings.xml" || findings[0].Line != 7 {
			t.Fatalf("unexpected location %s:%d", findings[0].File, findings[0].Line)
		}
	})

	t.Run("mixed specifiers trigger", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["mix"] = "Hello %s, you are %d"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("positional form is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greet"] = "%1$s meets %2$s"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("single specifier is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["one"] = "Hello %s"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("translatable=false is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greet"] = "%s meets %s"
		idx.StringsNonTranslate = map[string]bool{"greet": true}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("formatted=false is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greet"] = "%s meets %s"
		idx.StringsNonFormatted = map[string]bool{"greet": true}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("escaped percent is ignored", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["pct"] = "100%% sure %s"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("end-to-end via XML scan", func(t *testing.T) {
		dir := t.TempDir()
		valuesDir := filepath.Join(dir, "res", "values")
		if err := os.MkdirAll(valuesDir, 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}
		content := `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="bad">%s meets %s</string>
    <string name="ok">%1$s meets %2$s</string>
    <string name="single">Hello %s</string>
    <string name="bad_nontranslatable" translatable="false">%s meets %s</string>
</resources>
`
		if err := os.WriteFile(filepath.Join(valuesDir, "strings.xml"), []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
		idx, err := android.ScanResourceDir(filepath.Join(dir, "res"))
		if err != nil {
			t.Fatalf("ScanResourceDir: %v", err)
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "`bad`") {
			t.Fatalf("expected finding for `bad`, got %q", findings[0].Message)
		}
	})
}

// ---------------------------------------------------------------------------
// ExtraTextResource
// ---------------------------------------------------------------------------

func TestExtraTextResource(t *testing.T) {
	r := findResourceRule(t, "ExtraTextResource")

	t.Run("stray text in values file triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.ExtraTexts = []android.ExtraTextEntry{
			{FilePath: "res/values/strings.xml", Line: 3, Text: "some stray text"},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "Extraneous text") {
			t.Fatalf("expected message about extraneous text, got %q", findings[0].Message)
		}
	})

	t.Run("no extra text is clean", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// IllegalResourceRefResource
// ---------------------------------------------------------------------------

func TestWrongRegionResource(t *testing.T) {
	r := findResourceRule(t, "WrongRegionResource")

	t.Run("suspicious en-rBR triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/values-en-rBR/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "en") {
			t.Errorf("expected message to mention 'en', got %q", findings[0].Message)
		}
	})

	t.Run("valid en-rUS is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/values-en-rUS/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no qualifier is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// UnusedAttributeResource
// ---------------------------------------------------------------------------

func TestStringFormatInvalidResource(t *testing.T) {
	r := findResourceRule(t, "StringFormatInvalidResource")

	t.Run("bare percent at end triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greeting"] = "Hello %"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "bare") {
			t.Fatalf("expected message about bare %%, got: %s", findings[0].Message)
		}
	})

	t.Run("invalid conversion char triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["bad"] = "Value is %z"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "invalid conversion") {
			t.Fatalf("expected message about invalid conversion, got: %s", findings[0].Message)
		}
	})

	t.Run("valid format specifiers are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["ok1"] = "Hello %s, you have %d items"
		idx.Strings["ok2"] = "Price: %1$f"
		idx.Strings["ok3"] = "100%% complete"
		idx.Strings["ok4"] = "line%n"
		idx.Strings["ok5"] = "hex: %x"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no format specifiers is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["plain"] = "Hello world"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean for StringFormatInvalid", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringFormatCountResource
// ---------------------------------------------------------------------------

func TestStringFormatCountResource(t *testing.T) {
	r := findResourceRule(t, "StringFormatCountResource")

	t.Run("gap in positional args triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["msg"] = "%1$s and %3$s"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "gap") {
			t.Fatalf("expected message about gap, got: %s", findings[0].Message)
		}
		if !strings.Contains(findings[0].Message, "%2$") {
			t.Fatalf("expected message to mention %%2$, got: %s", findings[0].Message)
		}
	})

	t.Run("consecutive positional args are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["ok"] = "%1$s and %2$s"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-positional args are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["ok"] = "%s and %d"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("single positional arg is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["ok"] = "%1$s"
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean for StringFormatCount", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringFormatMatchesResource
// ---------------------------------------------------------------------------

func TestStringFormatMatchesResource(t *testing.T) {
	r := findResourceRule(t, "StringFormatMatchesResource")

	t.Run("type mismatch across quantities triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "%d item",
			"other": "%s items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "mismatch") {
			t.Fatalf("expected message about mismatch, got: %s", findings[0].Message)
		}
	})

	t.Run("different arg counts triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "%d item",
			"other": "%d of %d items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "format args") {
			t.Fatalf("expected message about format arg count, got: %s", findings[0].Message)
		}
	})

	t.Run("consistent types are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "%d item",
			"other": "%d items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("single quantity is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"other": "%d items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no format specifiers are clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "item",
			"other": "items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("empty index is clean for StringFormatMatches", func(t *testing.T) {
		findings := runResourceRule(r, emptyIndex())
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// NotSiblingResource
// ---------------------------------------------------------------------------

func TestLocaleConfigStale(t *testing.T) {
	r := findResourceRule(t, "LocaleConfigStale")

	write := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}

	scan := func(t *testing.T, root string) *android.ResourceIndex {
		t.Helper()
		idx, err := android.ScanResourceDir(root)
		if err != nil {
			t.Fatalf("ScanResourceDir(%s): %v", root, err)
		}
		return idx
	}

	t.Run("extra values locale triggers", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"), "<resources><string name=\"app_name\">App</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"), "<resources><string name=\"app_name\">Appli</string></resources>\n")
		write(t, filepath.Join(resDir, "values-de", "strings.xml"), "<resources><string name=\"app_name\">App DE</string></resources>\n")
		write(t, filepath.Join(resDir, "values-es", "strings.xml"), "<resources><string name=\"app_name\">App ES</string></resources>\n")
		write(t, filepath.Join(resDir, "xml", "locales_config.xml"), `<?xml version="1.0" encoding="utf-8"?>
<locale-config xmlns:android="http://schemas.android.com/apk/res/android">
    <locale android:name="en" />
    <locale android:name="fr" />
    <locale android:name="de" />
</locale-config>
`)

		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "extra values locales: es") {
			t.Fatalf("expected finding to mention extra locale es, got %q", findings[0].Message)
		}
	})

	t.Run("default plus matching variants is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"), "<resources><string name=\"app_name\">App</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"), "<resources><string name=\"app_name\">Appli</string></resources>\n")
		write(t, filepath.Join(resDir, "values-de", "strings.xml"), "<resources><string name=\"app_name\">App DE</string></resources>\n")
		write(t, filepath.Join(resDir, "xml", "locales_config.xml"), `<?xml version="1.0" encoding="utf-8"?>
<locale-config xmlns:android="http://schemas.android.com/apk/res/android">
    <locale android:name="en" />
    <locale android:name="fr" />
    <locale android:name="de" />
</locale-config>
`)

		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("missing locales_config is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"), "<resources><string name=\"app_name\">App</string></resources>\n")

		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// LayoutClickableWithoutMinSize
// ---------------------------------------------------------------------------

func TestStringNotSelectable(t *testing.T) {
	r := findResourceRule(t, "StringNotSelectable")

	t.Run("non-selectable text with URL triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textIsSelectable": "false",
				"android:text":             "Visit https://example.com for info",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("non-selectable text without URLs is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type: "TextView",
			Line: 5,
			Attributes: map[string]string{
				"android:textIsSelectable": "false",
				"android:text":             "Hello World",
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringRepeatedInContentDescription
// ---------------------------------------------------------------------------

func TestStringRepeatedInContentDescription(t *testing.T) {
	r := findResourceRule(t, "StringRepeatedInContentDescription")

	t.Run("contentDescription duplicates sibling text triggers", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       3,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:               "ImageView",
					ContentDescription: "Settings",
					Line:               5,
					Attributes:         map[string]string{},
				},
				{
					Type:       "TextView",
					Text:       "Settings",
					Line:       6,
					Attributes: map[string]string{},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("contentDescription differs from sibling text is clean", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       3,
			Attributes: map[string]string{},
			Children: []*android.View{
				{
					Type:               "ImageView",
					ContentDescription: "Open settings",
					Line:               5,
					Attributes:         map[string]string{},
				},
				{
					Type:       "TextView",
					Text:       "Settings",
					Line:       6,
					Attributes: map[string]string{},
				},
			},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringSpanInContentDescription
// ---------------------------------------------------------------------------

func TestStringSpanInContentDescription(t *testing.T) {
	r := findResourceRule(t, "StringSpanInContentDescription")

	t.Run("string with HTML used in contentDescription triggers", func(t *testing.T) {
		layout := &android.Layout{
			Name:     "main",
			FilePath: "res/layout/main.xml",
			RootView: &android.View{
				Type:               "ImageView",
				ContentDescription: "@string/img_desc",
				Line:               5,
				Attributes:         map[string]string{},
			},
		}
		idx := &android.ResourceIndex{
			Layouts: map[string]*android.Layout{"main": layout},
			LayoutConfigs: map[string]map[string]*android.Layout{
				"main": {"": layout},
			},
			Strings: map[string]string{
				"img_desc": "<b>Bold</b> image description",
			},
			StringsLocation: map[string]android.StringLocation{
				"img_desc": {FilePath: "res/values/strings.xml", Line: 3},
			},
			Colors:       make(map[string]string),
			Dimensions:   make(map[string]string),
			Styles:       make(map[string]*android.Style),
			StringArrays: make(map[string][]string),
			Plurals:      make(map[string]map[string]string),
			Integers:     make(map[string]string),
			Booleans:     make(map[string]string),
			IDs:          make(map[string]bool),
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("plain string in contentDescription is clean", func(t *testing.T) {
		layout := &android.Layout{
			Name:     "main",
			FilePath: "res/layout/main.xml",
			RootView: &android.View{
				Type:               "ImageView",
				ContentDescription: "@string/img_desc",
				Line:               5,
				Attributes:         map[string]string{},
			},
		}
		idx := &android.ResourceIndex{
			Layouts: map[string]*android.Layout{"main": layout},
			LayoutConfigs: map[string]map[string]*android.Layout{
				"main": {"": layout},
			},
			Strings: map[string]string{
				"img_desc": "Profile image",
			},
			StringsLocation: map[string]android.StringLocation{
				"img_desc": {FilePath: "res/values/strings.xml", Line: 3},
			},
			Colors:       make(map[string]string),
			Dimensions:   make(map[string]string),
			Styles:       make(map[string]*android.Style),
			StringArrays: make(map[string][]string),
			Plurals:      make(map[string]map[string]string),
			Integers:     make(map[string]string),
			Booleans:     make(map[string]string),
			IDs:          make(map[string]bool),
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// PluralsMissingZero
// ---------------------------------------------------------------------------

func TestPluralsMissingZero(t *testing.T) {
	r := findResourceRule(t, "PluralsMissingZero")

	write := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}

	scan := func(t *testing.T, root string) *android.ResourceIndex {
		t.Helper()
		idx, err := android.ScanResourceDir(root)
		if err != nil {
			t.Fatalf("ScanResourceDir(%s): %v", root, err)
		}
		return idx
	}

	t.Run("ar plurals without zero triggers", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"app_name\">App</string></resources>\n")
		write(t, filepath.Join(resDir, "values-ar", "plurals.xml"), `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <plurals name="item_count">
        <item quantity="one">عنصر واحد</item>
        <item quantity="other">%d عناصر</item>
    </plurals>
</resources>
`)
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "item_count") || !strings.Contains(findings[0].Message, "values-ar/") {
			t.Fatalf("unexpected message: %q", findings[0].Message)
		}
	})

	t.Run("ar plurals with zero is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"app_name\">App</string></resources>\n")
		write(t, filepath.Join(resDir, "values-ar", "plurals.xml"), `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <plurals name="item_count">
        <item quantity="zero">لا يوجد عناصر</item>
        <item quantity="one">عنصر واحد</item>
        <item quantity="two">عنصران</item>
        <item quantity="few">%d عناصر</item>
        <item quantity="many">%d عنصرًا</item>
        <item quantity="other">%d عنصر</item>
    </plurals>
</resources>
`)
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d:\n  %v", len(findings), findings)
		}
	})

	t.Run("default values plurals is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"app_name\">App</string></resources>\n")
		write(t, filepath.Join(resDir, "values", "plurals.xml"), `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <plurals name="item_count">
        <item quantity="one">%d item</item>
        <item quantity="other">%d items</item>
    </plurals>
</resources>
`)
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("non-zero-form locale is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"app_name\">App</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "plurals.xml"), `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <plurals name="item_count">
        <item quantity="one">%d élément</item>
        <item quantity="other">%d éléments</item>
    </plurals>
</resources>
`)
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("ru plurals without zero triggers", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"app_name\">App</string></resources>\n")
		write(t, filepath.Join(resDir, "values-ru", "plurals.xml"), `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <plurals name="item_count">
        <item quantity="one">%d элемент</item>
        <item quantity="few">%d элемента</item>
        <item quantity="many">%d элементов</item>
        <item quantity="other">%d элемента</item>
    </plurals>
</resources>
`)
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// TranslatableMarkupMismatch
// ---------------------------------------------------------------------------

func TestTranslatableMarkupMismatch(t *testing.T) {
	r := findResourceRule(t, "TranslatableMarkupMismatch")

	write := func(t *testing.T, path, content string) {
		t.Helper()
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", filepath.Dir(path), err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", path, err)
		}
	}

	scan := func(t *testing.T, root string) *android.ResourceIndex {
		t.Helper()
		idx, err := android.ScanResourceDir(root)
		if err != nil {
			t.Fatalf("ScanResourceDir(%s): %v", root, err)
		}
		return idx
	}

	t.Run("html default vs markdown variant triggers", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"emphasis\">This is &lt;b&gt;bold&lt;/b&gt;</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"),
			"<resources><string name=\"emphasis\">Ceci est **gras**</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "emphasis") || !strings.Contains(findings[0].Message, "values-fr/") {
			t.Fatalf("unexpected message: %q", findings[0].Message)
		}
	})

	t.Run("html default and html variant is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"emphasis\">This is &lt;b&gt;bold&lt;/b&gt;</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"),
			"<resources><string name=\"emphasis\">Ceci est &lt;b&gt;gras&lt;/b&gt;</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("plain default vs markdown variant triggers", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"hello\">Hello world</string></resources>\n")
		write(t, filepath.Join(resDir, "values-de", "strings.xml"),
			"<resources><string name=\"hello\">Hallo **Welt**</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("html default vs plain variant triggers", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"emphasis\">This is &lt;b&gt;bold&lt;/b&gt;</string></resources>\n")
		write(t, filepath.Join(resDir, "values-es", "strings.xml"),
			"<resources><string name=\"emphasis\">Esto es negrita</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("translatable false is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"sku\" translatable=\"false\">**X**</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"),
			"<resources><string name=\"sku\" translatable=\"false\">SKU</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("plain everywhere is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"hello\">Hello</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"),
			"<resources><string name=\"hello\">Bonjour</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("variant only is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"),
			"<resources><string name=\"hello\">Bonjour **monde**</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}
