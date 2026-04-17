package android

import "testing"

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
