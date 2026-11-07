package rules_test

import "testing"

// Expected positions come from running Android Lint 32.4.1 on the same manifests.
func TestManifestMiscLintParity(t *testing.T) {
	full := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-permission android:name="android.permission.ACCESS_FINE_LOCATION"/>
  <uses-permission android:name="android.permission.ACCESS_WIFI_STATE" android:permission="foo"/>
  <uses-feature android:name="android.hardware.location.gps"/>
  <uses-feature android:name="android.hardware.camera.autofocus" android:required="true"/>
  <uses-feature android:name="android.hardware.nfc" android:required="false"/>
  <uses-feature android:name="android.hardware.telephony" android:required="TRUE"/>
  <uses-feature android:name="android.hardware.wifi"/>
  <uses-feature android:name="android.software.leanback"/>
  <application android:label="x" android:permission="p">
    <receiver android:name="com.google.android.c2dm.C2DMBroadcastReceiver" android:permission="com.google.android.c2dm.permission.SEND">
      <intent-filter><action android:name="com.google.android.c2dm.intent.RECEIVE"/></intent-filter>
      <intent-filter><action android:name="com.google.android.c2dm.intent.REGISTRATION"/></intent-filter>
    </receiver>
    <receiver android:name="com.google.android.gcm.GCMBroadcastReceiver">
      <intent-filter><action android:name="com.google.android.c2dm.intent.RECEIVE"/></intent-filter>
    </receiver>
    <activity android:name=".A" android:permission="q">
      <intent-filter android:permission="bad"><action android:name="a"/></intent-filter>
    </activity>
    <provider android:name=".P" android:authorities="a" android:permission="r"><grant-uri-permission android:permission="s" android:path="x"/></provider>
  </application>
</manifest>
`
	cases := []struct{ rule, xml, want string }{
		{"UsingC2DM", full, "12:29"},
		{"InvalidPermission", full, "4:72,20:22,22:102"},
		{"CoarseFineLocation", full, "3:3"},
		{"UnnecessaryRequiredFeature", full, "5:31,6:31,8:31"},
		{"LeanbackUsesWifi", full, "9:3"},
	}
	for _, c := range cases {
		t.Run(c.rule, func(t *testing.T) {
			if got := manifestRulePositions(t, c.rule, c.xml); got != c.want {
				t.Fatalf("positions = %s, want %s", got, c.want)
			}
		})
	}

	coarse := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-permission android:name="android.permission.ACCESS_COARSE_LOCATION"/>
  <uses-permission android:name="android.permission.ACCESS_FINE_LOCATION"/>
</manifest>
`
	if got := manifestRulePositions(t, "CoarseFineLocation", coarse); got != "" {
		t.Errorf("both location permissions: positions = %s, want none", got)
	}
	capped := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-feature android:name="android.software.leanback"/>
  <uses-permission android:name="android.permission.CHANGE_WIFI_STATE"/>
  <uses-permission android:name="android.permission.ACCESS_FINE_LOCATION" android:maxSdkVersion="30"/>
  <uses-permission android:name="android.permission.ACCESS_FINE_LOCATION" android:maxSdkVersion="x"/>
</manifest>
`
	if got, want := manifestRulePositions(t, "LeanbackUsesWifi", capped), "4:3"; got != want {
		t.Errorf("wifi permission: positions = %s, want %s", got, want)
	}
	if got, want := manifestRulePositions(t, "CoarseFineLocation", capped), "6:3"; got != want {
		t.Errorf("uncapped fine location: positions = %s, want %s", got, want)
	}
}
