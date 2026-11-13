package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func unusedResourceContext(t *testing.T, ruleID string, files map[string]string, code ...string) (string, *api.Context) {
	t.Helper()
	root := t.TempDir()
	resDir := filepath.Join(root, "res")
	for name, body := range files {
		path := filepath.Join(resDir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	idx, err := android.ScanResourceDir(resDir)
	if err != nil {
		t.Fatal(err)
	}
	var sources [][]byte
	for _, c := range code {
		sources = append(sources, []byte(c))
	}
	r := findResourceRule(t, ruleID)
	ctx := &api.Context{ResourceIndex: idx, Rule: r, Collector: scanner.NewFindingCollector(0), ResourceCodeRefs: rules.ComputeResourceCodeRefs(sources)}
	r.Check(ctx)
	return root, ctx
}

func unusedResourceFindings(t *testing.T, ruleID string, files map[string]string, code ...string) []string {
	t.Helper()
	root, ctx := unusedResourceContext(t, ruleID, files, code...)
	var out []string
	for _, f := range api.ContextFindings(ctx) {
		rel, _ := filepath.Rel(root, f.File)
		out = append(out, filepath.ToSlash(rel)+" "+strings.TrimSuffix(strings.TrimPrefix(f.Message, "The resource `R."), "` appears to be unused"))
	}
	sort.Strings(out)
	return out
}

// unusedResourceLines returns "line id" for each finding, sorted by line.
func unusedResourceLines(t *testing.T, ruleID string, files map[string]string) []string {
	t.Helper()
	_, ctx := unusedResourceContext(t, ruleID, files)
	var out []string
	for _, f := range api.ContextFindings(ctx) {
		out = append(out, fmt.Sprintf("%d %s", f.Line, strings.TrimSuffix(strings.TrimPrefix(f.Message, "The resource `R."), "` appears to be unused")))
	}
	sort.Strings(out)
	return out
}

const unusedNS = `xmlns:android="http://schemas.android.com/apk/res/android"`

func TestUnusedResourcesFollowsLint(t *testing.T) {
	files := map[string]string{
		"values/strings.xml": `<resources>
  <string name="used_in_code">x</string>
  <string name="used_in_layout">x</string>
  <string name="unused">x</string>
  <string name="only_by_unused_array">x</string>
  <string-array name="unused_array"><item>@string/only_by_unused_array</item></string-array>
  <string name="kept_by_tools">x</string>
</resources>`,
		"values-de/strings.xml": `<resources><string name="unused">y</string></resources>`,
		"values/styles.xml": `<resources>
  <style name="AppTheme"/>
  <style name="Base"/>
  <style name="Base.Child"/>
  <style name="Used.Child" parent="Base"/>
  <attr name="someAttr" format="string"/>
  <declare-styleable name="Thing"><attr name="x" format="string"/></declare-styleable>
</resources>`,
		"layout/main.xml":   `<LinearLayout ` + unusedNS + `><TextView android:text="@string/used_in_layout" style="@style/Used.Child" android:id="@+id/title"/></LinearLayout>`,
		"layout/orphan.xml": `<LinearLayout ` + unusedNS + `/>`,
		"layout/bound.xml":  `<LinearLayout ` + unusedNS + `/>`,
		"drawable/ic.xml":   `<shape ` + unusedNS + `/>`,
		"raw/keep.xml":      `<resources xmlns:tools="http://schemas.android.com/tools" tools:keep="@string/kept_by_tools"/>`,
	}
	code := []string{
		"class A { fun f() { getString(R.string.used_in_code); setContentView(R.layout.main) } }",
		"// R.drawable.ic is only mentioned in a comment\nclass B { val b = BoundBinding.inflate(i) }",
	}
	got := strings.Join(unusedResourceFindings(t, "UnusedResources", files, code...), "\n")
	want := strings.Join([]string{
		"res/drawable/ic.xml drawable.ic",
		"res/layout/orphan.xml layout.orphan",
		"res/values/strings.xml array.unused_array",
		"res/values/strings.xml string.only_by_unused_array",
		"res/values/strings.xml string.unused",
		"res/values/styles.xml style.Base_Child",
		"res/values/styles.xml style.Used_Child",
	}, "\n")
	// Used.Child is used by the layout, so it stays; Base is its explicit parent, so it stays.
	want = strings.Replace(want, "\nres/values/styles.xml style.Used_Child", "", 1)
	want = strings.Replace(want, "res/values/styles.xml style.Base_Child", "res/values/styles.xml style.AppTheme\nres/values/styles.xml style.Base_Child", 1)
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUnusedIdsFollowsLint(t *testing.T) {
	files := map[string]string{
		"layout/main.xml": `<LinearLayout ` + unusedNS + `>
  <TextView android:id="@+id/used_in_code"/>
  <TextView android:id="@+id/referenced" android:layout_below="@id/other"/>
  <TextView android:id="@+id/other"/>
  <TextView android:id="@+id/unused"/>
</LinearLayout>`,
		"values/ids.xml": `<resources><item name="unused_item" type="id"/></resources>`,
	}
	// Nothing uses the main layout, so the reference it makes to `other` does not keep that id either.
	got := strings.Join(unusedResourceFindings(t, "UnusedIds", files, "class A { val i = R.id.used_in_code }"), "\n")
	want := "res/layout/main.xml id.other\nres/layout/main.xml id.referenced\nres/layout/main.xml id.unused\nres/values/ids.xml id.unused_item"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUnusedIdsFollowsLintReachability(t *testing.T) {
	files := map[string]string{
		"layout/main.xml": `<RelativeLayout ` + unusedNS + `>
  <TextView android:id="@+id/anchor"/>
  <TextView
      android:layout_below="@id/anchor"
      android:layout_above="@+id/forward"
      android:id="@+id/below_anchor"/>
  <TextView android:id="@+id/forward"/>
</RelativeLayout>`,
		"layout/dead.xml": `<RelativeLayout ` + unusedNS + `>
  <TextView android:id="@+id/dead_anchor"/>
  <TextView android:id="@+id/dead_user" android:layout_below="@id/dead_anchor"/>
</RelativeLayout>`,
		"navigation/graph.xml": `<navigation ` + unusedNS + `><fragment android:id="@+id/dest_a"/></navigation>`,
	}
	got := strings.Join(unusedResourceFindings(t, "UnusedIds", files, "class A { val l = R.layout.main }"), "\n")
	// A layout that code uses keeps the ids it references; ids in a layout nothing uses stay unused.
	// Navigation ids are never reported, and the forward `@+id/` reference is where Lint reports `forward`.
	want := "res/layout/dead.xml id.dead_anchor\nres/layout/dead.xml id.dead_user\nres/layout/main.xml id.below_anchor"
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUnusedIdsReportAtLintLocation(t *testing.T) {
	files := map[string]string{
		"layout/main.xml": "<!-- header -->\n<RelativeLayout " + unusedNS + ">\n" +
			"  <TextView\n      android:layout_width=\"1dp\"\n      android:id=\"@+id/multi_line\"/>\n" +
			"  <TextView android:layout_above=\"@+id/forward\"/>\n" +
			"  <TextView android:id=\"@+id/forward\"/>\n" +
			"</RelativeLayout>",
	}
	// The id attribute is on line 5 and the first `@+id/forward` mention on line 6.
	got := strings.Join(unusedResourceLines(t, "UnusedIds", files), "\n")
	if want := "5 id.multi_line\n6 id.forward"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUnusedResourcesFileLineIsRootElement(t *testing.T) {
	files := map[string]string{
		"drawable/shape.xml": "<?xml version=\"1.0\"?>\n<!--\n  header\n-->\n<shape " + unusedNS + "/>",
	}
	// Lint reports the root element, which is on line 5 below the header comment.
	got := strings.Join(unusedResourceLines(t, "UnusedResources", files), "\n")
	if want := "5 drawable.shape"; got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestUnusedResourcesCodeRefs(t *testing.T) {
	refs := rules.ComputeResourceCodeRefs([][]byte{
		[]byte("import com.example.strings.R as Strings\nfun f() = Strings.string.aliased + R.string.plain + android.R.string.skip + app.R.string.qualified"),
		[]byte("/* R.string.in_block */ // R.string.in_line\nval s = \"x\""),
		[]byte("class A(private val b: ActivityMainBinding)"),
	})
	for _, want := range []string{"string/aliased", "string/plain", "string/qualified"} {
		if !refs.Refs[want] {
			t.Errorf("missing reference %s in %v", want, refs.Refs)
		}
	}
	for _, not := range []string{"string/skip", "string/in_block", "string/in_line"} {
		if refs.Refs[not] {
			t.Errorf("unexpected reference %s", not)
		}
	}
	if !refs.Bindings["ActivityMainBinding"] {
		t.Errorf("missing binding class in %v", refs.Bindings)
	}
}

func TestUnusedResourcesCodeRefsAcrossLines(t *testing.T) {
	refs := rules.ComputeResourceCodeRefs([][]byte{[]byte("val x = Strings.string\n    .wrapped_name\nimport a.R as Strings")})
	if !refs.Refs["string/wrapped_name"] {
		t.Fatalf("a reference split over lines must count, got %v", refs.Refs)
	}
}

func TestUnusedResourcesCodeRefsRespectsLiterals(t *testing.T) {
	refs := rules.ComputeResourceCodeRefs([][]byte{
		[]byte("val mime = \"image/*\"\nval a = R.string.after_mime\nval q = '\"'\nval b = R.string.after_char"),
		[]byte("val raw = \"\"\"\n// not a comment /*\n\"\"\"\nval c = R.string.after_raw\n/* R.string.in_block */"),
		[]byte("val esc = \"\\\" /* \"\nval d = R.string.after_escape"),
	})
	for _, want := range []string{"string/after_mime", "string/after_char", "string/after_raw", "string/after_escape"} {
		if !refs.Refs[want] {
			t.Errorf("missing reference %s in %v", want, refs.Refs)
		}
	}
	if refs.Refs["string/in_block"] {
		t.Errorf("a real block comment must still be stripped")
	}
}
