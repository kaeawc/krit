package rules_test

import (
	"fmt"
	"testing"
)

func applicationManifest(minSdk int, appAttrs string) string {
	return fmt.Sprintf(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth">
  <uses-sdk android:minSdkVersion="%d" android:targetSdkVersion="34"/>
  <application android:label="x" %s/>
</manifest>
`, minSdk, appAttrs)
}

// Expected findings come from running Android Lint 32.4.1 on the same manifests.
func TestDataExtractionRulesLintParity(t *testing.T) {
	const (
		allowFalse = `android:allowBackup="false"`
		allowTrue  = `android:allowBackup="true"`
		full       = `android:fullBackupContent="@xml/b"`
		rules      = `android:dataExtractionRules="@xml/r"`
	)
	cases := []struct {
		name   string
		minSdk int
		attrs  string
		want   int
	}{
		{"allowBackup=false below 31", 21, allowFalse, 1},
		{"fullBackupContent only below 31", 21, full, 1},
		{"dataExtractionRules only below 31", 21, rules, 1},
		{"allowBackup=true only at 31", 31, allowTrue, 1},
		{"allowBackup=false only at 31", 31, allowFalse, 1},
		{"allowBackup and fullBackupContent at 31", 31, full + " " + allowTrue, 2},
		{"allowBackup=false with rules below 31", 21, allowFalse + " " + rules, 1},
		{"both backup attributes below 31", 21, full + " " + rules, 0},
		{"allowBackup=true with fullBackupContent below 31", 21, allowTrue + " " + full, 1},
		{"both backup attributes at 31", 31, full + " " + rules, 0},
		{"allowBackup=true with rules at 31", 31, allowTrue + " " + rules, 0},
		{"allowBackup=false with fullBackupContent below 31", 21, allowFalse + " " + full, 1},
		{"allowBackup=true only below 31", 21, allowTrue, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			xml := applicationManifest(c.minSdk, c.attrs)
			got := len(runManifestRule(findManifestRule(t, "DataExtractionRules"), parseManifestForTest(t, xml)))
			if got != c.want {
				t.Fatalf("findings = %d, want %d", got, c.want)
			}
		})
	}
}

func TestDataExtractionRulesPositionAndMessage(t *testing.T) {
	xml := applicationManifest(21, `android:fullBackupContent="@xml/b"`)
	findings := runManifestRule(findManifestRule(t, "DataExtractionRules"), parseManifestForTest(t, xml))
	if len(findings) != 1 || findings[0].Line != 3 || findings[0].Col != 61 {
		t.Fatalf("findings = %+v, want one at 3:61", findings)
	}
	want := "The attribute `android:fullBackupContent` is deprecated from Android 12 and higher and may be removed in future versions. Consider adding the attribute `android:dataExtractionRules` specifying an `@xml` resource which configures cloud backups and device transfers on Android 12 and higher."
	if findings[0].Message != want {
		t.Fatalf("message = %q, want %q", findings[0].Message, want)
	}
}

func TestRedundantLabelLintParity(t *testing.T) {
	xml := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <application android:label="@string/app_name" android:allowBackup="true">
    <activity android:name=".A" android:label="@string/app_name"/>
    <activity android:name=".B" android:label="@string/other"/>
    <activity android:name=".C"/>
    <activity android:name=".D" android:label="@string/app_name">
      <intent-filter><action android:name="android.intent.action.MAIN"/></intent-filter>
    </activity>
    <activity-alias android:name=".E" android:targetActivity=".A" android:label="@string/app_name"/>
    <service android:name=".S" android:label="@string/app_name"/>
  </application>
</manifest>
`
	if got, want := manifestRulePositions(t, "RedundantLabel", xml), "4:33,7:33"; got != want {
		t.Fatalf("positions = %s, want %s", got, want)
	}
}
