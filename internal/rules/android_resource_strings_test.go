package rules_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// locatePlurals records a position for every plural so rules can report it.
func locatePlurals(idx *android.ResourceIndex) {
	idx.PluralsLocation = map[string]android.StringLocation{}
	for name := range idx.Plurals {
		idx.PluralsLocation[name] = android.StringLocation{FilePath: "res/values/plurals.xml", Line: 3}
	}
}

func TestMissingQuantityResource(t *testing.T) {
	r := findResourceRule(t, "MissingQuantityResource")

	t.Run("missing other triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one": "%d apple",
		}
		locatePlurals(idx)
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
	run := func(defs ...android.PluralDefinition) []scanner.Finding {
		idx := emptyIndex()
		idx.PluralDefinitions = defs
		return runResourceRule(r, idx)
	}
	def := func(folder string, qs ...string) android.PluralDefinition {
		m := map[string]string{}
		for _, q := range qs {
			m[q] = "x"
		}
		return android.PluralDefinition{Name: "items", FilePath: "res/" + folder + "/strings.xml", Line: 3, Quantities: m}
	}

	t.Run("default folder is not judged", func(t *testing.T) {
		if got := run(def("values", "zero", "one", "two", "other"), def("values-night", "zero", "other")); len(got) != 0 {
			t.Fatalf("folders without a language are skipped, got %v", got)
		}
	})

	t.Run("one is unused for ja ko zh", func(t *testing.T) {
		for _, folder := range []string{"values-ja", "values-ko-rKR", "values-zh-rCN", "values-b+zh+Hans"} {
			got := run(def(folder, "one", "other"))
			if len(got) != 1 {
				t.Fatalf("%s: expected 1 finding, got %v", folder, got)
			}
		}
	})

	t.Run("message follows Lint", func(t *testing.T) {
		got := run(def("values-ja-rJP", "one", "other"))
		want := "For language \"ja\" (Japanese) the following quantities are not relevant: `one`"
		if len(got) != 1 || got[0].Message != want || got[0].Line != 3 {
			t.Fatalf("got %v, want message %q", got, want)
		}
	})

	t.Run("one finding per plurals element lists every unused quantity", func(t *testing.T) {
		got := run(def("values-de", "zero", "one", "two", "few", "many", "other"))
		want := "For language \"de\" (German) the following quantities are not relevant: `few`, `many`, `two`, `zero`"
		if len(got) != 1 || got[0].Message != want {
			t.Fatalf("got %v, want message %q", got, want)
		}
	})

	t.Run("relevant quantities are clean", func(t *testing.T) {
		clean := []android.PluralDefinition{
			def("values-ja", "other"),
			def("values-en", "one", "other"),
			def("values-fr", "one", "many", "other"),
			def("values-ru", "one", "few", "many", "other"),
			def("values-ar", "zero", "one", "two", "few", "many", "other"),
		}
		for _, d := range clean {
			if got := run(d); len(got) != 0 {
				t.Errorf("%s: expected 0 findings, got %v", d.FilePath, got)
			}
		}
	})

	t.Run("unknown language is skipped", func(t *testing.T) {
		if got := run(def("values-zz", "zero", "one", "other")); len(got) != 0 {
			t.Fatalf("expected 0 findings, got %v", got)
		}
	})

	t.Run("non-translatable plurals are still judged", func(t *testing.T) {
		idx := emptyIndex()
		idx.PluralDefinitions = []android.PluralDefinition{def("values-ja", "one", "other")}
		idx.PluralsNonTranslate = map[string]bool{"items": true}
		if got := runResourceRule(r, idx); len(got) != 1 {
			t.Fatalf("Lint ignores translatable=false here, expected 1 finding, got %v", got)
		}
	})

	t.Run("lowercase region qualifiers keep the language", func(t *testing.T) {
		for _, folder := range []string{"values-ko-rkr", "values-ja-rjp", "values-zh-rcn", "values-zh-rtw", "values-JA", "values-ja-r001"} {
			if got := run(def(folder, "one", "other")); len(got) != 1 {
				t.Errorf("%s: expected 1 finding, got %v", folder, got)
			}
		}
	})

	t.Run("language qualifier must follow Android qualifier order", func(t *testing.T) {
		valid := []string{"values-mcc310-ja", "values-mnc004-ja", "values-ja-ldrtl", "values-ja-sw600dp", "values-ja-land-night", "values-ja-night-hdpi", "values-ja-1080x1920-v21", "values-ja-rJP-v21", "values-b+ja-v21"}
		for _, folder := range valid {
			if got := run(def(folder, "one", "other")); len(got) != 1 {
				t.Errorf("%s: expected 1 finding, got %v", folder, got)
			}
		}
		invalid := []string{"values-land-ja", "values-night-ja", "values-v21-ja", "values-ldrtl-ja", "values-sw600dp-ja", "values-ja-v21-night", "values-ja-night-land", "values-ja-ko", "values-ja-rJP-rUS", "values-ja-rJPN", "values-ja-r", "values-ja-foo", "values-mcc1-ja", "values-mcc310-land-ja"}
		for _, folder := range invalid {
			if got := run(def(folder, "one", "other")); len(got) != 0 {
				t.Errorf("%s: not a valid configuration, expected 0 findings, got %v", folder, got)
			}
		}
	})
}

// ---------------------------------------------------------------------------
// ImpliedQuantityResource
// ---------------------------------------------------------------------------

func TestImpliedQuantityResource(t *testing.T) {
	r := findResourceRule(t, "ImpliedQuantityResource")

	t.Run("reports the recorded plurals location", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{"one": "One item", "other": "%d items"}
		idx.PluralSources = map[string]string{"items": "values-fr"}
		idx.PluralsLocation = map[string]android.StringLocation{"items": {FilePath: "res/values/arrays_plurals.xml", Line: 9}}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].File != "res/values/arrays_plurals.xml" || findings[0].Line != 9 {
			t.Fatalf("expected 1 finding at res/values/arrays_plurals.xml:9, got %v", findings)
		}
	})

	t.Run("one without %d triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{
			"one":   "One apple",
			"other": "%d apples",
		}
		idx.PluralSources = map[string]string{"apples": "values-ru"}
		locatePlurals(idx)
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("default and exact-one locales are skipped", func(t *testing.T) {
		for _, dir := range []string{"", "values", "values-v21", "values-en-rGB", "values-de", "values-es", "values-ja"} {
			idx := emptyIndex()
			idx.Plurals["apples"] = map[string]string{"one": "One apple", "other": "%d apples"}
			if dir != "" {
				idx.PluralSources = map[string]string{"apples": dir}
			}
			locatePlurals(idx)
			if findings := runResourceRule(r, idx); len(findings) != 0 {
				t.Errorf("%q: one means exactly 1, expected 0 findings, got %d", dir, len(findings))
			}
		}
	})

	t.Run("language without a one category is skipped", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["apples"] = map[string]string{"one": "苹果", "other": "%d 个苹果"}
		idx.PluralSources = map[string]string{"apples": "values-zh-rTW"}
		locatePlurals(idx)
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings for a zh plural, got %d", len(findings))
		}
		idx.PluralSources = map[string]string{"apples": "values-fr"}
		if findings := runResourceRule(r, idx); len(findings) != 1 {
			t.Fatalf("expected 1 finding for a fr plural, got %d", len(findings))
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

	t.Run("button and label strings are template wrappers and are skipped", func(t *testing.T) {
		for _, name := range []string{"edit_button_template", "Row_Label", "label_fmt"} {
			idx := emptyIndex()
			idx.Strings[name] = "%1$s"
			if findings := runResourceRule(r, idx); len(findings) != 0 {
				t.Errorf("%s: expected 0 findings, got %d", name, len(findings))
			}
		}
	})

	t.Run("strings with a translator note are skipped", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		valuesDir := filepath.Join(resDir, "values")
		if err := os.MkdirAll(valuesDir, 0o755); err != nil {
			t.Fatalf("MkdirAll(%s): %v", valuesDir, err)
		}
		xml := "<resources>\n" +
			"    <!-- Wraps a localized name -->\n    <string name=\"commented\">%1$s</string>\n" +
			"    <string name=\"attributed\" note=\"pass-through\">%1$s</string>\n" +
			"    <string name=\"plain\">%1$s</string>\n" +
			"</resources>\n"
		if err := os.WriteFile(filepath.Join(valuesDir, "strings.xml"), []byte(xml), 0o644); err != nil {
			t.Fatal(err)
		}
		idx, err := android.ScanResourceDir(resDir)
		if err != nil {
			t.Fatal(err)
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "`plain`") {
			t.Fatalf("expected only the plain string to be flagged, got %v", findings)
		}
	})

	t.Run("translation of a placeholder string is skipped", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greeting"] = "%s"
		idx.StringsLocation["greeting"] = android.StringLocation{FilePath: "res/values-de/strings.xml", Line: 3}
		idx.StringSources = map[string]android.StringSource{
			"other": {Value: "unrelated", FilePath: "res/values/strings.xml", Line: 1},
		}
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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

	t.Run("capitalized button labels are translatable", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["ok"] = "OK"
		idx.Strings["cancel"] = "CANCEL"
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings for plain capitalized words, got %d", len(findings))
		}
		idx.Strings["constant"] = "API_KEY"
		if findings := runResourceRule(r, idx); len(findings) != 1 {
			t.Fatalf("expected 1 finding for a constant-style value, got %d", len(findings))
		}
	})

	t.Run("translation override of the same name is ignored", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["site"] = "Visit us"
		idx.StringsLocation["site"] = android.StringLocation{FilePath: "res/values-zh-rTW/strings.xml", Line: 3}
		idx.StringSources = map[string]android.StringSource{
			"site": {Value: "Visit us", FilePath: "res/values/strings.xml", Line: 3},
		}
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("translation-only string is ignored", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["badge"] = "VIP"
		idx.StringsLocation["badge"] = android.StringLocation{FilePath: "res/values-fr/strings.xml", Line: 2}
		idx.StringSources = map[string]android.StringSource{}
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("default definition still triggers when a translation overrides it", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["site"] = "Visit us"
		idx.StringsLocation["site"] = android.StringLocation{FilePath: "res/values-fr/strings.xml", Line: 3}
		idx.StringSources = map[string]android.StringSource{
			"site": {Value: "https://example.com", FilePath: "res/values/strings.xml", Line: 4},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].File != "res/values/strings.xml" {
			t.Fatalf("expected 1 finding in default values, got %+v", findings)
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

	t.Run("Include triggers with Lint's message", func(t *testing.T) {
		idx := indexWithLayout("main", "res/layout/main.xml", &android.View{
			Type:       "Include",
			Line:       5,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if want := "Invalid tag `<Include>`; should be `<include>`"; findings[0].Message != want {
			t.Fatalf("got %q, want %q", findings[0].Message, want)
		}
	})

	t.Run("only the four lowercase-first tags are checked", func(t *testing.T) {
		for _, tag := range []string{"Merge", "Fragment", "RequestFocus"} {
			idx := indexWithLayout("main", "res/layout/main.xml", &android.View{Type: tag, Line: 1, Attributes: map[string]string{}})
			if got := runResourceRule(r, idx); len(got) != 1 {
				t.Errorf("%s: expected a finding, got %d", tag, len(got))
			}
		}
		for _, tag := range []string{"linearlayout", "Textview", "INCLUDE", "viewstub", "include", "merge"} {
			idx := indexWithLayout("main", "res/layout/main.xml", &android.View{Type: tag, Line: 1, Attributes: map[string]string{}})
			if got := runResourceRule(r, idx); len(got) != 0 {
				t.Errorf("%s: Lint does not report it, got %d", tag, len(got))
			}
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

	t.Run("translation identical to the default is reported on the default only", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greet"] = "%s meets %s"
		idx.StringsLocation["greet"] = android.StringLocation{FilePath: "res/values-fr/strings.xml", Line: 9}
		idx.StringSources = map[string]android.StringSource{
			"greet": {Value: "%s meets %s", FilePath: "res/values/strings.xml", Line: 3},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].File != "res/values/strings.xml" || findings[0].Line != 3 {
			t.Fatalf("expected one finding on the default, got %v", findings)
		}
	})

	t.Run("translation with its own bare arguments is reported on the translation", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["greet"] = "%s rencontre %s"
		idx.StringsLocation["greet"] = android.StringLocation{FilePath: "res/values-fr/strings.xml", Line: 9}
		idx.StringSources = map[string]android.StringSource{
			"greet": {Value: "%1$s meets %2$s", FilePath: "res/values/strings.xml", Line: 3},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].File != "res/values-fr/strings.xml" {
			t.Fatalf("expected one finding on the translation, got %v", findings)
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
	root := func(texts ...android.XMLTextNode) *android.View {
		return &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, TextNodes: texts}
	}

	t.Run("first stray text of a layout is reported with Lint wording", func(t *testing.T) {
		idx := indexWithLayout("main", "app/src/main/res/layout-land/main.xml", root(
			android.XMLTextNode{Text: "second", Line: 6, Col: 3}, android.XMLTextNode{Text: "stray   text\nhere", Line: 3, Col: 3}))
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Message != `Unexpected text found in layout file: "stray text here"` || findings[0].Line != 3 {
			t.Fatalf("unexpected finding %+v", findings[0])
		}
	})

	t.Run("menu text is reported", func(t *testing.T) {
		idx := indexWithMenu("m", "res/menu/m.xml", root(android.XMLTextNode{Text: "oops", Line: 2, Col: 1}))
		if findings := runResourceRule(r, idx); len(findings) != 1 || findings[0].Message != `Unexpected text found in menu file: "oops"` {
			t.Fatalf("unexpected findings %+v", findings)
		}
	})

	t.Run("aapt:attr contents are skipped", func(t *testing.T) {
		attr := &android.View{Type: "aapt:attr", Line: 2, Attributes: map[string]string{}, TextNodes: []android.XMLTextNode{{Text: "x", Line: 2, Col: 1}}}
		r0 := root()
		r0.Children = []*android.View{attr}
		if findings := runResourceRule(r, indexWithLayout("main", "res/layout/main.xml", r0)); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("values files are not checked", func(t *testing.T) {
		idx := emptyIndex()
		idx.ExtraTexts = []android.ExtraTextEntry{{FilePath: "res/values/strings.xml", Line: 3, Text: "some stray text"}}
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no extra text is clean", func(t *testing.T) {
		if findings := runResourceRule(r, emptyIndex()); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// IllegalResourceRefResource
// ---------------------------------------------------------------------------

func TestWrongRegionResource(t *testing.T) {
	r := findResourceRule(t, "WrongRegionResource")

	t.Run("de-rUS pairs German with a region Lint does not list", func(t *testing.T) {
		idx := indexWithLayout("main", "res/values-de-rUS/main.xml", &android.View{
			Type:       "LinearLayout",
			Line:       1,
			Attributes: map[string]string{},
		})
		findings := runResourceRule(r, idx)
		want := "Suspicious language and region combination de (German) with US (United States): language de is usually paired with: " +
			"DE (Germany), AT (Austria), BE (Belgium), CH (Switzerland), IT (Italy), LI (Liechtenstein), LU (Luxembourg)"
		if len(findings) != 1 || findings[0].Message != want {
			t.Fatalf("got %+v", findings)
		}
	})

	t.Run("regions Lint does not know for a language are not judged", func(t *testing.T) {
		for _, folder := range []string{"values-en-rUK", "values-es-rUS", "values-en-rEN", "values-xx-rYY"} {
			idx := indexWithLayout("main", "res/"+folder+"/main.xml", &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}})
			if findings := runResourceRule(r, idx); len(findings) != 0 {
				t.Errorf("%s: expected 0 findings, got %d", folder, len(findings))
			}
		}
	})

	t.Run("string translation folders are checked", func(t *testing.T) {
		idx := emptyIndex()
		idx.StringsLocation["greet"] = android.StringLocation{FilePath: "app/src/main/res/values-de-rUS/strings.xml", Line: 3}
		if findings := runResourceRule(r, idx); len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
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

	t.Run("space flag before a letter is probably a literal percent", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["quoted"] = "100% sure"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if want := "Incorrect formatting string `quoted`; missing conversion character in '`% s`'?"; findings[0].Message != want {
			t.Fatalf("message = %q, want %q", findings[0].Message, want)
		}
	})

	t.Run("Lint ignores a bare percent, unknown conversions and % n or % o", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["bare"] = "Hello %s, 50%"
		idx.Strings["badconv"] = "Value is %z"
		idx.Strings["discount"] = "Save 50% now"
		idx.Strings["discount2"] = "Save 50% of it"
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %v", findings)
		}
	})

	t.Run("a space-flag percent copied from the default is reported once per location", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["progress"] = "100% sure"
		idx.StringsLocation["progress"] = android.StringLocation{FilePath: "res/values-fr/strings.xml", Line: 2}
		idx.StringSources = map[string]android.StringSource{
			"progress": {Value: "100% sure", FilePath: "res/values/strings.xml", Line: 2},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].File != "res/values/strings.xml" {
			t.Fatalf("expected one finding on the default, got %v", findings)
		}
	})

	t.Run("comma flag is a valid specifier", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["count"] = "Total %1$,d and %2$(,.2f"
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("translatable=false and formatted=false are skipped", func(t *testing.T) {
		idx := emptyIndex()
		idx.StringsNonTranslate = map[string]bool{"a": true}
		idx.StringsNonFormatted = map[string]bool{"b": true}
		idx.Strings["a"] = "Value %z"
		idx.Strings["b"] = "Value %z"
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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

	t.Run("skipped positional index is reported like Lint does", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["msg"] = "%1$s and %3$s"
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if want := "Formatting string '`msg`' is not referencing numbered arguments [2]"; findings[0].Message != want {
			t.Fatalf("message = %q, want %q", findings[0].Message, want)
		}
	})

	t.Run("translation with a different argument count triggers", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["msg"] = "%1$s"
		idx.StringsLocation["msg"] = android.StringLocation{FilePath: "res/values-fr/strings.xml", Line: 4}
		idx.StringSources = map[string]android.StringSource{
			"msg": {Value: "%1$s and %2$s", FilePath: "res/values/strings.xml", Line: 3},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].File != "res/values-fr/strings.xml" || findings[0].Line != 4 {
			t.Fatalf("expected translation location, got %s:%d", findings[0].File, findings[0].Line)
		}
	})

	t.Run("translation with the same argument count is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["msg"] = "%2$s then %1$s"
		idx.StringsLocation["msg"] = android.StringLocation{FilePath: "res/values-fr/strings.xml", Line: 4}
		idx.StringSources = map[string]android.StringSource{
			"msg": {Value: "%1$s and %2$s", FilePath: "res/values/strings.xml", Line: 3},
		}
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("translatable=false does not exempt a variant", func(t *testing.T) {
		idx := emptyIndex()
		idx.Strings["msg"] = "%1$s"
		idx.StringsNonTranslate = map[string]bool{"msg": true}
		idx.StringsLocation["msg"] = android.StringLocation{FilePath: "res/values-fr/strings.xml", Line: 4}
		idx.StringSources = map[string]android.StringSource{
			"msg": {Value: "%1$s and %2$s", FilePath: "res/values/strings.xml", Line: 3},
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].Message != "Inconsistent number of arguments in formatting string `msg`; found both 1 here and 2 in values/strings.xml" {
			t.Fatalf("unexpected findings: %v", findings)
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
		locatePlurals(idx)
		findings := runResourceRule(r, idx)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "mismatch") {
			t.Fatalf("expected message about mismatch, got: %s", findings[0].Message)
		}
	})

	t.Run("quantity using a subset of the args is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "%d item",
			"other": "%d of %d items",
		}
		findings := runResourceRule(r, idx)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("quantity omitting the number is clean", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{
			"one":   "Only %s",
			"other": "%d of %s",
		}
		if findings := runResourceRule(r, idx); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("mismatch reports the recorded plurals location", func(t *testing.T) {
		idx := emptyIndex()
		idx.Plurals["items"] = map[string]string{"one": "%d item", "other": "%s items"}
		idx.PluralsLocation = map[string]android.StringLocation{"items": {FilePath: "res/values/plurals.xml", Line: 7}}
		findings := runResourceRule(r, idx)
		if len(findings) != 1 || findings[0].Line != 7 {
			t.Fatalf("expected 1 finding at line 7, got %v", findings)
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

	t.Run("non-language qualifier folders are clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"), "<resources><string name=\"app_name\">App</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"), "<resources><string name=\"app_name\">Appli</string></resources>\n")
		write(t, filepath.Join(resDir, "values-car", "dimens.xml"), "<resources><dimen name=\"pad\">8dp</dimen></resources>\n")
		write(t, filepath.Join(resDir, "xml", "locales_config.xml"), `<locale-config xmlns:android="http://schemas.android.com/apk/res/android">
    <locale android:name="en" />
    <locale android:name="fr" />
</locale-config>
`)
		if findings := runResourceRule(r, scan(t, resDir)); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %v", findingMessages(findings))
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

	t.Run("styled string parsed from markup elements triggers", func(t *testing.T) {
		layout := &android.Layout{
			Name:     "main",
			FilePath: "res/layout/main.xml",
			RootView: &android.View{Type: "ImageView", ContentDescription: "@string/img_desc", Line: 5, Attributes: map[string]string{}},
		}
		idx := &android.ResourceIndex{
			Layouts:           map[string]*android.Layout{"main": layout},
			LayoutConfigs:     map[string]map[string]*android.Layout{"main": {"": layout}},
			Strings:           map[string]string{"img_desc": "Bold image description"},
			StringsWithMarkup: map[string]bool{"img_desc": true},
			StringsLocation:   map[string]android.StringLocation{"img_desc": {FilePath: "res/values/strings.xml", Line: 3}},
			Colors:            make(map[string]string),
			Dimensions:        make(map[string]string),
			Styles:            make(map[string]*android.Style),
			StringArrays:      make(map[string][]string),
			Plurals:           make(map[string]map[string]string),
			Integers:          make(map[string]string),
			Booleans:          make(map[string]string),
			IDs:               make(map[string]bool),
		}
		if findings := runResourceRule(r, idx); len(findings) != 1 {
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

	t.Run("ru plurals without zero is clean", func(t *testing.T) {
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
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
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
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"),
			"<resources><string name=\"emphasis\">Ceci est &lt;b&gt;gras&lt;/b&gt;</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("many locales dropping the markup report once per key", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"emphasis\">This is &lt;b&gt;bold&lt;/b&gt;</string></resources>\n")
		write(t, filepath.Join(resDir, "values-de", "strings.xml"),
			"<resources><string name=\"emphasis\">Das ist fett</string></resources>\n")
		write(t, filepath.Join(resDir, "values-es", "strings.xml"),
			"<resources><string name=\"emphasis\">Esto es negrita</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"),
			"<resources><string name=\"emphasis\">Ceci est **gras**</string></resources>\n")
		write(t, filepath.Join(resDir, "values-it", "strings.xml"),
			"<resources><string name=\"emphasis\">Questo e &lt;b&gt;grassetto&lt;/b&gt;</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding for the key, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "values-de/") || !strings.Contains(findings[0].Message, "2 other locales differ too") {
			t.Fatalf("unexpected message: %q", findings[0].Message)
		}
	})

	t.Run("plain default with html in a translation is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"hello\">Hello world</string></resources>\n")
		write(t, filepath.Join(resDir, "values-de", "strings.xml"),
			"<resources><string name=\"hello\">Hallo <b>Welt</b></string></resources>\n")
		if findings := runResourceRule(r, scan(t, resDir)); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("cdata markup is judged like escaped and element markup", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"title\"><![CDATA[<b>No matches</b>]]></string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"),
			"<resources><string name=\"title\">&lt;b&gt;Aucun&lt;/b&gt;</string></resources>\n")
		write(t, filepath.Join(resDir, "values-it", "strings.xml"),
			"<resources><string name=\"title\"><![CDATA[<b>Nessuno</b>]]></string></resources>\n")
		write(t, filepath.Join(resDir, "values-es", "strings.xml"),
			"<resources><string name=\"title\"><![CDATA[Ninguno]]></string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "values-es/") {
			t.Fatalf("expected only the translation that lost the markup, got %v", findings)
		}
	})

	t.Run("markup stripped from every translation is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"emphasis\">This is &lt;b&gt;bold&lt;/b&gt;</string></resources>\n")
		write(t, filepath.Join(resDir, "values-es", "strings.xml"),
			"<resources><string name=\"emphasis\">Esto es negrita</string></resources>\n")
		write(t, filepath.Join(resDir, "values-de", "strings.xml"),
			"<resources><string name=\"emphasis\">Das ist fett</string></resources>\n")
		if findings := runResourceRule(r, scan(t, resDir)); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("message names the real values directory", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"hello\">Hello world</string></resources>\n")
		write(t, filepath.Join(resDir, "values-de-rDE", "strings.xml"),
			"<resources><string name=\"hello\">Hallo **Welt**</string></resources>\n")
		findings := runResourceRule(r, scan(t, resDir))
		if len(findings) != 1 || !strings.Contains(findings[0].Message, "`values-de-rDE/`") {
			t.Fatalf("expected the values-de-rDE directory in the message, got %v", findings)
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

	t.Run("formatted false is clean", func(t *testing.T) {
		resDir := filepath.Join(t.TempDir(), "res")
		write(t, filepath.Join(resDir, "values", "strings.xml"),
			"<resources><string name=\"tmpl\" formatted=\"false\">Hello **%{name}**</string></resources>\n")
		write(t, filepath.Join(resDir, "values-fr", "strings.xml"),
			"<resources><string name=\"tmpl\" formatted=\"false\">Bonjour %{name}</string></resources>\n")
		if findings := runResourceRule(r, scan(t, resDir)); len(findings) != 0 {
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

func TestImpliedQuantityResourceNumberPlaceholders(t *testing.T) {
	r := findResourceRule(t, "ImpliedQuantityResource")
	run := func(one string, mutate func(*android.ResourceIndex)) int {
		idx := emptyIndex()
		idx.Plurals["rows"] = map[string]string{"one": one, "other": "%d rows"}
		idx.PluralSources = map[string]string{"rows": "values-fr"}
		locatePlurals(idx)
		idx.PluralsNonTranslate = map[string]bool{}
		if mutate != nil {
			mutate(idx)
		}
		return len(runResourceRule(r, idx))
	}
	for _, one := range []string{"%d row", "%1$d row", "%2$,d row", "%1$s of %2$d rows", "%1$s row", "%s row", "^1 row"} {
		if n := run(one, nil); n != 0 {
			t.Errorf("%q has a number placeholder, got %d findings", one, n)
		}
	}
	if n := run("one row", nil); n != 1 {
		t.Errorf("literal one should report, got %d", n)
	}
	for _, one := range []string{"100%% done", "%n row"} {
		if n := run(one, nil); n != 1 {
			t.Errorf("%q has no format argument, got %d findings", one, n)
		}
	}
	if n := run("one row", func(idx *android.ResourceIndex) { idx.PluralsNonTranslate["rows"] = true }); n != 1 {
		t.Errorf("Lint still reports a translatable=false plural, got %d", n)
	}
}

func TestPluralRulesSkipUnlocatedEntries(t *testing.T) {
	idx := emptyIndex()
	idx.Plurals["rows"] = map[string]string{"one": "one row"}
	for _, id := range []string{"ImpliedQuantityResource", "MissingQuantityResource"} {
		if f := runResourceRule(findResourceRule(t, id), idx); len(f) != 0 {
			t.Errorf("%s: an entry without a recorded position must not be reported at a made-up path, got %v", id, f)
		}
	}
}

func TestStringNotLocalizableResourceSkipsPerLocaleValues(t *testing.T) {
	r := findResourceRule(t, "StringNotLocalizableResource")
	idx := emptyIndex()
	idx.Strings["home_url"] = "https://example.com/en"
	idx.TranslatedTokens = map[string][]string{"home_url": {"https://example.com/de"}, "help_url": {"https://example.com/help"}}
	idx.Strings["help_url"] = "https://example.com/help"
	findings := runResourceRule(r, idx)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "help_url") {
		t.Fatalf("expected only the identical-in-all-locales URL to report, got %v", findings)
	}
}

func TestStringFormatCountResourceCountsChildElementText(t *testing.T) {
	r := findResourceRule(t, "StringFormatCountResource")
	idx := emptyIndex()
	idx.Strings["greet"] = "Hallo %1$s"
	idx.StringsLocation["greet"] = android.StringLocation{FilePath: "res/values-de/strings.xml", Line: 2}
	idx.StringSources = map[string]android.StringSource{"greet": {Value: "Hello %1$s", FilePath: "res/values/strings.xml", Line: 2}}
	if f := runResourceRule(r, idx); len(f) != 0 {
		t.Fatalf("matching counts must be clean, got %v", f)
	}
}

func TestStringFormatCountResourceReportsEachVariant(t *testing.T) {
	r := findResourceRule(t, "StringFormatCountResource")
	idx := emptyIndex()
	idx.StringSources = map[string]android.StringSource{
		"msg": {Value: "Managed by %1$s", FilePath: "res/values/strings.xml", Line: 3},
	}
	idx.Strings["msg"] = "Verwaltet von"
	idx.StringsLocation["msg"] = android.StringLocation{FilePath: "res/values-de/strings.xml", Line: 4}
	idx.StringTranslations = map[string][]android.StringSource{"msg": {
		{FilePath: "res/values-de/strings.xml", Line: 4, Value: "Verwaltet von"},
		{FilePath: "res/values-fr/strings.xml", Line: 5, Value: "Gere par"},
		{FilePath: "res/values-it/strings.xml", Line: 6, Value: "Gestito da"},
		{FilePath: "res/values-es/strings.xml", Line: 7, Value: "Gestionado por %1$s"},
	}}
	got := runResourceRule(r, idx)
	if len(got) != 3 {
		t.Fatalf("Lint reports each variant with a different count, got %v", got)
	}
	if got[0].File != "res/values-de/strings.xml" || got[1].File != "res/values-fr/strings.xml" || got[2].File != "res/values-it/strings.xml" {
		t.Fatalf("unexpected locations: %v", got)
	}
}

func TestImpliedQuantityResource_PerLocaleDefinitions(t *testing.T) {
	r := findResourceRule(t, "ImpliedQuantityResource")
	idx := emptyIndex()
	idx.PluralDefinitions = []android.PluralDefinition{
		{Name: "a", FilePath: "res/values/p.xml", Line: 2, Quantities: map[string]string{"one": "One", "other": "%d"}},
		{Name: "a", FilePath: "res/values-fr/p.xml", Line: 2, Quantities: map[string]string{"one": "Une", "other": "%d"},
			QuantityPos: map[string]android.XMLPos{"one": {Line: 3, Col: 21}}},
		{Name: "a", FilePath: "res/values-lv/p.xml", Line: 2, Quantities: map[string]string{"zero": "Nulle", "one": "%d", "other": "%d"}},
		{Name: "a", FilePath: "res/values-de/p.xml", Line: 2, Quantities: map[string]string{"one": "Eine", "other": "%d"}},
	}
	got := runResourceRule(r, idx)
	if len(got) != 2 {
		t.Fatalf("expected the fr and lv definitions to report, got %+v", got)
	}
	var fr, lv string
	for _, f := range got {
		if strings.Contains(f.File, "values-fr") {
			fr = f.Message
			if f.Line != 3 || f.Col != 21 {
				t.Errorf("fr finding must sit on the item, got %d:%d", f.Line, f.Col)
			}
		} else {
			lv = f.Message
		}
	}
	if !strings.HasPrefix(fr, "The quantity `'one'` matches more than one specific number in this locale (0, 1), but the message did not include a formatting argument (such as `%d`).") {
		t.Errorf("unexpected fr message %q", fr)
	}
	if !strings.Contains(lv, "`'zero'`") || !strings.Contains(lv, "(0, 10~20, 30,") {
		t.Errorf("unexpected lv message %q", lv)
	}
}
