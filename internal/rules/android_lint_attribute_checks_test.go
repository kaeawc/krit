package rules_test

import (
	"reflect"
	"testing"
)

// Expected positions come from running Android Lint 32.4.1 on the same files.
func TestInvalidManifestAttributeLintParity(t *testing.T) {
	xml := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <application android:label="x">
    <activity android:name=".A"/>
    <activity-alias android:name=".B" android:targetActivity=".A" android:launchMode="singleTop" android:theme="@style/T" android:label="l" android:exported="true"/>
    <activity-alias android:name=".C" android:targetActivity=".A" android:taskAffinity="x" tools:foo="1" xmlns:tools="http://schemas.android.com/tools"/>
    <activity-alias android:targetActivity=".A" android:launchMode="singleTop"/>
  </application>
</manifest>
`
	if got, want := manifestRulePositions(t, "InvalidManifestAttribute", xml), "5:67,5:98,6:67"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}

func TestEllipsizeMaxLinesLintParity(t *testing.T) {
	files := map[string]string{"layout/a.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:ellipsize="marquee" android:maxLines="1"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:ellipsize="end" android:maxLines="1"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:ellipsize="start" android:lines="1"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:ellipsize="start" android:lines="2" android:maxLines="1"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:ellipsize="start" android:lines="1" android:maxLines="3"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:ellipsize="start" android:maxLines="3"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:maxLines="1"/>
</LinearLayout>`}
	// Lint reports at the lines attribute when present, otherwise at maxLines, even when only the other one is 1.
	want := []string{"layout/a.xml:2:114", "layout/a.xml:4:112", "layout/a.xml:5:112", "layout/a.xml:6:112"}
	if got := resourcePositions(t, "EllipsizeMaxLines", files); !reflect.DeepEqual(got, want) {
		t.Fatalf("positions:\n got  %v\n want %v", got, want)
	}
}
