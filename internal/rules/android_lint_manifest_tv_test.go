package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same manifests.
func TestTvManifestRulesLintParity(t *testing.T) {
	tvApp := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-feature android:name="android.software.leanback" android:required="false"/>
  <uses-feature android:name="android.hardware.camera"/>
  <uses-feature android:name="android.hardware.camera.autofocus" android:required="true"/>
  <uses-feature android:name="android.hardware.bluetooth" android:required="false"/>
  <uses-feature android:name="android.hardware.sensors.accelerometer" android:required="TRUE"/>
  <uses-feature android:name="android.hardware.wifi"/>
  <application android:label="x">
    <activity android:name=".A">
      <intent-filter><category android:name="android.intent.category.LEANBACK_LAUNCHER"/></intent-filter>
    </activity>
    <activity-alias android:name=".B" android:targetActivity=".A">
      <intent-filter><category android:name="android.intent.category.LEANBACK_LAUNCHER"/></intent-filter>
    </activity-alias>
    <activity android:name=".C" android:banner="@drawable/b">
      <intent-filter><category android:name="android.intent.category.LEANBACK_LAUNCHER"/></intent-filter>
    </activity>
  </application>
</manifest>
`
	bannered := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-feature android:name="android.hardware.touchscreen" android:required="false"/>
  <application android:label="x" android:banner="@drawable/b">
    <activity android:name=".A">
      <intent-filter><category android:name="android.intent.category.LEANBACK_LAUNCHER"/></intent-filter>
    </activity>
  </application>
</manifest>
`
	touchRequired := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-feature android:name="android.hardware.touchscreen"/>
  <uses-feature android:name="android.hardware.faketouch" android:required="false"/>
  <application android:label="x">
    <activity android:name=".A" android:banner="@drawable/b">
      <intent-filter><category android:name="android.intent.category.LEANBACK_LAUNCHER"/></intent-filter>
    </activity>
  </application>
</manifest>
`
	phoneApp := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-feature android:name="android.hardware.camera"/>
  <application android:label="x">
    <activity android:name=".A"/>
  </application>
</manifest>
`
	leanbackOnly := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-feature android:name="android.software.leanback"/>
  <uses-feature android:name="android.hardware.camera"/>
  <application android:label="x">
    <activity android:name=".A"/>
  </application>
</manifest>
`
	cases := []struct {
		name, rule, xml, want string
	}{
		{"banner missing", "MissingTvBanner", tvApp, "9:4"},
		{"banner on application", "MissingTvBanner", bannered, ""},
		{"banner on a phone app", "MissingTvBanner", phoneApp, ""},
		{"unsupported features", "UnsupportedTvHardware", tvApp, "4:4,5:66,7:71"},
		{"touchscreen required", "UnsupportedTvHardware", touchRequired, "3:4"},
		{"unsupported on phone app", "UnsupportedTvHardware", phoneApp, ""},
		{"leanback feature only", "UnsupportedTvHardware", leanbackOnly, "4:4"},
		{"touchscreen undeclared", "ImpliedTouchscreenHardware", tvApp, "1:2"},
		{"touchscreen optional", "ImpliedTouchscreenHardware", bannered, ""},
		{"touchscreen declared required", "ImpliedTouchscreenHardware", touchRequired, ""},
		{"touchscreen undeclared with leanback feature", "ImpliedTouchscreenHardware", leanbackOnly, "1:2"},
		{"touchscreen on a phone app", "ImpliedTouchscreenHardware", phoneApp, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := manifestRulePositions(t, c.rule, c.xml); got != c.want {
				t.Fatalf("%s positions = %q, want %q", c.rule, got, c.want)
			}
		})
	}
}
