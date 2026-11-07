package android

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConvertManifest_PreservesApplicationSecurityAndBackupFields(t *testing.T) {
	m := &Manifest{
		Package: "com.example.app",
		UsesSdk: UsesSdk{
			MinSdkVersion:    "24",
			TargetSdkVersion: "34",
		},
		UsesPermissions: []UsesPermission{
			{Name: "android.permission.INTERNET"},
		},
		Permissions: []Permission{
			{Name: "com.example.app.CUSTOM_PERMISSION"},
		},
		Application: Application{
			AllowBackup:          "true",
			Debuggable:           "false",
			UsesCleartextTraffic: "true",
			FullBackupContent:    "@xml/backup_rules",
			DataExtractionRules:  "@xml/data_extraction_rules",
			Icon:                 "@mipmap/ic_launcher",
			Activities: []Activity{
				{Name: ".MainActivity", Exported: "true", IntentFilters: []IntentFilter{{}}},
			},
		},
		Elements: []ElementInfo{
			{Tag: "manifest", ParentTag: "", Line: 1},
			{Tag: "application", ParentTag: "manifest", Line: 4},
			{Tag: "activity", ParentTag: "application", Line: 5},
		},
	}

	got := ConvertManifest(m, "/tmp/AndroidManifest.xml")

	if got.Path != "/tmp/AndroidManifest.xml" {
		t.Fatalf("Path = %q", got.Path)
	}
	if got.MinSDK != 24 || got.TargetSDK != 34 {
		t.Fatalf("SDKs = %d/%d, want 24/34", got.MinSDK, got.TargetSDK)
	}
	if !got.HasUsesSdk {
		t.Fatal("expected HasUsesSdk")
	}
	if got.AllowBackup == nil || !*got.AllowBackup {
		t.Fatal("expected AllowBackup=true")
	}
	if got.Debuggable == nil || *got.Debuggable {
		t.Fatal("expected Debuggable=false")
	}
	if got.UsesCleartextTraffic == nil || !*got.UsesCleartextTraffic {
		t.Fatal("expected UsesCleartextTraffic=true")
	}
	if got.FullBackupContent != "@xml/backup_rules" {
		t.Fatalf("FullBackupContent = %q", got.FullBackupContent)
	}
	if got.DataExtractionRules != "@xml/data_extraction_rules" {
		t.Fatalf("DataExtractionRules = %q", got.DataExtractionRules)
	}
	if len(got.UsesPermissions) != 1 || got.UsesPermissions[0] != "android.permission.INTERNET" {
		t.Fatalf("UsesPermissions = %#v", got.UsesPermissions)
	}
	if len(got.Permissions) != 1 || got.Permissions[0] != "com.example.app.CUSTOM_PERMISSION" {
		t.Fatalf("Permissions = %#v", got.Permissions)
	}
	if len(got.Activities) != 1 {
		t.Fatalf("Activities count = %d, want 1", len(got.Activities))
	}
	if !got.Activities[0].HasIntentFilter {
		t.Fatal("expected activity HasIntentFilter=true")
	}
	if got.Activities[0].Exported == nil || !*got.Activities[0].Exported {
		t.Fatal("expected activity Exported=true")
	}
	if len(got.Elements) < 3 {
		t.Fatalf("Elements count = %d, want at least 3", len(got.Elements))
	}
}

func TestConvertManifest_UsesTreeSitterLines(t *testing.T) {
	m := loadTestManifest(t)

	got := ConvertManifest(m, "/tmp/AndroidManifest.xml")

	if got.UsesSdkLine != 7 {
		t.Fatalf("UsesSdkLine = %d, want 7", got.UsesSdkLine)
	}
	if got.AppLine != 22 {
		t.Fatalf("AppLine = %d, want 22", got.AppLine)
	}
	if len(got.Activities) < 3 {
		t.Fatalf("Activities count = %d, want at least 3", len(got.Activities))
	}
	if got.Activities[0].Line != 33 {
		t.Fatalf("Main activity line = %d, want 33", got.Activities[0].Line)
	}
	if len(got.Services) < 2 {
		t.Fatalf("Services count = %d, want at least 2", len(got.Services))
	}
	if got.Services[0].Line != 57 {
		t.Fatalf("Sync service line = %d, want 57", got.Services[0].Line)
	}
	if len(got.Receivers) < 2 {
		t.Fatalf("Receivers count = %d, want at least 2", len(got.Receivers))
	}
	if got.Receivers[0].Line != 68 {
		t.Fatalf("Boot receiver line = %d, want 68", got.Receivers[0].Line)
	}
	if len(got.Providers) < 2 {
		t.Fatalf("Providers count = %d, want at least 2", len(got.Providers))
	}
	if got.Providers[0].Line != 82 {
		t.Fatalf("Data provider line = %d, want 82", got.Providers[0].Line)
	}
}

func TestConvertManifest_DropsUsesPermissionWithToolsNodeRemove(t *testing.T) {
	m := &Manifest{
		UsesPermissions: []UsesPermission{
			{Name: "android.permission.INTERNET"},
			{Name: "android.permission.READ_EXTERNAL_STORAGE", ToolsNode: ToolsNodeRemove},
			{Name: "android.permission.WRITE_EXTERNAL_STORAGE", ToolsNode: ToolsNodeRemoveAll},
			{Name: "android.permission.CAMERA", ToolsNode: "replace"},
		},
	}

	got := ConvertManifest(m, "/tmp/AndroidManifest.xml")

	want := []string{
		"android.permission.INTERNET",
		"android.permission.CAMERA",
	}
	if len(got.UsesPermissions) != len(want) {
		t.Fatalf("UsesPermissions = %#v, want %#v", got.UsesPermissions, want)
	}
	for i, p := range got.UsesPermissions {
		if p != want[i] {
			t.Errorf("UsesPermissions[%d] = %q, want %q", i, p, want[i])
		}
	}
}

func TestConvertManifest_PreservesLocaleConfigWithoutOtherApplicationFields(t *testing.T) {
	m := &Manifest{
		Application: Application{
			LocaleConfig: "@xml/locales_config",
		},
		Elements: []ElementInfo{
			{Tag: "manifest", ParentTag: "", Line: 1},
			{Tag: "application", ParentTag: "manifest", Line: 2},
		},
	}

	got := ConvertManifest(m, "/tmp/AndroidManifest.xml")

	if !got.HasApplication {
		t.Fatal("expected HasApplication when localeConfig is set")
	}
	if got.LocaleConfig != "@xml/locales_config" {
		t.Fatalf("LocaleConfig = %q, want %q", got.LocaleConfig, "@xml/locales_config")
	}
	if got.AppLine != 2 {
		t.Fatalf("AppLine = %d, want 2", got.AppLine)
	}
}

func TestConvertManifest_CarriesHostsAliasTargetsAndMaxSdk(t *testing.T) {
	m, err := ParseManifestBytes([]byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.app">
  <uses-permission android:name="android.permission.BLUETOOTH" android:maxSdkVersion="30"/>
  <application>
    <activity android:name=".LinkActivity">
      <intent-filter>
        <action android:name="android.intent.action.VIEW"/>
        <data android:scheme="https" android:host="example.com"/>
      </intent-filter>
    </activity>
    <activity-alias android:name=".Entry" android:targetActivity=".MainActivity">
      <intent-filter>
        <action android:name="android.intent.action.MAIN"/>
        <category android:name="android.intent.category.LAUNCHER"/>
      </intent-filter>
    </activity-alias>
  </application>
</manifest>`))
	if err != nil {
		t.Fatalf("ParseManifestBytes: %v", err)
	}
	got := ConvertManifest(m, "/tmp/AndroidManifest.xml")
	if len(got.Activities) != 2 || len(got.Activities[0].IntentFilterDataHosts) != 1 || got.Activities[0].IntentFilterDataHosts[0] != "example.com" {
		t.Fatalf("expected data host to be carried, got %#v", got.Activities)
	}
	if len(got.LauncherAliasTargets) != 1 || got.LauncherAliasTargets[0] != ".MainActivity" {
		t.Fatalf("LauncherAliasTargets = %v", got.LauncherAliasTargets)
	}
	if got.UsesPermissionMaxSdk["android.permission.BLUETOOTH"] != 30 {
		t.Fatalf("UsesPermissionMaxSdk = %v", got.UsesPermissionMaxSdk)
	}
}

func TestConvertManifest_UsesFeatureLinesAndGlEsVersion(t *testing.T) {
	src := `<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <uses-feature android:name="android.hardware.camera"/>
    <uses-feature android:glEsVersion="0x00020000"/>
    <uses-feature
        android:name="android.hardware.camera"/>
</manifest>`
	m, err := ParseManifestBytes([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	got := ConvertManifest(m, "/tmp/AndroidManifest.xml")
	if len(got.UsesFeatures) != 3 {
		t.Fatalf("UsesFeatures = %d, want 3", len(got.UsesFeatures))
	}
	wantLines := []int{3, 4, 5}
	for i, f := range got.UsesFeatures {
		if f.Line != wantLines[i] {
			t.Errorf("feature %d line = %d, want %d", i, f.Line, wantLines[i])
		}
	}
	if got.UsesFeatures[1].GlEsVersion != "0x00020000" {
		t.Errorf("GlEsVersion = %q", got.UsesFeatures[1].GlEsVersion)
	}
}

func TestConvertManifest_ProviderReadWriteAndPathPermissionsProtect(t *testing.T) {
	m := &Manifest{
		Package: "com.example.app",
		Application: Application{
			Providers: []Provider{
				{Name: ".Open", Exported: "true"},
				{Name: ".Read", Exported: "true", ReadPermission: "p.READ"},
				{Name: ".Write", Exported: "true", WritePermission: "p.WRITE"},
				{Name: ".Path", Exported: "true", PathPermissions: []struct {
					Path string `xml:"http://schemas.android.com/apk/res/android path,attr"`
				}{{Path: "/x"}}},
			},
		},
	}
	got := ConvertManifest(m, "/tmp/AndroidManifest.xml")
	var guarded []bool
	for _, p := range got.Providers {
		guarded = append(guarded, p.Permission != "")
	}
	want := []bool{false, true, true, true}
	for i := range want {
		if i >= len(guarded) || guarded[i] != want[i] {
			t.Fatalf("guarded = %v, want %v", guarded, want)
		}
	}
}

func TestConvertManifest_AliasesRemovalsAndFeatureLines(t *testing.T) {
	m, err := ParseManifestBytes([]byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools" package="com.example.app">
  <uses-feature android:name="android.hardware.camera" android:required="false"/>
  <application>
    <activity android:name=".Gone" tools:node="remove"/>
    <service android:name=".GoneSvc" tools:node="removeAll"/>
    <activity-alias android:name=".Alias" android:targetActivity=".Real" android:exported="true">
      <intent-filter><action android:name="a"/></intent-filter>
    </activity-alias>
    <activity-alias android:name=".AliasGone" android:targetActivity=".Real" tools:node="remove"/>
  </application>
</manifest>`))
	if err != nil {
		t.Fatalf("ParseManifestBytes: %v", err)
	}
	got := ConvertManifest(m, "/tmp/AndroidManifest.xml")
	if len(got.Services) != 0 {
		t.Errorf("tools:node removed service should be dropped, got %#v", got.Services)
	}
	if len(got.Activities) != 1 || got.Activities[0].Tag != "activity-alias" || got.Activities[0].Name != ".Alias" ||
		got.Activities[0].Exported == nil || !*got.Activities[0].Exported || !got.Activities[0].HasIntentFilter {
		t.Errorf("expected only the live alias as a component, got %#v", got.Activities)
	}
	if len(got.UsesFeatures) != 1 || got.UsesFeatures[0].Line != 3 {
		t.Errorf("uses-feature line = %#v, want 3", got.UsesFeatures)
	}
}

func TestConvertManifest_ToolsIgnoreAndPermissionLines(t *testing.T) {
	m, err := ParseManifestBytes([]byte(`<manifest xmlns:android="http://schemas.android.com/apk/res/android"
    xmlns:tools="http://schemas.android.com/tools" package="com.example.app" tools:ignore="Outer">
  <uses-permission android:name="android.permission.DUMP" tools:ignore="ProtectedPermissions"/>
  <uses-permission android:name="android.permission.CAMERA"/>
  <application tools:ignore="A, B">
    <activity android:name=".Main"/>
  </application>
</manifest>`))
	if err != nil {
		t.Fatalf("ParseManifestBytes: %v", err)
	}
	got := ConvertManifest(m, "/tmp/AndroidManifest.xml")
	if got.UsesPermissionLines["android.permission.DUMP"] != 3 || got.UsesPermissionLines["android.permission.CAMERA"] != 4 {
		t.Errorf("UsesPermissionLines = %v", got.UsesPermissionLines)
	}
	if want := []string{"Outer", "ProtectedPermissions"}; strings.Join(got.ToolsIgnoreByLine[3], ",") != strings.Join(want, ",") {
		t.Errorf("line 3 tokens = %v, want %v", got.ToolsIgnoreByLine[3], want)
	}
	if want := "Outer,A,B"; strings.Join(got.ToolsIgnoreByLine[6], ",") != want {
		t.Errorf("activity inherits application tokens: line 6 = %v, want %s", got.ToolsIgnoreByLine[6], want)
	}
}

func TestConvertManifest_RecordsPermissionElementLines(t *testing.T) {
	m := &Manifest{
		UsesPermissions: []UsesPermission{{Name: "android.permission.CAMERA"}, {Name: "android.permission.DUMP"}},
		Permissions:     []Permission{{Name: "com.example.CUSTOM"}},
		Elements: []ElementInfo{
			{Tag: "manifest", Line: 2},
			{Tag: "uses-permission", ParentTag: "manifest", Line: 3},
			{Tag: "uses-permission", ParentTag: "manifest", Line: 4},
			{Tag: "permission", ParentTag: "manifest", Line: 5},
			{Tag: "uses-permission", ParentTag: "application", Line: 9},
		},
	}
	cm := ConvertManifest(m, "AndroidManifest.xml")
	if cm.UsesPermissionLines["android.permission.CAMERA"] != 3 || cm.UsesPermissionLines["android.permission.DUMP"] != 4 {
		t.Fatalf("uses-permission lines = %v", cm.UsesPermissionLines)
	}
	if cm.PermissionLines["com.example.CUSTOM"] != 5 {
		t.Fatalf("permission lines = %v", cm.PermissionLines)
	}
}

func TestConvertManifestKeepsApplicationToolsNode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "AndroidManifest.xml")
	body := `<manifest xmlns:android="http://schemas.android.com/apk/res/android" xmlns:tools="http://schemas.android.com/tools"><application tools:node="merge"/></manifest>`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	got := ConvertManifest(parsed, path)
	if !got.HasApplication || got.AppToolsNode != "merge" {
		t.Fatalf("HasApplication=%v AppToolsNode=%q, want true and merge", got.HasApplication, got.AppToolsNode)
	}
}
