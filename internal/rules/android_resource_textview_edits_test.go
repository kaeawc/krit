package rules_test

import (
	"strings"
	"testing"
)

// Expected findings were produced by running Android Lint 32.4.1 on the same file.
func TestTextViewEditsLintParity(t *testing.T) {
	wh := ` android:layout_width="wrap_content" android:layout_height="wrap_content"`
	files := map[string]string{"layout/tve.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android"` + wh + `>
<TextView` + wh + ` android:inputType="text"/>
<TextView` + wh + ` android:editable="true"/>
<TextView` + wh + ` android:editable="false"/>
<TextView` + wh + ` android:inputType="none"/>
<TextView` + wh + ` android:inputType="@string/x"/>
<TextView` + wh + ` android:autoText="true"/>
<TextView` + wh + ` android:phoneNumber="true"/>
<Button` + wh + ` android:inputType="text"/>
<CheckedTextView` + wh + ` android:inputType="text"/>
<EditText` + wh + ` android:inputType="text"/>
</LinearLayout>`}
	tv := func(attr string) string {
		return " Attribute `" + attr + "` should not be used with `<TextView>`: Change element type to `<EditText>` ?"
	}
	got := strings.Join(resourceFindings(t, "TextViewEditsResource", files), "\n")
	want := strings.Join([]string{
		"layout/tve.xml:02" + tv("inputType"),
		"layout/tve.xml:03" + tv("editable"),
		"layout/tve.xml:06" + tv("inputType"),
		"layout/tve.xml:07" + tv("autoText"),
		"layout/tve.xml:08" + tv("phoneNumber"),
		"layout/tve.xml:09 Attribute `inputType` should not be used with `<Button>`: intended for editable text widgets",
		"layout/tve.xml:10 Attribute `inputType` should not be used with `<CheckedTextView>`: intended for editable text widgets",
	}, "\n")
	if got != want {
		t.Fatalf("got:\n%s\nwant:\n%s", got, want)
	}
}
