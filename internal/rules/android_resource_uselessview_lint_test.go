package rules_test

import (
	"strings"
	"testing"
)

// Expected findings were produced by running Android Lint 32.4.1 on the same files.
func TestUselessViewLintParity(t *testing.T) {
	files := map[string]string{"layout/uv.xml": `<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <LinearLayout android:layout_width="match_parent" android:layout_height="match_parent">
    <LinearLayout android:layout_width="match_parent" android:layout_height="match_parent">
      <TextView android:layout_width="wrap_content" android:layout_height="wrap_content"/>
      <TextView android:layout_width="wrap_content" android:layout_height="wrap_content"/>
    </LinearLayout>
  </LinearLayout>
  <RelativeLayout android:layout_width="match_parent" android:layout_height="match_parent" android:background="#fff">
    <LinearLayout android:id="@+id/x" android:layout_width="match_parent" android:layout_height="match_parent">
      <TextView android:layout_width="wrap_content" android:layout_height="wrap_content"/>
    </LinearLayout>
  </RelativeLayout>
  <LinearLayout android:layout_width="match_parent" android:layout_height="match_parent" android:background="#fff">
    <LinearLayout android:layout_width="match_parent" android:layout_height="match_parent" android:padding="4dp" android:background="#000">
      <TextView android:layout_width="wrap_content" android:layout_height="wrap_content"/>
    </LinearLayout>
  </LinearLayout>
  <ScrollView android:layout_width="match_parent" android:layout_height="match_parent">
    <LinearLayout android:layout_width="match_parent" android:layout_height="match_parent">
      <TextView android:layout_width="wrap_content" android:layout_height="wrap_content"/>
    </LinearLayout>
  </ScrollView>
  <FrameLayout android:layout_width="match_parent" android:layout_height="wrap_content"/>
  <FrameLayout android:layout_width="match_parent" android:layout_height="wrap_content" android:tag="t"/>
  <LinearLayout android:layout_width="match_parent" android:layout_height="wrap_content" style="@style/x"/>
</FrameLayout>
`, "layout/uv2.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <FrameLayout android:layout_width="match_parent" android:layout_height="wrap_content"/>
</LinearLayout>
`}
	const leaf = " view is unnecessary (no children, no `background`, no `id`, no `style`)"
	check := func(rule string, want ...string) {
		got := resourceFindings(t, rule, files)
		if strings.Join(got, "\n") != strings.Join(want, "\n") {
			t.Fatalf("%s got:\n%s\nwant:\n%s", rule, strings.Join(got, "\n"), strings.Join(want, "\n"))
		}
	}
	check("UselessLeafResource",
		"layout/uv.xml:23 This `FrameLayout`"+leaf,
		"layout/uv2.xml:02 This `FrameLayout`"+leaf)
	check("UselessParentResource",
		"layout/uv.xml:03 This `LinearLayout` layout or its `LinearLayout` parent is unnecessary",
		"layout/uv.xml:09 This `LinearLayout` layout or its `RelativeLayout` parent is possibly unnecessary; transfer the `background` attribute to the other view")
}

func TestUseCompoundDrawablesLintConditions(t *testing.T) {
	layout := func(parentAttrs, imageAttrs string, imageTag string) map[string]string {
		return map[string]string{"layout/l.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="wrap_content" ` + parentAttrs + `>
  <` + imageTag + ` android:layout_width="wrap_content" android:layout_height="wrap_content" ` + imageAttrs + `/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content"/>
</LinearLayout>`}
	}
	const want = "layout/l.xml:01 This tag and its children can be replaced by one `<TextView/>` and a compound drawable"
	cases := []struct {
		name string
		in   map[string]string
		hit  bool
	}{
		{"plain pair", layout("", "", "ImageView"), true},
		{"ImageButton is not an ImageView", layout("", "", "ImageButton"), false},
		{"weighted image", layout("", `android:layout_weight="1"`, "ImageView"), false},
		{"clickable image", layout("", `android:clickable="false"`, "ImageView"), false},
		{"scaled image", layout("", `android:scaleType="fitXY"`, "ImageView"), false},
		{"parent background", layout(`android:background="#fff"`, "", "ImageView"), false},
	}
	for _, c := range cases {
		got := resourceFindings(t, "UseCompoundDrawablesResource", c.in)
		if c.hit != (len(got) == 1 && got[0] == want) || (!c.hit && len(got) != 0) {
			t.Errorf("%s: got %v", c.name, got)
		}
	}
}

// Expected findings were produced by running Android Lint 32.4.1 on the same file.
func TestWebViewLayoutLintParity(t *testing.T) {
	files := map[string]string{"layout/w.xml": `<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="wrap_content" android:layout_height="match_parent">
  <WebView android:layout_width="match_parent" android:layout_height="match_parent"/>
  <LinearLayout android:layout_width="match_parent" android:layout_height="wrap_content">
    <WebView android:layout_width="match_parent" android:layout_height="match_parent"/>
  </LinearLayout>
  <LinearLayout android:layout_width="wrap_content" android:layout_height="wrap_content">
    <WebView android:layout_width="match_parent" android:layout_height="match_parent"/>
  </LinearLayout>
</FrameLayout>`}
	msg := func(attr string) string {
		return " Placing a `<WebView>` in a parent element that uses a `wrap_content " + attr + "` can lead to subtle bugs; use `match_parent` instead"
	}
	got := strings.Join(resourceFindings(t, "WebViewLayoutResource", files), "\n")
	want := strings.Join([]string{
		"layout/w.xml:02" + msg("layout_width"),
		"layout/w.xml:04" + msg("layout_height"),
		"layout/w.xml:07" + msg("layout_height"),
	}, "\n")
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}

// Lint's UselessViewDetector.CONTAINERS includes the scroll views, so an empty one is a useless leaf.
func TestUselessLeafScrollViews(t *testing.T) {
	files := map[string]string{"layout/sv.xml": `<FrameLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <ScrollView android:layout_width="match_parent" android:layout_height="wrap_content"/>
  <HorizontalScrollView android:layout_width="match_parent" android:layout_height="wrap_content"/>
  <ScrollView android:id="@+id/keep" android:layout_width="match_parent" android:layout_height="wrap_content"/>
</FrameLayout>
`}
	got := resourceFindings(t, "UselessLeafResource", files)
	want := []string{
		"layout/sv.xml:02 This `ScrollView` view is unnecessary (no children, no `background`, no `id`, no `style`)",
		"layout/sv.xml:03 This `HorizontalScrollView` view is unnecessary (no children, no `background`, no `id`, no `style`)",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("got:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
