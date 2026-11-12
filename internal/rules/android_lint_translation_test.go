package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

func writeTranslationFiles(t *testing.T, files map[string]string) string {
	t.Helper()
	root := filepath.Join(t.TempDir(), "app", "src", "main", "res")
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// translationLines renders findings as "Issue folder/file line:col message".
func translationLines(root string) []string {
	var out []string
	for _, f := range analyzeTranslations([]string{root}) {
		rel, _ := filepath.Rel(root, f.file)
		out = append(out, strings.Join([]string{f.issue, filepath.ToSlash(rel), strconv.Itoa(f.line) + ":" + strconv.Itoa(f.col), f.msg}, " | "))
	}
	sort.Strings(out)
	return out
}

func wantLines(t *testing.T, got []string, want ...string) {
	t.Helper()
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("findings differ\ngot:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestTranslationMissingAndExtra(t *testing.T) {
	root := writeTranslationFiles(t, map[string]string{
		"values/strings.xml": `<resources>
  <string name="a">x</string>
  <string name="b">x</string>
  <string name="nt" translatable="false">x</string>
  <string name="google_app_id">x</string>
</resources>
`,
		"values-fr/strings.xml": `<resources>
  <string name="a">x</string>
  <string name="extra">x</string>
</resources>
`,
		"values-es/strings.xml": `<resources>
  <string name="a">x</string>
</resources>
`,
		"values-night/colors.xml": `<resources>
  <color name="night">#000</color>
</resources>
`,
	})
	wantLines(t, translationLines(root),
		`MissingTranslation | values/strings.xml | 3:11 | "b" is not translated in "fr" (French) or "es" (Spanish)`,
		`ExtraTranslation | values-fr/strings.xml | 3:11 | "extra" is translated here but not found in default locale`,
		"MissingDefaultResource | values-night/colors.xml | 2:10 | The color \"night\" in values-night has no declaration in the base `values` folder; this can lead to crashes when the resource is queried in a configuration that does not match this qualifier",
	)
}

func TestTranslationUntranslatableAndDefaultFolders(t *testing.T) {
	root := writeTranslationFiles(t, map[string]string{
		"values/strings.xml": `<resources>
  <string name="nt" translatable="false">x</string>
</resources>
`,
		"values-v21/more.xml": `<resources>
  <string name="v21">x</string>
</resources>
`,
		"values-de/strings.xml": `<resources>
  <string name="nt">x</string>
  <string name="v21">x</string>
</resources>
`,
		"drawable-hdpi/icon.png":  "x",
		"drawable-night/icon.png": "x",
		"layout-land/only.xml":    "<?xml version=\"1.0\"?>\n<!-- c -->\n<FrameLayout/>\n",
	})
	wantLines(t, translationLines(root),
		"Untranslatable | values-de/strings.xml | 2:11 | The resource string \"nt\" has been marked as translatable=\"false\" elsewhere (usually in the `values` folder), but is translated to \"de\" (German) here",
		"MissingDefaultResource | layout-land/only.xml | 3:1 | The layout \"only\" in layout-land has no declaration in the base `layout` folder; this can lead to crashes when the resource is queried in a configuration that does not match this qualifier",
	)
}

func TestTranslationDuplicates(t *testing.T) {
	root := writeTranslationFiles(t, map[string]string{
		"values/a.xml": `<resources>
  <string name="dup">1</string>
  <string name="dup">2</string>
  <color name="dup">#fff</color>
  <string name="phone" product="phone">p</string>
  <string name="phone" product="tablet">t</string>
  <style name="S"><item name="android:a">1</item><item name="android:a">2</item></style>
</resources>
`,
		"values/b.xml": `<resources>
  <string name="dup">3</string>
</resources>
`,
		"values-night/a.xml": `<resources>
  <string name="dup">4</string>
</resources>
`,
	})
	var dups []string
	for _, l := range translationLines(root) {
		if strings.HasPrefix(l, "DuplicateDefinition") {
			dups = append(dups, l)
		}
	}
	wantLines(t, dups,
		"DuplicateDefinition | values/a.xml | 3:11 | `dup` has already been defined in this folder",
		"DuplicateDefinition | values/b.xml | 2:11 | `dup` has already been defined in this folder",
		"DuplicateDefinition | values/a.xml | 7:56 | `android:a` has already been defined in this `<style>`",
	)
}

func TestTranslationLocaleOrderMatchesJavaHashSet(t *testing.T) {
	got := javaHashSetOrder([]string{"fr", "es", "sr"})
	if strings.Join(got, ",") != "fr,es,sr" {
		t.Fatalf("order = %v", got)
	}
	got = javaHashSetOrder([]string{"b+zh+Hant"[2:4], "es"})
	if strings.Join(got, ",") != "zh,es" {
		t.Fatalf("collision order = %v", got)
	}
}

func TestTranslationToolsLocaleAndIgnoredFiles(t *testing.T) {
	root := writeTranslationFiles(t, map[string]string{
		"values/strings.xml": `<resources xmlns:tools="http://schemas.android.com/tools" tools:locale="en">
  <string name="a">x</string>
</resources>
`,
		"values/donottranslate.xml": `<resources><string name="skipped">x</string></resources>`,
		"values-en/strings.xml":     `<resources><string name="b">x</string></resources>`,
		"values-fr/strings.xml": `<resources translatable="false">
  <string name="a">x</string>
</resources>
`,
	})
	wantLines(t, translationLines(root),
		`ExtraTranslation | values-en/strings.xml | 1:20 | "b" is translated here but not found in default locale`,
		`Untranslatable | values-fr/strings.xml | 1:12 | This resource folder is marked as non-translatable yet is in a translated resource folder ("fr" (French))`,
		`MissingTranslation | values/strings.xml | 2:11 | "a" is not translated in "fr" (French)`,
	)
}

func TestTranslationUntranslatableCopiedIntoRegionLocale(t *testing.T) {
	// Translation tooling often copies translatable="false" strings into every locale file; Lint flags each copy once.
	root := writeTranslationFiles(t, map[string]string{
		"values/strings.xml": `<resources>
  <string name="help_url" translatable="false">https://example.com/help</string>
  <string name="greeting">Hello</string>
</resources>
`,
		"values-pt-rBR/strings.xml": `<resources>
  <string name="help_url" translatable="false">https://example.com/help</string>
  <string name="greeting">Olá</string>
</resources>
`,
		"values-zh-rTW/strings.xml": `<resources>
  <string name="greeting">你好</string>
</resources>
`,
	})
	wantLines(t, translationLines(root),
		`Untranslatable | values-pt-rBR/strings.xml | 2:27 | The resource string "help_url" is marked as translatable="false", but is translated to "pt" (Portuguese) here`,
	)
}

func TestTranslationRulesMatchLintRegistration(t *testing.T) {
	// Lint 32.4.1 enables all five issues by default; severities and priorities come from their Issue.create calls.
	want := map[string]struct {
		sev  AndroidLintSeverity
		cat  AndroidLintCategory
		prio int
	}{
		lintMissingTranslation: {ALSError, ALCMessages, 8},
		lintExtraTranslation:   {ALSFatal, ALCMessages, 6},
		lintMissingDefault:     {ALSFatal, ALCCorrectness, 6},
		lintUntranslatable:     {ALSWarning, ALCMessages, 6},
		lintDuplicateDef:       {ALSError, ALCCorrectness, 6},
	}
	found := 0
	for _, r := range api.Registry {
		w, ok := want[r.ID]
		if !ok {
			continue
		}
		var ar AndroidRule
		switch impl := r.Implementation.(type) {
		case *MissingTranslationRule:
			ar = impl.AndroidRule
		case *ExtraTranslationRule:
			ar = impl.AndroidRule
		case *MissingDefaultResourceRule:
			ar = impl.AndroidRule
		case *UntranslatableRule:
			ar = impl.AndroidRule
		case *DuplicateDefinitionRule:
			ar = impl.AndroidRule
		default:
			continue
		}
		found++
		if ar.ALSeverity != w.sev || ar.Category != w.cat || ar.Priority != w.prio {
			t.Errorf("%s: severity %v category %v priority %d, want %v %v %d", r.ID, ar.ALSeverity, ar.Category, ar.Priority, w.sev, w.cat, w.prio)
		}
		if !IsDefaultActive(r.ID) {
			t.Errorf("%s should be active by default like Lint", r.ID)
		}
	}
	if found != len(want) {
		t.Fatalf("found %d of %d translation rules in the registry", found, len(want))
	}
}

func TestTranslationRootsSkipTestSourceSets(t *testing.T) {
	main := writeTranslationFiles(t, map[string]string{
		"values/strings.xml": "<resources>\n  <string name=\"a\">x</string>\n</resources>\n",
	})
	src := filepath.Dir(filepath.Dir(main))
	locale := "<resources>\n  <string name=\"other\">x</string>\n</resources>\n"
	for _, set := range []string{"debug", "test", "androidTest", "androidTestDebug"} {
		dir := filepath.Join(src, set, "res", "values-de")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "strings.xml"), []byte(locale), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	got := translationRoots(&api.Context{ResourceIndex: &android.ResourceIndex{ResDirs: []string{main}}})
	want := []string{filepath.Join(src, "debug", "res"), main}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("roots = %v, want %v", got, want)
	}
}
