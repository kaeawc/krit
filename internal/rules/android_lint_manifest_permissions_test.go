package rules_test

import "testing"

const manifestHead = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth">
`

// Expected positions come from running Android Lint 32.4.1 on the same manifests.
func TestExactAlarmLintParity(t *testing.T) {
	old := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="31"/>
  <uses-permission android:name="android.permission.USE_EXACT_ALARM"/>
  <uses-permission android:name="android.permission.READ_MEDIA_IMAGES"/>
  <application android:label="x"/>
</manifest>
`
	if got, want := manifestRulePositions(t, "ExactAlarm", old), "3:34"; got != want {
		t.Errorf("targetSdk 31 positions = %s, want %s", got, want)
	}
	current := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-permission android:name="android.permission.USE_EXACT_ALARM"/>
  <application android:label="x"/>
</manifest>
`
	if got := manifestRulePositions(t, "ExactAlarm", current); got != "" {
		t.Errorf("targetSdk 34 positions = %s, want none", got)
	}
}

func TestSelectedPhotoAccessLintParity(t *testing.T) {
	unhandled := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-permission android:name="android.permission.USE_EXACT_ALARM"/>
  <uses-permission android:name="android.permission.READ_MEDIA_IMAGES"/>
  <uses-permission android:name="android.permission.READ_MEDIA_VIDEO"/>
  <application android:label="x"/>
</manifest>
`
	if got, want := manifestRulePositions(t, "SelectedPhotoAccess", unhandled), "4:34,5:34"; got != want {
		t.Errorf("positions = %s, want %s", got, want)
	}
	handled := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-permission android:name="android.permission.READ_MEDIA_IMAGES"/>
  <uses-permission android:name="android.permission.READ_MEDIA_VISUAL_USER_SELECTED"/>
  <application android:label="x"/>
</manifest>
`
	if got := manifestRulePositions(t, "SelectedPhotoAccess", handled); got != "" {
		t.Errorf("handled positions = %s, want none", got)
	}
	old := manifestHead + `  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="31"/>
  <uses-permission android:name="android.permission.READ_MEDIA_IMAGES"/>
  <application android:label="x"/>
</manifest>
`
	if got := manifestRulePositions(t, "SelectedPhotoAccess", old); got != "" {
		t.Errorf("targetSdk 31 positions = %s, want none", got)
	}
}
