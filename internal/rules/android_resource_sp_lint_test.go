package rules_test

import (
	"strings"
	"testing"
)

// Expected findings were produced by running Android Lint 32.4.1 on the same files.
func TestSmallSpAndSpUsageLintParity(t *testing.T) {
	files := map[string]string{
		"values/v.xml": `<resources>
  <dimen name="dpdim">14dp</dimen>
  <style name="S"><item name="textSize">9sp</item></style>
  <style name="S2"><item name="android:textSize">9sp</item></style>
  <style name="S3"><item name="textSize">14dip</item></style>
</resources>`,
		"layout/l.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:textSize="10sp"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:textSize="11sp"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:textSize="10.9sp"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:textSize="0sp"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:textSize="14dip"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:textSize="-14dp"/>
  <View android:layout_width="wrap_content" android:layout_height="10sp"/>
</LinearLayout>`,
	}
	small := strings.Join(resourceFindings(t, "SmallSpResource", files), "\n")
	wantSmall := strings.Join([]string{
		"layout/l.xml:02 Avoid using sizes smaller than `11sp`: `10sp`",
		"layout/l.xml:04 Avoid using sizes smaller than `11sp`: `10.9sp`",
		"layout/l.xml:08 Avoid using sizes smaller than `11sp`: `10sp`",
		"values/v.xml:03 Avoid using sizes smaller than `11sp`: `9sp`",
	}, "\n")
	if small != wantSmall {
		t.Fatalf("SmallSp got:\n%s\nwant:\n%s", small, wantSmall)
	}
	sp := strings.Join(resourceFindings(t, "SpUsageResource", files), "\n")
	const m = "Should use \"`sp`\" instead of \"`dp`\" for text sizes"
	wantSp := "layout/l.xml:06 " + m + "\nvalues/v.xml:05 " + m
	if sp != wantSp {
		t.Fatalf("SpUsage got:\n%s\nwant:\n%s", sp, wantSp)
	}
}
