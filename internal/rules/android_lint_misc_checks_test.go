package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
)

// Expected positions come from running Android Lint 32.4.1 on the same files.
func TestSignatureOrSystemAndHighSamplingRateLintParity(t *testing.T) {
	xml := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <permission android:name="com.example.P1" android:protectionLevel="signatureOrSystem"/>
  <permission android:name="com.example.P2" android:protectionLevel="signature"/>
  <permission android:name="com.example.P3" android:protectionLevel="signature|privileged"/>
  <permission android:name="com.example.P4" android:protectionLevel="signatureOrSystem|dangerous"/>
  <uses-permission android:name="android.permission.HIGH_SAMPLING_RATE_SENSORS"/>
  <application android:label="x">
    <activity android:name=".A" android:protectionLevel="signatureOrSystem"/>
  </application>
</manifest>
`
	if got, want := manifestRulePositions(t, "SignatureOrSystemPermissions", xml), "3:45,9:33"; got != want {
		t.Errorf("SignatureOrSystemPermissions positions = %s, want %s", got, want)
	}
	if got, want := manifestRulePositions(t, "HighSamplingRate", xml), "7:34"; got != want {
		t.Errorf("HighSamplingRate positions = %s, want %s", got, want)
	}
	old := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="30"/>
  <uses-permission android:name="android.permission.HIGH_SAMPLING_RATE_SENSORS"/>
</manifest>
`
	if got := manifestRulePositions(t, "HighSamplingRate", old); got != "" {
		t.Errorf("targetSdk 30 positions = %s, want none", got)
	}
}

func TestKeyboardInaccessibleWidgetLintParity(t *testing.T) {
	files := map[string]string{"layout/a.xml": `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:clickable="true"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:clickable="true" android:focusable="true"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:clickable="true" android:focusable="false"/>
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:clickable="false"/>
</LinearLayout>`}
	got := resourcePositions(t, "KeyboardInaccessibleWidget", files)
	want := []string{"layout/a.xml:2:86", "layout/a.xml:4:86"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("positions = %v, want %v", got, want)
	}
}

// Lint reports only when the module's minSdk is below 26, where clickable views are not focusable by default.
func TestKeyboardInaccessibleWidgetGatesOnModuleMinSdk(t *testing.T) {
	layout := `<LinearLayout xmlns:android="http://schemas.android.com/apk/res/android" android:layout_width="match_parent" android:layout_height="match_parent">
  <TextView android:layout_width="wrap_content" android:layout_height="wrap_content" android:clickable="true"/>
</LinearLayout>`
	for minSdk, want := range map[int]int{21: 1, 25: 1, 26: 0, 29: 0} {
		dir := t.TempDir()
		gradle := fmt.Sprintf("android {\n  defaultConfig {\n    minSdk = %d\n  }\n}\n", minSdk)
		if err := os.WriteFile(filepath.Join(dir, "build.gradle.kts"), []byte(gradle), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Join(dir, "res/layout"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "res/layout/a.xml"), []byte(layout), 0o644); err != nil {
			t.Fatal(err)
		}
		idx, err := android.ScanResourceDir(filepath.Join(dir, "res"))
		if err != nil {
			t.Fatal(err)
		}
		if got := len(runResourceRule(findResourceRule(t, "KeyboardInaccessibleWidget"), idx)); got != want {
			t.Errorf("minSdk %d: %d findings, want %d", minSdk, got, want)
		}
	}
}
