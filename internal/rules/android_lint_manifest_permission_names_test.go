package rules_test

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/pipeline"
)

// manifestEntriesAt writes the files into a temp directory and runs a manifest rule on the manifest at manifestRel,
// returning "line:col" entries, each followed by the message when withMessage is set, sorted by position.
func manifestEntriesAt(t *testing.T, ruleID string, files map[string]string, manifestRel string, withMessage bool) []string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	parsed, err := android.ParseManifestBytes([]byte(files[manifestRel]))
	if err != nil {
		t.Fatal(err)
	}
	m := pipeline.ConvertManifestForRules(android.ConvertManifest(parsed, filepath.Join(dir, manifestRel)))
	findings := runManifestRule(findManifestRule(t, ruleID), m)
	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Line != findings[j].Line {
			return findings[i].Line < findings[j].Line
		}
		return findings[i].Col < findings[j].Col
	})
	var out []string
	for _, f := range findings {
		entry := fmt.Sprintf("%d:%d", f.Line, f.Col)
		if withMessage {
			entry += " " + f.Message
		}
		out = append(out, entry)
	}
	return out
}

// checkManifestParity runs every rule in want on the manifest and compares its entries to the recorded Lint output.
func checkManifestParity(t *testing.T, manifest string, want map[string][]string, withMessage bool) {
	t.Helper()
	files := map[string]string{"AndroidManifest.xml": manifest}
	for rule, entries := range want {
		t.Run(rule, func(t *testing.T) {
			got := manifestEntriesAt(t, rule, files, "AndroidManifest.xml", withMessage)
			if len(got) == 0 && len(entries) == 0 {
				return
			}
			if !reflect.DeepEqual(got, entries) {
				t.Fatalf("findings differ from Lint\n got: %q\nwant: %q", got, entries)
			}
		})
	}
}

// permissionNamesManifest and its expectations come from running Android Lint 32.4.1 on the same manifest.
const permissionNamesManifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-permission android:name="android.permission.read_contacts"/>
  <uses-permission android:name="androd.permission.CAMERA"/>
  <uses-permission android:name="android.permission.CAMERAA"/>
  <uses-permission android:name="android.permission.WRITE_EXTERNAL_STORAG"/>
  <uses-permission android:name="com.android.vending.BILLING"/>
  <uses-permission android:name="com.google.android.c2dm.permission.RECEIVE"/>
  <uses-permission android:name="android.permission.ACCESS_FINE_LOCATIONS"/>
  <uses-permission android:name="READ_CONTACTS"/>
  <uses-permission android:name="android.permission.INTERNET2"/>
  <uses-permission android:name="com.foo.READ_CONTACTS"/>
  <uses-permission android:name="android.permission.SYSTEM_ALERT_WINDOW"/>
  <uses-permission android:name="android.Manifest.permission.CAMERA"/>
  <uses-permission android:name="android.permissions.CAMERA"/>
  <uses-permission android:name="android.permission.get_accounts"/>
  <uses-permission android:name="android.permission.BIND_ACCESSIBILITY_SERVICES"/>
  <uses-permission android:name="com.android.launcher.permission.INSTALL_SHORTCUTS"/>
  <uses-permission android:name="android.permission.READ_SMSS"/>
  <uses-permission android:name="true"/>
  <uses-permission android:name="FALSE"/>
  <uses-permission android:name="com.example.synth.permission.A2"/>
  <uses-permission android:name="com.example.synth.permission.a1"/>
  <uses-permission android:name="com.example.synth.permission.TOTALLY_DIFFERENT_NAME"/>
  <uses-permission android:name="com.example.synth.permission.A1"/>
  <uses-permission android:name="com.example.synth.permissions.A1"/>
  <uses-permission android:name="com.example.synth.permission.A11"/>
  <permission android:name="android.permission.CAMERA"/>
  <permission android:name="android.permission.REBOOT"/>
  <permission android:name="com.example.synth.permission.A1"/>
  <permission android:name="com.example.synth.permission.lower"/>
  <permission android:name="com.example.synth.perm.A"/>
  <permission android:name="other.permission.A"/>
  <permission android:name="com.example.synth.permission."/>
  <permission android:name="com.example.synth.permission"/>
  <permission android:name="android.something.ELSE"/>
  <permission android:name="com.android.voicemail.permission.READ_VOICEMAIL"/>
  <permission android:name="com.example.synth.permission.A_1"/>
  <application android:label="x" android:permission="com.example.synth.permission.A2">
    <activity android:name=".Main" android:exported="true" android:permission="true"/>
    <activity-alias android:name=".Alias" android:targetActivity=".Main" android:exported="false" android:permission="com.example.synth.permission.a1"/>
    <service android:name=".S" android:exported="false" android:permission="com.example.synth.permission.A1x"/>
    <receiver android:name=".Rc" android:exported="false" android:permission="android.permission.BIND_JOB_SERVISE"/>
    <provider android:name=".P" android:authorities="com.example.synth.p" android:exported="false" android:permission="FALSE"/>
  </application>
</manifest>
`

func TestPermissionNameChecksMatchLint(t *testing.T) {
	checkManifestParity(t, permissionNamesManifest, map[string][]string{
		"SystemPermissionTypo": {
			"3:34 Did you mean `android.permission.READ_CONTACTS`?",
			"4:34 Did you mean `android.permission.CAMERA`?",
			"5:34 Did you mean `android.permission.CAMERA`?",
			"6:34 Did you mean `android.permission.WRITE_EXTERNAL_STORAGE`?",
			"9:34 Did you mean `android.permission.ACCESS_FINE_LOCATION`?",
			"11:34 Did you mean `android.permission.INTERNET`?",
			"14:34 Did you mean `android.permission.CAMERA`?",
			"15:34 Did you mean `android.permission.CAMERA`?",
			"16:34 Did you mean `android.permission.GET_ACCOUNTS`?",
			"17:34 Did you mean `android.permission.BIND_ACCESSIBILITY_SERVICE`?",
			"18:34 Did you mean `com.android.launcher.permission.INSTALL_SHORTCUT`?",
			"19:34 Did you mean `android.permission.READ_SMS`?",
			"43:79 Did you mean `android.permission.BIND_JOB_SERVICE`?",
		},
		"CustomPermissionTypo": {
			"4:34 Did you mean `android.permission.CAMERA`?",
			"5:34 Did you mean `android.permission.CAMERA`?",
			"15:34 Did you mean `android.permission.CAMERA`?",
			"22:34 Did you mean `com.example.synth.permission.A1`?",
			"23:34 Did you mean `com.example.synth.permission.A1`?",
			"26:34 Did you mean `com.example.synth.permission.A1`?",
			"27:34 Did you mean `com.example.synth.permission.A1`?",
			"39:54 Did you mean `com.example.synth.permission.A1`?",
			"41:119 Did you mean `com.example.synth.permission.A1`?",
			"42:77 Did you mean `com.example.synth.permission.A1`?",
		},
		"PermissionNamingConvention": {
			"28:29 `android.permission.CAMERA does not follow recommended naming convention`",
			"29:29 `android.permission.REBOOT does not follow recommended naming convention`",
			"31:29 `com.example.synth.permission.lower does not follow recommended naming convention`",
			"32:29 `com.example.synth.perm.A does not follow recommended naming convention`",
			"33:29 `other.permission.A does not follow recommended naming convention`",
			"34:29 `com.example.synth.permission. does not follow recommended naming convention`",
			"35:29 `com.example.synth.permission does not follow recommended naming convention`",
			"36:29 `android.something.ELSE does not follow recommended naming convention`",
			"37:29 `com.android.voicemail.permission.READ_VOICEMAIL does not follow recommended naming convention`",
		},
		"ReservedSystemPermission": {
			"28:29 `android.permission.CAMERA` is a reserved permission",
			"29:29 `android.permission.REBOOT` is a reserved permission",
			"36:29 `android.something.ELSE` is using the reserved system prefix `android.`",
			"37:29 `com.android.voicemail.permission.READ_VOICEMAIL` is a reserved permission",
		},
		"KnownPermissionError": {
			"20:34 `true` is not a valid permission value",
			"21:34 `FALSE` is not a valid permission value",
			"40:80 `true` is not a valid permission value",
			"44:120 `FALSE` is not a valid permission value",
		},
	}, true)
}

// orientationManifest and its expectations come from running Android Lint 32.4.1 on the same manifest.
const orientationManifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <application android:label="x">
    <activity android:name=".Main" android:exported="true" android:screenOrientation="portrait"/>
    <activity android:name=".Main" android:exported="false" android:screenOrientation="unspecified"/>
    <activity android:name=".Unspecified" android:exported="false" android:screenOrientation="unspecified"/>
    <activity android:name=".FullSensor" android:exported="false" android:screenOrientation="fullSensor"/>
    <activity android:name=".Landscape" android:exported="false" android:screenOrientation="landscape"/>
    <activity android:name=".SensorLandscape" android:exported="false" android:screenOrientation="sensorLandscape"/>
    <activity android:name=".Reverse" android:exported="false" android:screenOrientation="reversePortrait"/>
    <activity android:name=".Sensor" android:exported="false" android:screenOrientation="sensorPortrait"/>
    <activity android:name=".User" android:exported="false" android:screenOrientation="userPortrait"/>
    <activity android:name=".Locked" android:exported="false" android:screenOrientation="locked"/>
    <activity android:name=".R1" android:exported="false" android:resizeableActivity="TRUE"/>
    <activity android:name=".R2" android:exported="false" android:resizeableActivity="true"/>
    <activity android:name=".R3" android:exported="false" android:resizeableActivity="@bool/resizeable"/>
    <activity android:name=".R4" android:exported="false" android:resizeableActivity=""/>
    <activity android:name=".R5" android:exported="false" android:resizeableActivity="false" android:screenOrientation="sensorPortrait"/>
    <activity android:name=".R6" android:exported="false"/>
    <activity android:exported="false" android:screenOrientation="reversePortrait"/>
    <activity android:exported="false" android:resizeableActivity="false"/>
    <activity android:name=".Ratio" android:exported="false" android:minAspectRatio="1.5" android:maxAspectRatio="2.0"/>
    <activity android:name=".Links" android:exported="true">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <data android:scheme="http"/>
        <data android:host="a.example" android:port="80"/>
        <data android:pathPrefix="/x" android:pathSuffix=".html"/>
        <data android:scheme="" android:host=""/>
        <data android:mimeType="text/plain" android:scheme="content"/>
        <data android:path="/a" android:pathPattern="/b" android:pathAdvancedPattern="/c"/>
        <data android:port="1"/>
      </intent-filter>
      <intent-filter>
        <data android:scheme="http" android:host="x.example"/>
      </intent-filter>
      <intent-filter>
        <data android:scheme="http" android:host="x.example"/>
        <data android:scheme="https"/>
      </intent-filter>
    </activity>
  </application>
</manifest>
`

func TestOrientationAndDataChecksMatchLint(t *testing.T) {
	checkManifestParity(t, orientationManifest, map[string][]string{
		"LockedOrientationActivity": {
			"4:60",
			"5:61",
			"10:64",
			"11:63",
			"12:61",
			"18:94",
			"20:40",
			"21:6",
		},
		"NonResizeableActivity": {
			"16:59",
			"17:59",
			"18:59",
			"20:6",
			"21:40",
		},
		"IntentFilterUniqueDataAttributes": {
			"28:9",
			"29:9",
			"31:9",
			"38:9",
		},
		"DiscouragedApi": {
			"4:60",
			"5:61",
			"6:68",
			"7:67",
			"8:66",
			"9:72",
			"10:64",
			"11:63",
			"12:61",
			"13:63",
			"18:59",
			"18:94",
			"20:40",
			"21:40",
			"22:62",
			"22:91",
		},
	}, false)
}

const orientationOnlyManifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth">
  <application android:label="x">
    <activity android:name=".Main" android:exported="true" android:screenOrientation="portrait" android:resizeableActivity="false"/>
  </application>
</manifest>
`

// Lint skips both large-screen activity checks in library projects.
func TestLargeScreenActivityChecksSkipLibraries(t *testing.T) {
	library := map[string]string{
		"lib/build.gradle.kts":             `plugins { id("com.android.library") }`,
		"lib/src/main/AndroidManifest.xml": orientationOnlyManifest,
	}
	app := map[string]string{
		"app/build.gradle.kts":             `plugins { id("com.android.application") }`,
		"app/src/main/AndroidManifest.xml": orientationOnlyManifest,
	}
	for _, rule := range []string{"LockedOrientationActivity", "NonResizeableActivity"} {
		if got := manifestEntriesAt(t, rule, library, "lib/src/main/AndroidManifest.xml", false); len(got) != 0 {
			t.Errorf("%s in a library = %q, want none", rule, got)
		}
		if got := manifestEntriesAt(t, rule, app, "app/src/main/AndroidManifest.xml", false); len(got) != 1 {
			t.Errorf("%s in an app = %q, want one finding", rule, got)
		}
	}
	// DiscouragedApi has no project-type constraint.
	got := manifestEntriesAt(t, "DiscouragedApi", library, "lib/src/main/AndroidManifest.xml", false)
	if want := []string{"3:60", "3:97"}; !reflect.DeepEqual(got, want) {
		t.Errorf("DiscouragedApi in a library = %q, want %q", got, want)
	}
}

// Expected positions come from running Android Lint 32.4.1 on the same manifest.
func TestDiscouragedApiMessages(t *testing.T) {
	manifest := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth">
  <application android:label="x">
    <activity android:name=".A" android:screenOrientation="sensor" android:minAspectRatio="1.5" android:resizeableActivity="false"/>
    <activity android:name=".B" android:maxAspectRatio="2.0" android:resizeableActivity="true"/>
  </application>
</manifest>
`
	const tail = " will be ignored in most cases, starting from Android 16. " +
		"Android is moving toward a model where apps are expected to adapt to various orientations, display sizes, and aspect ratios."
	want := []string{
		"3:33 Fixed screen orientations" + tail,
		"3:68 Minimum and maximum aspect ratios" + tail,
		"3:97 Setting `resizeableActivity` to `false`" + tail,
		"4:33 Minimum and maximum aspect ratios" + tail,
	}
	got := manifestEntriesAt(t, "DiscouragedApi", map[string]string{"AndroidManifest.xml": manifest}, "AndroidManifest.xml", true)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("DiscouragedApi messages\n got: %q\nwant: %q", got, want)
	}
}

// Expected positions come from running Android Lint 32.4.1 on the same manifest with each minSdkVersion.
func TestSystemPermissionMatchesLintProtectedPermissions(t *testing.T) {
	const manifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.synth">
__SDK__
  <uses-permission android:name="android.permission.INSTALL_PACKAGES"/>
  <uses-permission android:name="android.permission.READ_LOGS"/>
  <uses-permission android:name="android.permission.READ_LOGS" android:maxSdkVersion="15"/>
  <uses-permission android:name="android.permission.READ_LOGS" android:maxSdkVersion="16"/>
  <uses-permission android:name="android.permission.READ_LOGS" android:maxSdkVersion="x"/>
  <uses-permission android:name="android.permission.SET_DEBUG_APP"/>
  <uses-permission android:name="android.permission.CHANGE_CONFIGURATION"/>
  <uses-permission android:name="android.permission.ACCESS_MOCK_LOCATION"/>
  <uses-permission android:name="android.permission.SET_TIME_ZONE"/>
  <uses-permission android:name="android.permission.SUBSCRIBE_TO_KEYGUARD_LOCKED_STATE"/>
  <uses-permission android:name="android.permission.REBOOT" android:maxSdkVersion="20"/>
  <uses-permission android:name="android.permission.CAMERA"/>
  <uses-permission android:name="android.permission.WRITE_SECURE_SETTINGS"/>
  <uses-permission android:name="android.permission.ACCESS_SURFACE_FLINGER"/>
  <uses-permission android:name="com.android.voicemail.permission.READ_VOICEMAIL"/>
  <uses-permission android:name="android.intent.category.MASTER_CLEAR.permission.C2D_MESSAGE"/>
  <uses-permission-sdk-23 android:name="android.permission.INSTALL_PACKAGES"/>
  <application android:label="x"/>
</manifest>
`
	cases := []struct {
		name    string
		usesSdk string
		want    string
	}{
		{"no uses-sdk", ``, "3:20,4:20,6:20,7:20,8:20,9:20,10:20,11:20,12:20,13:20,15:20,16:20,17:20,18:20"},
		{"minSdk 16", `  <uses-sdk android:minSdkVersion="16" android:targetSdkVersion="34"/>`, "3:20,9:20,10:20,11:20,12:20,13:20,15:20,16:20,17:20,18:20"},
		{"minSdk 26", `  <uses-sdk android:minSdkVersion="26" android:targetSdkVersion="34"/>`, "3:20,12:20,13:20,15:20,16:20,17:20,18:20"},
		{"minSdk 34", `  <uses-sdk android:minSdkVersion="34" android:targetSdkVersion="34"/>`, "3:20,13:20,15:20,16:20,17:20,18:20"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			xml := strings.Replace(manifest, "__SDK__", c.usesSdk, 1)
			if got := manifestRulePositions(t, "SystemPermission", xml); got != c.want {
				t.Errorf("positions = %s, want %s", got, c.want)
			}
		})
	}
}

func TestSystemPermissionMessage(t *testing.T) {
	findings := runManifestRule(findManifestRule(t, "SystemPermission"), parseManifestForTest(t, manifestHead+
		`  <uses-permission android:name="android.permission.INSTALL_PACKAGES"/>
  <uses-permission android:name="android.permission.CAMERA"/>
</manifest>
`))
	if len(findings) != 1 || findings[0].Message != "Permission is only granted to system apps" {
		t.Fatalf("findings = %+v, want one system-app message", findings)
	}
}
