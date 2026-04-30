package android

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// setupTestResDir creates a temporary res/ directory with sample XML fixtures.
func setupTestResDir(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	resDir := filepath.Join(root, "res")

	// layout/
	layoutDir := filepath.Join(resDir, "layout")
	os.MkdirAll(layoutDir, 0o755)

	writeFile(t, filepath.Join(layoutDir, "activity_main.xml"), `<?xml version="1.0" encoding="utf-8"?>
<LinearLayout
    xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="match_parent"
    android:orientation="vertical"
    android:background="@color/white">

    <TextView
        android:id="@+id/title"
        android:layout_width="wrap_content"
        android:layout_height="wrap_content"
        android:text="Hello World" />

    <ImageView
        android:id="@+id/icon"
        android:layout_width="48dp"
        android:layout_height="48dp"
        android:src="@drawable/ic_launcher" />

    <Button
        android:id="@+id/submit"
        android:layout_width="match_parent"
        android:layout_height="wrap_content"
        android:text="@string/submit" />

</LinearLayout>
`)

	writeFile(t, filepath.Join(layoutDir, "nested_scroll.xml"), `<?xml version="1.0" encoding="utf-8"?>
<ScrollView
    xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="match_parent">

    <LinearLayout
        android:layout_width="match_parent"
        android:layout_height="wrap_content">

        <ScrollView
            android:layout_width="match_parent"
            android:layout_height="200dp">

            <TextView
                android:layout_width="match_parent"
                android:layout_height="wrap_content"
                android:text="Nested content" />

        </ScrollView>

    </LinearLayout>

</ScrollView>
`)

	writeFile(t, filepath.Join(layoutDir, "single_child.xml"), `<?xml version="1.0" encoding="utf-8"?>
<FrameLayout
    xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="match_parent">

    <LinearLayout
        android:layout_width="match_parent"
        android:layout_height="match_parent">

        <TextView
            android:layout_width="wrap_content"
            android:layout_height="wrap_content"
            android:text="@string/hello" />

    </LinearLayout>

</FrameLayout>
`)

	// values/
	valuesDir := filepath.Join(resDir, "values")
	os.MkdirAll(valuesDir, 0o755)

	writeFile(t, filepath.Join(valuesDir, "strings.xml"), `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">My Application</string>
    <string name="submit">Submit</string>
    <string name="cancel">Cancel</string>
    <string name="hello">Hello, %s!</string>

    <string-array name="planets">
        <item>Mercury</item>
        <item>Venus</item>
        <item>Earth</item>
    </string-array>

    <plurals name="items_count">
        <item quantity="one">%d item</item>
        <item quantity="other">%d items</item>
    </plurals>
</resources>
`)

	writeFile(t, filepath.Join(valuesDir, "colors.xml"), `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <color name="primary">#6200EE</color>
    <color name="primary_dark">#3700B3</color>
    <color name="accent">#03DAC5</color>
    <color name="white">#FFFFFF</color>
</resources>
`)

	writeFile(t, filepath.Join(valuesDir, "dimens.xml"), `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <dimen name="margin_small">8dp</dimen>
    <dimen name="margin_medium">16dp</dimen>
    <dimen name="text_size_body">14sp</dimen>
</resources>
`)

	writeFile(t, filepath.Join(valuesDir, "styles.xml"), `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <style name="AppTheme" parent="Theme.MaterialComponents.Light.DarkActionBar">
        <item name="colorPrimary">@color/primary</item>
        <item name="colorPrimaryDark">@color/primary_dark</item>
        <item name="colorAccent">@color/accent</item>
    </style>

    <style name="AppTheme.NoActionBar">
        <item name="windowActionBar">false</item>
        <item name="windowNoTitle">true</item>
    </style>
</resources>
`)

	writeFile(t, filepath.Join(valuesDir, "integers.xml"), `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <integer name="max_retries">3</integer>
    <bool name="is_tablet">false</bool>
</resources>
`)

	// drawable/
	drawableDir := filepath.Join(resDir, "drawable")
	os.MkdirAll(drawableDir, 0o755)
	writeFile(t, filepath.Join(drawableDir, "ic_launcher.xml"), `<?xml version="1.0" encoding="utf-8"?>
<vector xmlns:android="http://schemas.android.com/apk/res/android"
    android:width="24dp"
    android:height="24dp"
    android:viewportWidth="24"
    android:viewportHeight="24">
    <path android:fillColor="#000" android:pathData="M12,2L2,22h20z"/>
</vector>
`)
	writeFile(t, filepath.Join(drawableDir, "bg_rounded.xml"), `<?xml version="1.0" encoding="utf-8"?>
<shape xmlns:android="http://schemas.android.com/apk/res/android">
    <corners android:radius="8dp"/>
    <solid android:color="@color/white"/>
</shape>
`)

	return resDir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("writing %s: %v", path, err)
	}
}

func writeValuesFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	writeFile(t, path, content)
	return path
}

func TestScanResourceDir(t *testing.T) {
	resDir := setupTestResDir(t)
	idx, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatalf("ScanResourceDir: %v", err)
	}

	// Verify layouts
	if len(idx.Layouts) != 3 {
		t.Errorf("expected 3 layouts, got %d", len(idx.Layouts))
	}
	if _, ok := idx.Layouts["activity_main"]; !ok {
		t.Error("missing layout: activity_main")
	}
	if _, ok := idx.Layouts["nested_scroll"]; !ok {
		t.Error("missing layout: nested_scroll")
	}

	// Verify strings
	if idx.Strings["app_name"] != "My Application" {
		t.Errorf("expected string app_name='My Application', got %q", idx.Strings["app_name"])
	}
	if idx.Strings["submit"] != "Submit" {
		t.Errorf("expected string submit='Submit', got %q", idx.Strings["submit"])
	}
	if len(idx.Strings) != 4 {
		t.Errorf("expected 4 strings, got %d", len(idx.Strings))
	}

	// Verify colors
	if idx.Colors["primary"] != "#6200EE" {
		t.Errorf("expected color primary='#6200EE', got %q", idx.Colors["primary"])
	}
	if len(idx.Colors) != 4 {
		t.Errorf("expected 4 colors, got %d", len(idx.Colors))
	}

	// Verify dimensions
	if idx.Dimensions["margin_small"] != "8dp" {
		t.Errorf("expected dimen margin_small='8dp', got %q", idx.Dimensions["margin_small"])
	}
	if len(idx.Dimensions) != 3 {
		t.Errorf("expected 3 dimensions, got %d", len(idx.Dimensions))
	}

	// Verify styles
	if len(idx.Styles) != 2 {
		t.Errorf("expected 2 styles, got %d", len(idx.Styles))
	}
	appTheme := idx.Styles["AppTheme"]
	if appTheme == nil {
		t.Fatal("missing style: AppTheme")
	}
	if appTheme.Parent != "Theme.MaterialComponents.Light.DarkActionBar" {
		t.Errorf("expected AppTheme parent, got %q", appTheme.Parent)
	}
	if appTheme.Items["colorPrimary"] != "@color/primary" {
		t.Errorf("expected colorPrimary item, got %q", appTheme.Items["colorPrimary"])
	}

	// Verify drawables
	if len(idx.Drawables) != 2 {
		t.Errorf("expected 2 drawables, got %d", len(idx.Drawables))
	}

	// Verify string arrays
	planets := idx.StringArrays["planets"]
	if len(planets) != 3 {
		t.Errorf("expected 3 planets, got %d", len(planets))
	}
	if len(planets) >= 3 && planets[2] != "Earth" {
		t.Errorf("expected planets[2]='Earth', got %q", planets[2])
	}

	// Verify plurals
	itemsCount := idx.Plurals["items_count"]
	if itemsCount == nil {
		t.Fatal("missing plural: items_count")
	}
	if itemsCount["one"] != "%d item" {
		t.Errorf("expected plural one='%%d item', got %q", itemsCount["one"])
	}
	if itemsCount["other"] != "%d items" {
		t.Errorf("expected plural other='%%d items', got %q", itemsCount["other"])
	}

	// Verify integers and booleans
	if idx.Integers["max_retries"] != "3" {
		t.Errorf("expected integer max_retries='3', got %q", idx.Integers["max_retries"])
	}
	if idx.Booleans["is_tablet"] != "false" {
		t.Errorf("expected bool is_tablet='false', got %q", idx.Booleans["is_tablet"])
	}

	// Verify IDs
	if !idx.IDs["title"] {
		t.Error("missing ID: title")
	}
	if !idx.IDs["icon"] {
		t.Error("missing ID: icon")
	}
	if !idx.IDs["submit"] {
		t.Error("missing ID: submit")
	}
}

func TestLayoutParsing(t *testing.T) {
	resDir := setupTestResDir(t)
	idx, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatalf("ScanResourceDir: %v", err)
	}

	layout := idx.Layouts["activity_main"]
	if layout == nil {
		t.Fatal("missing layout: activity_main")
	}

	root := layout.RootView
	if root.Type != "LinearLayout" {
		t.Errorf("expected root type LinearLayout, got %s", root.Type)
	}
	if root.Line != 2 {
		t.Errorf("expected root line 2, got %d", root.Line)
	}
	if root.LayoutWidth != "match_parent" {
		t.Errorf("expected layout_width match_parent, got %s", root.LayoutWidth)
	}
	if root.Background != "@color/white" {
		t.Errorf("expected background @color/white, got %s", root.Background)
	}

	if len(root.Children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(root.Children))
	}

	// TextView
	tv := root.Children[0]
	if tv.Type != "TextView" {
		t.Errorf("expected child 0 type TextView, got %s", tv.Type)
	}
	if tv.ID != "@+id/title" {
		t.Errorf("expected id @+id/title, got %s", tv.ID)
	}
	if tv.Text != "Hello World" {
		t.Errorf("expected text 'Hello World', got %q", tv.Text)
	}
	if tv.Line != 9 {
		t.Errorf("expected TextView line 9, got %d", tv.Line)
	}

	// ImageView
	iv := root.Children[1]
	if iv.Type != "ImageView" {
		t.Errorf("expected child 1 type ImageView, got %s", iv.Type)
	}
	if iv.ContentDescription != "" {
		t.Errorf("expected no contentDescription, got %q", iv.ContentDescription)
	}

	// Button
	btn := root.Children[2]
	if btn.Type != "Button" {
		t.Errorf("expected child 2 type Button, got %s", btn.Type)
	}
	if btn.Text != "@string/submit" {
		t.Errorf("expected text '@string/submit', got %q", btn.Text)
	}
	if btn.Line != 21 {
		t.Errorf("expected Button line 21, got %d", btn.Line)
	}
}

func TestHardcodedText(t *testing.T) {
	resDir := setupTestResDir(t)
	idx, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatalf("ScanResourceDir: %v", err)
	}

	layout := idx.Layouts["activity_main"]
	tv := layout.RootView.Children[0]
	btn := layout.RootView.Children[2]

	if !tv.HasHardcodedText() {
		t.Error("expected hardcoded text on TextView with 'Hello World'")
	}
	if btn.HasHardcodedText() {
		t.Error("Button with @string/submit should not be hardcoded text")
	}
}

func TestMissingContentDescription(t *testing.T) {
	resDir := setupTestResDir(t)
	idx, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatalf("ScanResourceDir: %v", err)
	}

	layout := idx.Layouts["activity_main"]
	iv := layout.RootView.Children[1]

	if iv.HasContentDescription() {
		t.Error("ImageView should not have contentDescription set")
	}
}

func TestNestedScrollDetection(t *testing.T) {
	resDir := setupTestResDir(t)
	idx, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatalf("ScanResourceDir: %v", err)
	}

	layout := idx.Layouts["nested_scroll"]
	scrollViews := layout.FindViewsByType("ScrollView")
	if len(scrollViews) != 2 {
		t.Errorf("expected 2 ScrollViews, got %d", len(scrollViews))
	}
	for _, sv := range scrollViews {
		if !IsScrollableView(sv.Type) {
			t.Errorf("expected %s to be scrollable", sv.Type)
		}
	}
}

func TestViewCountAndDepth(t *testing.T) {
	resDir := setupTestResDir(t)
	idx, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatalf("ScanResourceDir: %v", err)
	}

	main := idx.Layouts["activity_main"]
	if main.ViewCount() != 4 {
		t.Errorf("expected 4 views in activity_main, got %d", main.ViewCount())
	}
	if main.MaxDepth() != 2 {
		t.Errorf("expected max depth 2 in activity_main, got %d", main.MaxDepth())
	}

	nested := idx.Layouts["nested_scroll"]
	if nested.MaxDepth() != 4 {
		t.Errorf("expected max depth 4 in nested_scroll, got %d", nested.MaxDepth())
	}
}

func TestUselessParent(t *testing.T) {
	resDir := setupTestResDir(t)
	idx, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatalf("ScanResourceDir: %v", err)
	}

	// single_child layout has FrameLayout -> LinearLayout -> TextView
	layout := idx.Layouts["single_child"]
	root := layout.RootView

	if len(root.Children) != 1 {
		t.Fatalf("expected 1 child on root, got %d", len(root.Children))
	}
	if !IsLayoutView(root.Type) {
		t.Errorf("expected root %s to be a layout view", root.Type)
	}
	child := root.Children[0]
	if !IsLayoutView(child.Type) {
		t.Errorf("expected child %s to be a layout view", child.Type)
	}
	// A FrameLayout with a single layout child is a candidate for UselessParent
	if len(child.Children) != 1 {
		t.Errorf("expected single-child LinearLayout to have 1 child, got %d", len(child.Children))
	}
}

func TestScanResourceDir_NotExist(t *testing.T) {
	_, err := ScanResourceDir("/nonexistent/res")
	if err == nil {
		t.Error("expected error for nonexistent directory")
	}
}

func TestScanResourceDir_NotDir(t *testing.T) {
	tmp := t.TempDir()
	f := filepath.Join(tmp, "notadir")
	os.WriteFile(f, []byte("x"), 0o644)
	_, err := ScanResourceDir(f)
	if err == nil {
		t.Error("expected error for non-directory path")
	}
}

func TestParseValuesFile_StreamingParserPreservesResourceData(t *testing.T) {
	tmp := t.TempDir()
	valuesDir := filepath.Join(tmp, "values")
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatalf("mkdir values dir: %v", err)
	}

	writeValuesFile(t, valuesDir, "mixed.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="headline">  Hello world  </string>
    <style name="AppTheme" parent="Theme.App">
        <item name="colorPrimary">@color/primary</item>
        <item name="windowActionBar">false</item>
    </style>
    <string-array name="planets">
        <item> Mercury </item>
        <item>Venus</item>
    </string-array>
    <plurals name="songs">
        <item quantity="one">%d song</item>
        <item quantity="other">%d songs</item>
    </plurals>
    <integer name="max_retries">7</integer>
    <bool name="enabled">true</bool>
    <color name="accent">#00FF00</color>
    <dimen name="gutter">12dp</dimen>
</resources>
`)

	idx := &ResourceIndex{
		Strings:      make(map[string]string),
		Colors:       make(map[string]string),
		Dimensions:   make(map[string]string),
		Styles:       make(map[string]*Style),
		StringArrays: make(map[string][]string),
		Plurals:      make(map[string]map[string]string),
		Integers:     make(map[string]string),
		Booleans:     make(map[string]string),
	}
	if _, err := idx.scanValuesDir(valuesDir, 1); err != nil {
		t.Fatalf("scanValuesDir: %v", err)
	}

	if got := idx.Strings["headline"]; got != "Hello world" {
		t.Fatalf("expected trimmed string, got %q", got)
	}
	if got := idx.Colors["accent"]; got != "#00FF00" {
		t.Fatalf("expected color, got %q", got)
	}
	if got := idx.Dimensions["gutter"]; got != "12dp" {
		t.Fatalf("expected dimen, got %q", got)
	}
	if got := idx.Integers["max_retries"]; got != "7" {
		t.Fatalf("expected integer, got %q", got)
	}
	if got := idx.Booleans["enabled"]; got != "true" {
		t.Fatalf("expected bool, got %q", got)
	}
	if got := idx.StringArrays["planets"]; len(got) != 2 || got[0] != "Mercury" || got[1] != "Venus" {
		t.Fatalf("unexpected string-array contents: %#v", got)
	}
	if got := idx.Plurals["songs"]; got["one"] != "%d song" || got["other"] != "%d songs" {
		t.Fatalf("unexpected plurals contents: %#v", got)
	}
	style := idx.Styles["AppTheme"]
	if style == nil {
		t.Fatal("expected AppTheme style")
	}
	if style.Parent != "Theme.App" {
		t.Fatalf("expected parent Theme.App, got %q", style.Parent)
	}
	if style.Items["colorPrimary"] != "@color/primary" || style.Items["windowActionBar"] != "false" {
		t.Fatalf("unexpected style items: %#v", style.Items)
	}
}

func TestParseValuesFile_TracksExtraTextLine(t *testing.T) {
	tmp := t.TempDir()
	valuesDir := filepath.Join(tmp, "values")
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatalf("mkdir values dir: %v", err)
	}

	writeValuesFile(t, valuesDir, "extra_text.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="hello">Hello</string>
    stray text
    <string name="goodbye">Goodbye</string>
</resources>
`)

	idx := &ResourceIndex{
		Strings:      make(map[string]string),
		Colors:       make(map[string]string),
		Dimensions:   make(map[string]string),
		Styles:       make(map[string]*Style),
		StringArrays: make(map[string][]string),
		Plurals:      make(map[string]map[string]string),
		Integers:     make(map[string]string),
		Booleans:     make(map[string]string),
	}
	if _, err := idx.scanValuesDir(valuesDir, 1); err != nil {
		t.Fatalf("scanValuesDir: %v", err)
	}

	if len(idx.ExtraTexts) != 1 {
		t.Fatalf("expected 1 extra text entry, got %d", len(idx.ExtraTexts))
	}
	if got := idx.ExtraTexts[0].Text; got != "stray text" {
		t.Fatalf("expected stray text entry, got %q", got)
	}
	if got := idx.ExtraTexts[0].Line; got != 4 {
		t.Fatalf("expected stray text on line 4, got %d", got)
	}
}

func TestParseValuesFile_RejectsNonResourcesRoot(t *testing.T) {
	tmp := t.TempDir()
	valuesDir := filepath.Join(tmp, "values")
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatalf("mkdir values dir: %v", err)
	}

	writeValuesFile(t, valuesDir, "bad.xml", `<not-resources><string name="bad">x</string></not-resources>`)

	idx := &ResourceIndex{
		Strings:      make(map[string]string),
		Colors:       make(map[string]string),
		Dimensions:   make(map[string]string),
		Styles:       make(map[string]*Style),
		StringArrays: make(map[string][]string),
		Plurals:      make(map[string]map[string]string),
		Integers:     make(map[string]string),
		Booleans:     make(map[string]string),
	}
	_, err := idx.scanValuesDir(valuesDir, 1)
	if err == nil {
		t.Fatal("expected error for non-resources root")
	}
	if !strings.Contains(err.Error(), "root element is not <resources>") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestScanResourceDir_ValuesLastWriteWinsInFilenameOrder(t *testing.T) {
	tmp := t.TempDir()
	resDir := filepath.Join(tmp, "res")
	valuesDir := filepath.Join(resDir, "values")
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatalf("mkdir values dir: %v", err)
	}

	writeValuesFile(t, valuesDir, "a.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Alpha</string>
</resources>
`)
	writeValuesFile(t, valuesDir, "b.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Beta</string>
</resources>
`)

	idx, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatalf("ScanResourceDir: %v", err)
	}
	if got := idx.Strings["app_name"]; got != "Beta" {
		t.Fatalf("expected later file to win, got %q", got)
	}
}

func TestLazyValuesScan_LoadIntoPreservesValuesSemantics(t *testing.T) {
	tmp := t.TempDir()
	valuesDir := filepath.Join(tmp, "values")
	if err := os.MkdirAll(valuesDir, 0o755); err != nil {
		t.Fatalf("mkdir values dir: %v", err)
	}

	writeValuesFile(t, valuesDir, "a.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Alpha</string>
    <bool name="enabled">true</bool>
</resources>
`)
	writeValuesFile(t, valuesDir, "b.xml", `<?xml version="1.0" encoding="utf-8"?>
<resources>
    <string name="app_name">Beta</string>
    stray text
</resources>
`)

	provider := NewLazyValuesScan(valuesDir, 1)
	provider.Start()

	first := newResourceIndex()
	stats, err := provider.LoadInto(first)
	if err != nil {
		t.Fatalf("LoadInto first: %v", err)
	}
	if got := first.Strings["app_name"]; got != "Beta" {
		t.Fatalf("expected later file to win, got %q", got)
	}
	if got := first.Booleans["enabled"]; got != "true" {
		t.Fatalf("expected bool resource, got %q", got)
	}
	if len(first.ExtraTexts) != 1 || first.ExtraTexts[0].Text != "stray text" {
		t.Fatalf("unexpected extra texts: %#v", first.ExtraTexts)
	}
	if stats.ValuesReadMs < 0 || stats.ValuesParseMs < 0 || stats.ValuesIndexMs < 0 {
		t.Fatalf("expected non-negative stats, got %#v", stats)
	}

	second := newResourceIndex()
	stats2, err := provider.LoadInto(second)
	if err != nil {
		t.Fatalf("LoadInto second: %v", err)
	}
	if got := second.Strings["app_name"]; got != "Beta" {
		t.Fatalf("expected cached provider to remain reusable, got %q", got)
	}
	if stats != stats2 {
		t.Fatalf("expected stable cached stats, got %#v vs %#v", stats, stats2)
	}
}

func TestIsScrollableView(t *testing.T) {
	scrollable := []string{"ScrollView", "HorizontalScrollView", "NestedScrollView",
		"androidx.core.widget.NestedScrollView"}
	for _, v := range scrollable {
		if !IsScrollableView(v) {
			t.Errorf("expected %s to be scrollable", v)
		}
	}
	if IsScrollableView("LinearLayout") {
		t.Error("LinearLayout should not be scrollable")
	}
}

func TestScanResourceDir_ParsesDrawableSelectors(t *testing.T) {
	tmp := t.TempDir()
	resDir := filepath.Join(tmp, "res")
	drawableDir := filepath.Join(resDir, "drawable")
	os.MkdirAll(drawableDir, 0o755)

	writeFile(t, filepath.Join(drawableDir, "button.xml"), `<?xml version="1.0" encoding="utf-8"?>
<selector xmlns:android="http://schemas.android.com/apk/res/android">
    <item android:drawable="@drawable/normal" />
    <item android:state_pressed="true" android:drawable="@drawable/pressed" />
</selector>
`)

	idx, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatalf("ScanResourceDir: %v", err)
	}
	items := idx.DrawableSelectors["button"]
	if len(items) != 2 {
		t.Fatalf("expected 2 selector items, got %d", len(items))
	}
	if items[0].Line != 3 {
		t.Fatalf("first item line = %d, want 3", items[0].Line)
	}
	if len(items[0].StateAttrs) != 0 {
		t.Fatalf("first item state attrs = %#v, want none", items[0].StateAttrs)
	}
	if got := items[1].StateAttrs["android:state_pressed"]; got != "true" {
		t.Fatalf("second item pressed state = %q, want true", got)
	}
}

func TestIsLayoutView(t *testing.T) {
	layouts := []string{"LinearLayout", "RelativeLayout", "FrameLayout",
		"ConstraintLayout", "CoordinatorLayout"}
	for _, v := range layouts {
		if !IsLayoutView(v) {
			t.Errorf("expected %s to be a layout view", v)
		}
	}
	if IsLayoutView("TextView") {
		t.Error("TextView should not be a layout view")
	}
}

func TestEmptyLayout(t *testing.T) {
	tmp := t.TempDir()
	resDir := filepath.Join(tmp, "res")
	layoutDir := filepath.Join(resDir, "layout")
	os.MkdirAll(layoutDir, 0o755)

	writeFile(t, filepath.Join(layoutDir, "empty.xml"), `<?xml version="1.0" encoding="utf-8"?>
<FrameLayout
    xmlns:android="http://schemas.android.com/apk/res/android"
    android:layout_width="match_parent"
    android:layout_height="match_parent" />
`)

	idx, err := ScanResourceDir(resDir)
	if err != nil {
		t.Fatalf("ScanResourceDir: %v", err)
	}

	layout := idx.Layouts["empty"]
	if layout == nil {
		t.Fatal("missing layout: empty")
	}
	if layout.ViewCount() != 1 {
		t.Errorf("expected 1 view, got %d", layout.ViewCount())
	}
	if len(layout.RootView.Children) != 0 {
		t.Errorf("expected 0 children, got %d", len(layout.RootView.Children))
	}
}
