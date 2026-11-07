package rules_test

import (
	"strings"
	"testing"
)

// Expected findings were produced by running Android Lint 32.4.1 on the same files.
func TestUnknownIdFamilyLintParity(t *testing.T) {
	const ns = `xmlns:android="http://schemas.android.com/apk/res/android"`
	const wh = ` android:layout_width="wrap_content" android:layout_height="wrap_content"`
	files := map[string]string{
		"layout/a.xml": `<RelativeLayout ` + ns + wh + `>
<TextView android:id="@+id/one"` + wh + `/>
<TextView android:id="@+id/two" android:layout_below="@+id/one"` + wh + `/>
<TextView android:id="@+id/four" android:layout_below="@+id/missing2"` + wh + `/>
<TextView android:id="@+id/five" android:layout_below="@+id/other"` + wh + `/>
<TextView android:id="@+id/six" android:layout_below="@+id/six"` + wh + `/>
<TextView android:id="@+id/seven" android:layout_below="@id/other"` + wh + `/>
</RelativeLayout>`,
		"layout/b.xml": `<LinearLayout ` + ns + wh + `>
<TextView android:id="@+id/other"` + wh + `/>
</LinearLayout>`,
	}
	join := func(rule string) string { return strings.Join(resourceFindings(t, rule, files), "\n") }
	if got, want := join("UnknownIdInLayout"), "layout/a.xml:05 The id \"`other`\" is not referring to any views in this layout"; got != want {
		t.Errorf("UnknownIdInLayout:\ngot  %s\nwant %s", got, want)
	}
	want := strings.Join([]string{
		"layout/a.xml:06 Cannot be relative to self: id=six, layout_below=six",
		"layout/a.xml:07 `@id/other` is not a sibling in the same `RelativeLayout`",
	}, "\n")
	if got := join("NotSiblingResource"); got != want {
		t.Errorf("NotSibling:\ngot  %s\nwant %s", got, want)
	}
}
