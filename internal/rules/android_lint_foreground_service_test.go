package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
)

// foregroundServiceManifest and its expectations come from running Android Lint 32.4.1 on the same manifest.
const foregroundServiceManifest = `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools" package="com.example.synth">
  <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>
  <uses-permission android:name="android.permission.FOREGROUND_SERVICE"/>
  <uses-permission android:name="android.permission.FOREGROUND_SERVICE_LOCATION"/>
  <uses-permission android:name="android.permission.FOREGROUND_SERVICE_MEDIA_PLAYBACK" tools:node="remove"/>
  <uses-permission-sdk-23 android:name="android.permission.FOREGROUND_SERVICE_DATA_SYNC"/>
  <uses-permission android:name="android.permission.FOREGROUND_SERVICE_CAMERA"/>
  <uses-permission android:name="android.permission.CAMERA"/>
  <uses-permission android:name="android.permission.QUERY_ALL_PACKAGES"/>
  <uses-permission
      android:name="android.permission.QUERY_ALL_PACKAGES"/>
  <uses-permission android:name="android.permission.query_all_packages"/>
  <uses-permission-sdk-23 android:name="android.permission.QUERY_ALL_PACKAGES"/>
  <uses-permission android:name="android.permission.INTERNET"/>
  <application android:icon="@mipmap/ic" android:name=".App">
    <service android:name=".A" android:foregroundServiceType="location"/>
    <service android:name=".B" android:foregroundServiceType="mediaPlayback"/>
    <service android:name=".C" android:foregroundServiceType="dataSync"/>
    <service android:name=".D" android:foregroundServiceType="camera"/>
    <service android:name=".E" android:foregroundServiceType="dataSync | camera"/>
    <service android:name=".F" android:foregroundServiceType="specialUse|health|microphone"/>
    <service android:name=".G" android:foregroundServiceType="@string/kind"/>
    <service android:name=".H"/>
    <service android:name=".I" android:foregroundServiceType="shortService|systemExempted"/>
    <service android:name=".J" android:foregroundServiceType="shortService"/>
    <service android:name=".K" android:foregroundServiceType="Location"/>
  </application>
</manifest>
`

func TestForegroundServicePermissionMatchesLint(t *testing.T) {
	const location = "foregroundServiceType:location requires permission:[android.permission.FOREGROUND_SERVICE_LOCATION]" +
		" AND any permission in list:[android.permission.ACCESS_COARSE_LOCATION, android.permission.ACCESS_FINE_LOCATION]"
	const health = "foregroundServiceType:health requires permission:[android.permission.FOREGROUND_SERVICE_HEALTH]" +
		" AND any permission in list:[android.permission.ACTIVITY_RECOGNITION, android.permission.BODY_SENSORS," +
		" android.permission.HIGH_SAMPLING_RATE_SENSORS]"
	const microphone = "foregroundServiceType:microphone requires permission:[android.permission.FOREGROUND_SERVICE_MICROPHONE]" +
		" AND any permission in list:[android.permission.CAPTURE_AUDIO_HOTWORD, android.permission.CAPTURE_AUDIO_OUTPUT," +
		" android.permission.CAPTURE_MEDIA_OUTPUT, android.permission.CAPTURE_TUNER_AUDIO_INPUT," +
		" android.permission.CAPTURE_VOICE_COMMUNICATION_OUTPUT, android.permission.RECORD_AUDIO]"
	const exempted = "foregroundServiceType:systemExempted requires permission:[android.permission.FOREGROUND_SERVICE_SYSTEM_EXEMPTED]" +
		" AND any permission in list:[android.permission.SCHEDULE_EXACT_ALARM, android.permission.USE_EXACT_ALARM]"
	checkManifestParity(t, foregroundServiceManifest, map[string][]string{
		"ForegroundServicePermission": {
			"16:5 " + location,
			"18:5 foregroundServiceType:dataSync requires permission:[android.permission.FOREGROUND_SERVICE_DATA_SYNC]",
			"21:5 foregroundServiceType:specialUse requires permission:[android.permission.FOREGROUND_SERVICE_SPECIAL_USE]",
			"21:5 " + health,
			"21:5 " + microphone,
			"24:5 " + exempted,
		},
	}, true)
}

func TestForegroundServicePermissionNeedsTargetSdk34AndBasePermission(t *testing.T) {
	older := strings.Replace(foregroundServiceManifest, `targetSdkVersion="34"`, `targetSdkVersion="33"`, 1)
	noBase := strings.Replace(foregroundServiceManifest,
		`<uses-permission android:name="android.permission.FOREGROUND_SERVICE"/>`, "", 1)
	noSdk := strings.Replace(foregroundServiceManifest, `<uses-sdk android:minSdkVersion="21" android:targetSdkVersion="34"/>`, "", 1)
	for name, manifest := range map[string]string{"target 33": older, "no base permission": noBase, "no uses-sdk": noSdk} {
		t.Run(name, func(t *testing.T) {
			checkManifestParity(t, manifest, map[string][]string{"ForegroundServicePermission": nil}, true)
		})
	}
}

func TestQueryAllPackagesPermissionMatchesLint(t *testing.T) {
	const message = "A `<queries>` declaration should generally be used instead of QUERY_ALL_PACKAGES;" +
		" see https://g.co/dev/packagevisibility for details"
	checkManifestParity(t, foregroundServiceManifest, map[string][]string{
		"QueryAllPackagesPermission": {"9:20 " + message, "11:7 " + message},
	}, true)
	older := strings.Replace(foregroundServiceManifest, `targetSdkVersion="34"`, `targetSdkVersion="29"`, 1)
	checkManifestParity(t, older, map[string][]string{"QueryAllPackagesPermission": nil}, true)
	sdk30 := strings.Replace(foregroundServiceManifest, `targetSdkVersion="34"`, `targetSdkVersion="30"`, 1)
	checkManifestParity(t, sdk30, map[string][]string{
		"QueryAllPackagesPermission": {"9:20 " + message, "11:7 " + message},
	}, true)
}

func TestManifestPermissionLintDefaults(test *testing.T) {
	if !rules.IsDefaultActive("ForegroundServicePermission") {
		test.Fatal("Lint enables ForegroundServicePermission by default")
	}
	if rules.IsDefaultActive("QueryAllPackagesPermission") {
		test.Fatal("Lint disables QueryAllPackagesPermission by default")
	}
}

func TestManifestPermissionProjectGates(test *testing.T) {
	const manifestPath = "app/src/main/AndroidManifest.xml"
	for _, testCase := range []struct {
		name       string
		plugin     string
		sdk        string
		foreground bool
		queryAll   bool
	}{
		{"application 34", "com.android.application", "34", true, true},
		{"application 35", "com.android.application", "35", true, true},
		{"application 33", "com.android.application", "33", false, true},
		{"application 30", "com.android.application", "30", false, true},
		{"application 29 overrides manifest 34", "com.android.application", "29", false, false},
		{"unresolved override abstains", "com.android.application", "sdkVersion", false, false},
		{"numeric expression abstains", "com.android.application", "34 - 10", false, false},
		{"high SDK expression abstains", "com.android.application", "34 + 0", false, false},
		{"multiple overrides abstain", "com.android.application", "34\n    targetSdk = 29", false, false},
		{"commented override abstains", "com.android.application", "34\n    /* targetSdk = 29 */", false, false},
		{"library", "com.android.library", "34", false, false},
		{"test module", "com.android.test", "34", false, false},
		{"unknown convention plugin abstains", "example.android.convention", "34", false, false},
	} {
		test.Run(testCase.name, func(test *testing.T) {
			build := "plugins { id(\"" + testCase.plugin + "\") }\nandroid {\n  defaultConfig {\n    targetSdk = " + testCase.sdk + "\n  }\n}"
			files := map[string]string{manifestPath: foregroundServiceManifest, "app/build.gradle.kts": build}
			for ruleID, wantReport := range map[string]bool{
				"ForegroundServicePermission": testCase.foreground,
				"QueryAllPackagesPermission":  testCase.queryAll,
			} {
				entries := manifestEntriesAt(test, ruleID, files, manifestPath, false)
				if (len(entries) != 0) != wantReport {
					test.Errorf("%s: findings %v, want report=%v", ruleID, entries, wantReport)
				}
			}
		})
	}
}

func TestManifestPermissionSkipsTestManifestsAndKeepsVariantOverlays(test *testing.T) {
	build := "plugins { id(\"com.android.application\") }\nandroid {\n  defaultConfig {\n    targetSdk = 34\n  }\n}"
	for _, sourceSet := range []string{"androidTest", "test", "testFixtures"} {
		test.Run(sourceSet, func(test *testing.T) {
			manifestPath := "app/src/" + sourceSet + "/AndroidManifest.xml"
			files := map[string]string{manifestPath: foregroundServiceManifest, "app/build.gradle.kts": build}
			for _, ruleID := range []string{"ForegroundServicePermission", "QueryAllPackagesPermission"} {
				if entries := manifestEntriesAt(test, ruleID, files, manifestPath, false); len(entries) != 0 {
					test.Errorf("%s: test manifest findings %v, want none", ruleID, entries)
				}
			}
		})
	}
	for _, sourceSet := range []string{"debug", "release", "free", "freeDebug"} {
		test.Run(sourceSet, func(test *testing.T) {
			manifestPath := "app/src/" + sourceSet + "/AndroidManifest.xml"
			files := map[string]string{manifestPath: foregroundServiceManifest, "app/build.gradle.kts": build}
			for ruleID, wantReport := range map[string]bool{
				"ForegroundServicePermission": true,
				"QueryAllPackagesPermission":  true,
			} {
				if entries := manifestEntriesAt(test, ruleID, files, manifestPath, false); (len(entries) != 0) != wantReport {
					test.Errorf("%s: findings %v, want report=%v", ruleID, entries, wantReport)
				}
			}
		})
	}
}

func TestManifestPermissionGradleWithoutTargetSdkAbstains(test *testing.T) {
	const manifestPath = "app/src/main/AndroidManifest.xml"
	build := `plugins { id("acme.android.application") }
android { defaultConfig { applicationId = "com.example" } }`
	files := map[string]string{manifestPath: foregroundServiceManifest, "app/build.gradle.kts": build}
	for _, ruleID := range []string{"ForegroundServicePermission", "QueryAllPackagesPermission"} {
		if entries := manifestEntriesAt(test, ruleID, files, manifestPath, false); len(entries) != 0 {
			test.Errorf("%s: findings %v, want none without a proven Gradle target SDK", ruleID, entries)
		}
	}
}

func TestManifestPermissionSdkTextGuards(test *testing.T) {
	for name, declaration := range map[string]string{
		"block comment":    "/*\n    targetSdk = 34\n*/",
		"multiline string": "val example = \"\"\"\n    targetSdk = 34\n\"\"\"",
	} {
		test.Run(name, func(test *testing.T) {
			const manifestPath = "app/src/main/AndroidManifest.xml"
			source := strings.Replace(foregroundServiceManifest, `targetSdkVersion="34"`, `targetSdkVersion="29"`, 1)
			build := "plugins { id(\"com.android.application\") }\nandroid {\n  defaultConfig {\n" + declaration + "\n  }\n}"
			files := map[string]string{manifestPath: source, "app/build.gradle.kts": build}
			for _, ruleID := range []string{"ForegroundServicePermission", "QueryAllPackagesPermission"} {
				if entries := manifestEntriesAt(test, ruleID, files, manifestPath, false); len(entries) != 0 {
					test.Errorf("%s: SDK text is not a declaration, got %v", ruleID, entries)
				}
			}
		})
	}
}

func TestManifestPermissionNamespaceGates(test *testing.T) {
	for name, source := range map[string]string{
		"wrong Android URI":         strings.ReplaceAll(foregroundServiceManifest, "http://schemas.android.com/apk/res/android", "https://example.com/not-android"),
		"alternate prefix abstains": strings.NewReplacer("android:", "platform:", "xmlns:android=", "xmlns:platform=").Replace(foregroundServiceManifest),
		"mixed permission prefix": `<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:platform="http://schemas.android.com/apk/res/android">
  <uses-sdk android:targetSdkVersion="34"/>
  <uses-permission android:name="android.permission.FOREGROUND_SERVICE"/>
  <uses-permission platform:name="android.permission.FOREGROUND_SERVICE_DATA_SYNC"/>
  <application><service android:foregroundServiceType="dataSync"/></application>
</manifest>`,
		"subtree rebinding abstains": strings.Replace(foregroundServiceManifest, "<application ", `<application xmlns:android="https://example.com/not-android" `, 1),
	} {
		test.Run(name, func(test *testing.T) {
			checkManifestParity(test, source, map[string][]string{
				"ForegroundServicePermission": nil,
				"QueryAllPackagesPermission":  nil,
			}, true)
		})
	}
}

func TestForegroundServicePermissionSatisfiedTypes(test *testing.T) {
	for serviceType, permissions := range map[string][]string{
		"dataSync":        {"FOREGROUND_SERVICE_DATA_SYNC"},
		"mediaPlayback":   {"FOREGROUND_SERVICE_MEDIA_PLAYBACK"},
		"phoneCall":       {"FOREGROUND_SERVICE_PHONE_CALL"},
		"connectedDevice": {"FOREGROUND_SERVICE_CONNECTED_DEVICE"},
		"mediaProjection": {"FOREGROUND_SERVICE_MEDIA_PROJECTION"},
		"remoteMessaging": {"FOREGROUND_SERVICE_REMOTE_MESSAGING"},
		"fileManagement":  {"FOREGROUND_SERVICE_FILE_MANAGEMENT"},
		"specialUse":      {"FOREGROUND_SERVICE_SPECIAL_USE"},
		"location":        {"FOREGROUND_SERVICE_LOCATION", "ACCESS_FINE_LOCATION"},
		"camera":          {"FOREGROUND_SERVICE_CAMERA", "SYSTEM_CAMERA"},
		"microphone":      {"FOREGROUND_SERVICE_MICROPHONE", "RECORD_AUDIO"},
		"health":          {"FOREGROUND_SERVICE_HEALTH", "BODY_SENSORS"},
		"systemExempted":  {"FOREGROUND_SERVICE_SYSTEM_EXEMPTED", "USE_EXACT_ALARM"},
		"shortService":    nil,
		"unknown":         nil,
	} {
		test.Run(serviceType, func(test *testing.T) {
			source := `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
  <uses-sdk android:targetSdkVersion="34"/>
  <uses-permission android:name="android.permission.FOREGROUND_SERVICE"/>`
			for _, permission := range permissions {
				source += `
  <uses-permission android:name="android.permission.` + permission + `"/>`
			}
			source += `
  <application><service android:foregroundServiceType="` + serviceType + `"/></application>
</manifest>`
			checkManifestParity(test, source, map[string][]string{"ForegroundServicePermission": nil}, true)
			for _, permission := range permissions {
				missing := strings.Replace(source, `<uses-permission android:name="android.permission.`+permission+`"/>`, "", 1)
				entries := manifestEntriesAt(test, "ForegroundServicePermission", map[string]string{"AndroidManifest.xml": missing}, "AndroidManifest.xml", true)
				if len(entries) != 1 {
					test.Errorf("missing %s: findings %v, want one", permission, entries)
				}
			}
		})
	}
}
