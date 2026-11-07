package rules

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/manifest"
	"github.com/kaeawc/krit/internal/scanner"
)

// This suite covers the single tools:ignore module (tools_ignore.go): the
// manifest, values, layout and xml/ resource filters and the token model they share.

func ignoreFindings(path string, rows ...scanner.Finding) scanner.FindingColumns {
	c := scanner.NewFindingCollector(0)
	for _, f := range rows {
		f.File = path
		if f.RuleSet == "" {
			f.RuleSet = "android-lint"
		}
		c.Append(f)
	}
	return *c.Columns()
}

func keptLines(cols scanner.FindingColumns) []int {
	var lines []int
	for row := 0; row < cols.Len(); row++ {
		lines = append(lines, cols.LineAt(row))
	}
	sort.Ints(lines)
	return lines
}

func equalLines(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

func writeIgnoreFixture(t *testing.T, rel, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// --- token model ---

func TestToolsIgnoreSetSilences(t *testing.T) {
	for _, tc := range []struct {
		raw, rule string
		want      bool
	}{
		{"all", "HardcodedValuesResource", true},
		{"ALL", "HardcodedValuesResource", true},
		{"HardcodedValuesResource", "HardcodedValuesResource", true},
		{"hardcodedvaluesresource", "HardcodedValuesResource", true},
		{"HardcodedText", "HardcodedValuesResource", true}, // Android Lint ID of the rule
		{"hardcodedtext", "HardcodedValuesResource", true},
		{"AndroidLintHardcodedText", "HardcodedValuesResource", true},
		{"Foo,HardcodedText", "HardcodedValuesResource", true},
		{"Foo, HardcodedText ", "HardcodedValuesResource", true},
		{"Foo\nHardcodedText", "HardcodedValuesResource", true},
		{"Hardcoded", "HardcodedValuesResource", false},
		{"HardcodedTextX", "HardcodedValuesResource", false},
		{"android:HardcodedText", "HardcodedValuesResource", false},
		{"Other", "HardcodedValuesResource", false},
		{"", "HardcodedValuesResource", false},
	} {
		if got := toolsIgnoreSetOf(android.ToolsIgnoreTokens(tc.raw)).silences(tc.rule); got != tc.want {
			t.Errorf("tools:ignore=%q on %s: got %v, want %v", tc.raw, tc.rule, got, tc.want)
		}
	}
}

func TestToolsIgnoreSetExtendInherits(t *testing.T) {
	parent := toolsIgnoreSetOf([]string{"A"})
	child := parent.extend("B, C")
	if !child.silences("A") || !child.silences("B") || !child.silences("C") {
		t.Fatalf("a child keeps its ancestors' tokens, got %v", child)
	}
	if parent.silences("B") {
		t.Fatal("extending must not change the parent")
	}
	if got := parent.extend(""); got.silences("B") || !got.silences("A") {
		t.Fatalf("an element without the attribute inherits unchanged, got %v", got)
	}
}

func TestResourceToolsIgnoreMatchesViewAttribute(t *testing.T) {
	v := &android.View{Attributes: map[string]string{"tools:ignore": "Foo, hardcodedtext"}}
	if !resourceToolsIgnoreMatches(v, "HardcodedValuesResource") {
		t.Error("expected the Lint ID, matched ignoring case, to silence the rule")
	}
	if resourceToolsIgnoreMatches(v, "ContentDescription") {
		t.Error("unrelated IDs are not silenced")
	}
	if resourceToolsIgnoreMatches(&android.View{Attributes: map[string]string{}}, "HardcodedText") || resourceToolsIgnoreMatches(nil, "x") {
		t.Error("a view without the attribute, or nil, silences nothing")
	}
}

// --- manifest: element-line index from the manifest model ---

func TestFilterManifestToolsIgnoredByLine(t *testing.T) {
	collector := scanner.NewFindingCollector(0)
	for _, line := range []int{4, 5, 6} {
		collector.Append(scanner.Finding{File: "AndroidManifest.xml", Line: line, Col: 1, Rule: "ProtectedPermissionsManifest", RuleSet: "android-lint", Severity: "error", Message: "m"})
	}
	m := &manifest.Manifest{ToolsIgnoreByLine: map[int][]string{4: {"ProtectedPermissions"}, 6: {"all"}}}
	got := filterManifestToolsIgnoredByLine(*collector.Columns(), m)
	if got.Len() != 1 || got.LineAt(0) != 5 {
		t.Fatalf("expected only the unignored line 5 to remain, got %d rows", got.Len())
	}
}

func TestFilterManifestToolsIgnoredByLineFileLevelUsesRootElement(t *testing.T) {
	collector := scanner.NewFindingCollector(0)
	collector.Append(scanner.Finding{File: "AndroidManifest.xml", Line: 1, Col: 1, Rule: "MissingVersionManifest", RuleSet: "android-lint", Severity: "warning", Message: "m"})
	collector.Append(scanner.Finding{File: "AndroidManifest.xml", Line: 1, Col: 1, Rule: "UsesSdkManifest", RuleSet: "android-lint", Severity: "warning", Message: "m"})
	m := &manifest.Manifest{RootLine: 2, ToolsIgnoreByLine: map[int][]string{2: {"MissingVersion"}}}
	got := filterManifestToolsIgnoredByLine(*collector.Columns(), m)
	if got.Len() != 1 || got.RuleAt(0) != "UsesSdkManifest" {
		t.Fatalf("expected only UsesSdkManifest to remain, got %d rows", got.Len())
	}
}

// --- manifest: source scan ---

const manifestIgnoreSample = `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools">
    <application
        android:allowBackup="true"
        tools:ignore="AllowBackup">
        <service android:name=".S" android:exported="true" tools:ignore="ExportedService"/>
        <activity android:name=".A" android:exported="true"/>
    </application>
</manifest>
`

func TestFilterManifestToolsIgnoredInSource(t *testing.T) {
	cols := ignoreFindings("AndroidManifest.xml",
		scanner.Finding{Rule: "AllowBackupManifest", Line: 5},
		scanner.Finding{Rule: "ExportedServiceManifest", Line: 7},
		scanner.Finding{Rule: "ExportedWithoutPermission", Line: 8},
	)
	got := filterManifestToolsIgnoredInSource(cols, "AndroidManifest.xml", []byte(manifestIgnoreSample))
	if got.Len() != 1 || got.LineAt(0) != 8 {
		t.Fatalf("expected only the unignored activity finding on line 8, got %d findings", got.Len())
	}
}

func TestFilterManifestToolsIgnoredInSourceRootAppliesToEverything(t *testing.T) {
	src := `<?xml version="1.0"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" tools:ignore="all">
    <application android:allowBackup="true"/>
</manifest>
`
	cols := ignoreFindings("AndroidManifest.xml",
		scanner.Finding{Rule: "AllowBackupManifest", Line: 3},
		scanner.Finding{Rule: "GoogleAppIndexingWarningManifest", Line: 1},
	)
	if got := filterManifestToolsIgnoredInSource(cols, "AndroidManifest.xml", []byte(src)); got.Len() != 0 {
		t.Fatalf("expected tools:ignore=all on <manifest> to drop everything, got %d", got.Len())
	}
}

func TestFilterManifestToolsIgnoredFromParsedSource(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" tools:ignore="MissingVersion">
  <uses-permission android:name="android.permission.REBOOT" tools:ignore="SystemPermission"/>
  <uses-permission android:name="android.permission.DUMP"/>
  <application>
    <service android:name=".Svc" android:exported="true" tools:ignore="ExportedService"/>
    <service android:name=".Other" android:exported="true"/>
  </application>
</manifest>
`)
	cols := ignoreFindings("AndroidManifest.xml",
		scanner.Finding{Rule: "MissingVersionManifest", Line: 1},  // root tools:ignore via the AOSP id
		scanner.Finding{Rule: "SystemPermission", Line: 3},        // element tools:ignore by rule id
		scanner.Finding{Rule: "SystemPermission", Line: 4},        // sibling without ignore
		scanner.Finding{Rule: "ExportedServiceManifest", Line: 6}, // ignored through the ExportedService id
		scanner.Finding{Rule: "ExportedServiceManifest", Line: 7}, // other service
		scanner.Finding{Rule: "UnsupportedChromeOsHardwareManifest", Line: 6},
	)
	got := filterManifestToolsIgnoredInSource(cols, "AndroidManifest.xml", src)
	if kept := keptLines(got); !equalLines(kept, []int{4, 6, 7}) {
		t.Fatalf("expected lines 4, 6, 7 to remain, got %v", kept)
	}
}

func TestFilterManifestToolsIgnoredCombinesIndexAndSource(t *testing.T) {
	// The element-line index works without source text; the source scan adds later lines of multi-line elements.
	path := writeIgnoreFixture(t, "AndroidManifest.xml", manifestIgnoreSample)
	m := &manifest.Manifest{ToolsIgnoreByLine: map[int][]string{7: {"ExportedService"}}}
	cols := ignoreFindings(path,
		scanner.Finding{Rule: "ExportedServiceManifest", Line: 7}, // line index
		scanner.Finding{Rule: "AllowBackupManifest", Line: 5},     // source: inside the multi-line <application>
		scanner.Finding{Rule: "ExportedWithoutPermission", Line: 8},
	)
	if got := filterManifestToolsIgnored(cols, m, path, nil); !equalLines(keptLines(got), []int{8}) {
		t.Fatalf("expected only line 8 to remain, got %v", keptLines(got))
	}
	if got := filterManifestToolsIgnored(cols, m, "", nil); !equalLines(keptLines(got), []int{5, 8}) {
		t.Fatalf("without source only the line index applies, got %v", keptLines(got))
	}
}

// --- values and xml/ resources ---

func TestFilterValuesToolsIgnoredAttributeLines(t *testing.T) {
	path := writeIgnoreFixture(t, "values/strings.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources xmlns:tools="http://schemas.android.com/tools">
  <string name="a">%s %s</string>
  <string name="b" tools:ignore="StringResourceMissingPositional">%s %s</string>
  <plurals name="p" tools:ignore="UnusedQuantity"><item quantity="few">x</item></plurals>
</resources>
`)
	cols := ignoreFindings(path,
		scanner.Finding{Rule: "StringResourceMissingPositional", Line: 3},
		scanner.Finding{Rule: "StringResourceMissingPositional", Line: 4},
		scanner.Finding{Rule: "UnusedQuantityResource", Line: 5},
	)
	got := filterValuesToolsIgnored(cols)
	if got.Len() != 1 || got.LineAt(0) != 3 {
		t.Fatalf("expected only the line 3 finding to remain, got %d rows", got.Len())
	}
}

func TestFilterValuesToolsIgnored(t *testing.T) {
	path := writeIgnoreFixture(t, "res/values/strings.xml", `<resources xmlns:tools="http://schemas.android.com/tools">
    <plurals name="a" tools:ignore="MissingQuantity"><item quantity="one">x</item></plurals>
    <plurals name="b"><item quantity="one">x</item></plurals>
    <plurals name="c" tools:ignore="Other"><item quantity="one">x</item></plurals>
</resources>
`)
	cols := ignoreFindings(path,
		scanner.Finding{Rule: "MissingQuantityResource", Line: 2},
		scanner.Finding{Rule: "MissingQuantityResource", Line: 3},
		scanner.Finding{Rule: "MissingQuantityResource", Line: 4},
	)
	if got := filterValuesToolsIgnored(cols); !equalLines(keptLines(got), []int{3, 4}) {
		t.Fatalf("expected findings on lines 3 and 4 to remain, got %v", keptLines(got))
	}
}

func TestFilterValuesToolsIgnoredRootResourcesElement(t *testing.T) {
	root := writeIgnoreFixture(t, "values/root.xml", `<resources xmlns:tools="http://schemas.android.com/tools" tools:ignore="MissingQuantity">
    <string name="e">A -- B</string>
    <string name="f">A -- B</string>
</resources>
`)
	all := writeIgnoreFixture(t, "values/all.xml", `<resources xmlns:tools="http://schemas.android.com/tools" tools:ignore="all">
    <string name="g">A -- B</string>
</resources>
`)
	other := writeIgnoreFixture(t, "values/other.xml", `<resources xmlns:tools="http://schemas.android.com/tools" tools:ignore="Other">
    <string name="h">A -- B</string>
</resources>
`)
	cols := scanner.CollectFindings([]scanner.Finding{
		{File: root, Line: 2, Col: 1, Rule: "MissingQuantityResource", RuleSet: "android-lint"},
		{File: root, Line: 3, Col: 1, Rule: "MissingQuantityResource", RuleSet: "android-lint"},
		{File: all, Line: 2, Col: 1, Rule: "MissingQuantityResource", RuleSet: "android-lint"},
		{File: other, Line: 2, Col: 1, Rule: "MissingQuantityResource", RuleSet: "android-lint"},
	})
	got := filterValuesToolsIgnored(cols)
	if got.Len() != 1 || got.FileAt(0) != other {
		t.Fatalf("a root <resources> ignore covers its children; only the unrelated one remains, got %d rows", got.Len())
	}
}

func TestFilterValuesToolsIgnoredCoversXMLFoldersAndLaterLines(t *testing.T) {
	path := writeIgnoreFixture(t, "xml/automotive_app_desc.xml", `<automotiveApp xmlns:tools="http://schemas.android.com/tools">
    <uses
        tools:ignore="InvalidUsesTagAttribute"
        name="bogus" />
    <uses
        name="bogus" />
</automotiveApp>
`)
	cols := ignoreFindings(path,
		scanner.Finding{Rule: "InvalidUsesTagAttribute", Line: 4}, // attribute on a later line of the ignoring element
		scanner.Finding{Rule: "InvalidUsesTagAttribute", Line: 6},
	)
	if got := filterValuesToolsIgnored(cols); !equalLines(keptLines(got), []int{6}) {
		t.Fatalf("expected only line 6 to remain, got %v", keptLines(got))
	}
}

func TestFilterValuesToolsIgnoredIgnoresOtherFiles(t *testing.T) {
	// Only files in res/ folders are read; a Kotlin finding is left alone.
	cols := ignoreFindings("src/Main.kt", scanner.Finding{Rule: "HardcodedValuesResource", Line: 3})
	if got := filterValuesToolsIgnored(cols); got.Len() != 1 {
		t.Fatalf("expected the finding to remain, got %d rows", got.Len())
	}
}

// --- layouts ---

func TestFilterLayoutToolsIgnoredFromViewModel(t *testing.T) {
	root := &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{"tools:ignore": "HardcodedText"}, Children: []*android.View{
		{Type: "TextView", Line: 2, Attributes: map[string]string{}},
		{Type: "LinearLayout", Line: 3, Attributes: map[string]string{}, Children: []*android.View{
			{Type: "TextView", Line: 4, Attributes: map[string]string{}},
		}},
	}}
	other := &android.View{Type: "LinearLayout", Line: 1, Attributes: map[string]string{}, Children: []*android.View{
		{Type: "TextView", Line: 2, Attributes: map[string]string{"tools:ignore": "Foo, all"}},
		{Type: "TextView", Line: 3, Attributes: map[string]string{}},
	}}
	idx := &android.ResourceIndex{Layouts: map[string]*android.Layout{
		"a": {Name: "a", FilePath: "res/layout/a.xml", RootView: root},
		"b": {Name: "b", FilePath: "res/layout/b.xml", RootView: other},
	}}
	cols := scanner.CollectFindings([]scanner.Finding{
		{File: "res/layout/a.xml", Line: 2, Rule: "HardcodedValuesResource", RuleSet: "android-lint"}, // child of an ignoring parent
		{File: "res/layout/a.xml", Line: 4, Rule: "HardcodedValuesResource", RuleSet: "android-lint"}, // grandchild
		{File: "res/layout/b.xml", Line: 2, Rule: "HardcodedValuesResource", RuleSet: "android-lint"}, // ignore=all in a list
		{File: "res/layout/b.xml", Line: 3, Rule: "HardcodedValuesResource", RuleSet: "android-lint"},
	})
	got := filterLayoutToolsIgnored(cols, idx)
	if got.Len() != 1 || got.FileAt(0) != "res/layout/b.xml" || got.LineAt(0) != 3 {
		t.Fatalf("expected only b.xml:3 to remain, got %d rows", got.Len())
	}
}

func TestFilterValuesToolsIgnoredCoversLayoutFilesOnDisk(t *testing.T) {
	path := writeIgnoreFixture(t, "res/layout/main.xml", `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools">
    <TextView
        tools:ignore="hardcodedtext"
        android:layout_width="wrap_content"
        android:text="multi-line view" />
    <LinearLayout tools:ignore="HardcodedText">
        <TextView
            android:layout_width="wrap_content"
            android:text="nested" />
    </LinearLayout>
    <TextView
        android:layout_width="wrap_content"
        android:text="plain" />
</LinearLayout>
`)
	cols := ignoreFindings(path,
		scanner.Finding{Rule: "HardcodedValuesResource", Line: 6},  // attribute on a later line of an ignoring view
		scanner.Finding{Rule: "HardcodedValuesResource", Line: 10}, // nested under an ignoring ancestor
		scanner.Finding{Rule: "HardcodedValuesResource", Line: 14},
	)
	if got := filterValuesToolsIgnored(cols); !equalLines(keptLines(got), []int{14}) {
		t.Fatalf("expected only line 14 to remain, got %v", keptLines(got))
	}
}
