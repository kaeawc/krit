package android

import (
	"path/filepath"
	"runtime"
	"testing"
)

func fixtureDir() string {
	_, f, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(f), "..", "..", "tests", "fixtures", "android")
}

func loadTestManifest(t *testing.T) *Manifest {
	t.Helper()
	m, err := ParseManifest(filepath.Join(fixtureDir(), "AndroidManifest.xml"))
	if err != nil {
		t.Fatalf("ParseManifest failed: %v", err)
	}
	return m
}

func TestParseManifest_Package(t *testing.T) {
	m := loadTestManifest(t)
	if m.Package != "com.example.app" {
		t.Errorf("Package = %q, want %q", m.Package, "com.example.app")
	}
}

func TestParseManifest_VersionInfo(t *testing.T) {
	m := loadTestManifest(t)
	if m.VersionCode != "42" {
		t.Errorf("VersionCode = %q, want %q", m.VersionCode, "42")
	}
	if m.VersionName != "1.2.3" {
		t.Errorf("VersionName = %q, want %q", m.VersionName, "1.2.3")
	}
}

func TestParseManifest_SdkVersions(t *testing.T) {
	m := loadTestManifest(t)
	if m.UsesSdk.MinSdkVersion != "21" {
		t.Errorf("MinSdkVersion = %q, want %q", m.UsesSdk.MinSdkVersion, "21")
	}
	if m.UsesSdk.TargetSdkVersion != "34" {
		t.Errorf("TargetSdkVersion = %q, want %q", m.UsesSdk.TargetSdkVersion, "34")
	}
}

func TestParseManifest_UsesPermissions(t *testing.T) {
	m := loadTestManifest(t)
	if len(m.UsesPermissions) != 2 {
		t.Fatalf("UsesPermissions count = %d, want 2", len(m.UsesPermissions))
	}
	want := []string{
		"android.permission.INTERNET",
		"android.permission.ACCESS_FINE_LOCATION",
	}
	for i, up := range m.UsesPermissions {
		if up.Name != want[i] {
			t.Errorf("UsesPermissions[%d] = %q, want %q", i, up.Name, want[i])
		}
	}
}

func TestParseManifest_Permissions(t *testing.T) {
	m := loadTestManifest(t)
	if len(m.Permissions) != 1 {
		t.Fatalf("Permissions count = %d, want 1", len(m.Permissions))
	}
	p := m.Permissions[0]
	if p.Name != "com.example.app.CUSTOM_PERMISSION" {
		t.Errorf("Permission.Name = %q", p.Name)
	}
	if p.ProtectionLevel != "signature" {
		t.Errorf("Permission.ProtectionLevel = %q, want %q", p.ProtectionLevel, "signature")
	}
}

func TestParseManifest_ApplicationAttributes(t *testing.T) {
	m := loadTestManifest(t)
	app := m.Application
	if app.Debuggable != "true" {
		t.Errorf("Debuggable = %q, want %q", app.Debuggable, "true")
	}
	if app.AllowBackup != "true" {
		t.Errorf("AllowBackup = %q, want %q", app.AllowBackup, "true")
	}
	if app.UsesCleartextTraffic != "false" {
		t.Errorf("UsesCleartextTraffic = %q, want %q", app.UsesCleartextTraffic, "false")
	}
	if app.Theme != "@style/AppTheme" {
		t.Errorf("Theme = %q", app.Theme)
	}
	if app.Name != ".App" {
		t.Errorf("Application.Name = %q, want %q", app.Name, ".App")
	}
}

func TestParseManifest_ApplicationBackupAttributes(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.app">
    <application
        android:allowBackup="true"
        android:fullBackupContent="@xml/backup_rules"
        android:dataExtractionRules="@xml/data_extraction_rules" />
</manifest>`)

	m, err := ParseManifestBytes(data)
	if err != nil {
		t.Fatalf("ParseManifestBytes failed: %v", err)
	}
	if m.Application.FullBackupContent != "@xml/backup_rules" {
		t.Errorf("FullBackupContent = %q", m.Application.FullBackupContent)
	}
	if m.Application.DataExtractionRules != "@xml/data_extraction_rules" {
		t.Errorf("DataExtractionRules = %q", m.Application.DataExtractionRules)
	}
}

func TestParseManifest_ApplicationLocaleConfig(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="utf-8"?>
<manifest xmlns:android="http://schemas.android.com/apk/res/android" package="com.example.app">
    <application android:localeConfig="@xml/locales_config" />
</manifest>`)

	m, err := ParseManifestBytes(data)
	if err != nil {
		t.Fatalf("ParseManifestBytes failed: %v", err)
	}
	if m.Application.LocaleConfig != "@xml/locales_config" {
		t.Errorf("LocaleConfig = %q, want %q", m.Application.LocaleConfig, "@xml/locales_config")
	}
}

func TestParseManifest_Activities(t *testing.T) {
	m := loadTestManifest(t)
	if len(m.Application.Activities) != 3 {
		t.Fatalf("Activities count = %d, want 3", len(m.Application.Activities))
	}

	main := m.Application.Activities[0]
	if main.Name != ".MainActivity" {
		t.Errorf("Activity[0].Name = %q", main.Name)
	}
	if main.Exported != "true" {
		t.Errorf("Activity[0].Exported = %q", main.Exported)
	}
	if len(main.IntentFilters) != 1 {
		t.Fatalf("Activity[0].IntentFilters count = %d, want 1", len(main.IntentFilters))
	}
	if main.IntentFilters[0].Actions[0].Name != "android.intent.action.MAIN" {
		t.Errorf("Activity[0] action = %q", main.IntentFilters[0].Actions[0].Name)
	}
}

func TestParseManifest_Services(t *testing.T) {
	m := loadTestManifest(t)
	if len(m.Application.Services) != 2 {
		t.Fatalf("Services count = %d, want 2", len(m.Application.Services))
	}
	sync := m.Application.Services[0]
	if sync.Permission != "com.example.app.CUSTOM_PERMISSION" {
		t.Errorf("Service[0].Permission = %q", sync.Permission)
	}
}

func TestParseManifest_Receivers(t *testing.T) {
	m := loadTestManifest(t)
	if len(m.Application.Receivers) != 2 {
		t.Fatalf("Receivers count = %d, want 2", len(m.Application.Receivers))
	}
}

func TestParseManifest_Providers(t *testing.T) {
	m := loadTestManifest(t)
	if len(m.Application.Providers) != 2 {
		t.Fatalf("Providers count = %d, want 2", len(m.Application.Providers))
	}
	dp := m.Application.Providers[0]
	if dp.Authorities != "com.example.app.provider" {
		t.Errorf("Provider[0].Authorities = %q", dp.Authorities)
	}
}

func TestParseManifest_UsesFeatures(t *testing.T) {
	m := loadTestManifest(t)
	if len(m.UsesFeatures) != 1 {
		t.Fatalf("UsesFeatures count = %d, want 1", len(m.UsesFeatures))
	}
	if m.UsesFeatures[0].Name != "android.hardware.camera" {
		t.Errorf("UsesFeature.Name = %q", m.UsesFeatures[0].Name)
	}
	if m.UsesFeatures[0].Required != "true" {
		t.Errorf("UsesFeature.Required = %q", m.UsesFeatures[0].Required)
	}
}

func TestParseManifest_MetaData(t *testing.T) {
	m := loadTestManifest(t)
	if len(m.Application.MetaData) != 1 {
		t.Fatalf("MetaData count = %d, want 1", len(m.Application.MetaData))
	}
	md := m.Application.MetaData[0]
	if md.Name != "com.google.android.geo.API_KEY" {
		t.Errorf("MetaData.Name = %q", md.Name)
	}
}

func TestParseManifest_Queries(t *testing.T) {
	m := loadTestManifest(t)
	if len(m.Queries) != 1 {
		t.Fatalf("Queries count = %d, want 1", len(m.Queries))
	}
}

func TestParseManifest_Elements(t *testing.T) {
	m := loadTestManifest(t)
	if len(m.Elements) == 0 {
		t.Fatal("expected raw manifest elements to be captured")
	}

	has := func(tag, parent string) bool {
		for _, elem := range m.Elements {
			if elem.Tag == tag && elem.ParentTag == parent {
				return true
			}
		}
		return false
	}

	if !has("application", "manifest") {
		t.Error("missing <application> under <manifest>")
	}
	if !has("activity", "application") {
		t.Error("missing <activity> under <application>")
	}
	if !has("uses-permission", "manifest") {
		t.Error("missing <uses-permission> under <manifest>")
	}
}

// --- IsExported tests ---

func TestIsExported_ExplicitTrue(t *testing.T) {
	c := Component{Exported: "true"}
	if !IsExported(c, false) {
		t.Error("IsExported should return true for exported=true")
	}
}

func TestIsExported_ExplicitFalse(t *testing.T) {
	c := Component{Exported: "false"}
	if IsExported(c, true) {
		t.Error("IsExported should return false for exported=false even with intent-filters")
	}
}

func TestIsExported_UnsetWithIntentFilters(t *testing.T) {
	c := Component{Exported: ""}
	if !IsExported(c, true) {
		t.Error("IsExported should return true for unset exported with intent-filters")
	}
}

func TestIsExported_UnsetWithoutIntentFilters(t *testing.T) {
	c := Component{Exported: ""}
	if IsExported(c, false) {
		t.Error("IsExported should return false for unset exported without intent-filters")
	}
}

// --- HasPermission tests ---

func TestHasPermission_WithPermission(t *testing.T) {
	c := Component{Permission: "com.example.PERM"}
	if !HasPermission(c) {
		t.Error("HasPermission should return true")
	}
}

func TestHasPermission_WithoutPermission(t *testing.T) {
	c := Component{}
	if HasPermission(c) {
		t.Error("HasPermission should return false")
	}
}

// --- FindComponent tests ---

func TestFindComponent_BySimpleName(t *testing.T) {
	m := loadTestManifest(t)
	c := m.FindComponent("MainActivity")
	if c == nil {
		t.Fatal("FindComponent returned nil for MainActivity")
	}
	if c.Type != "activity" {
		t.Errorf("Type = %q, want activity", c.Type)
	}
}

func TestFindComponent_ByRelativeName(t *testing.T) {
	m := loadTestManifest(t)
	c := m.FindComponent(".SyncService")
	if c == nil {
		t.Fatal("FindComponent returned nil for .SyncService")
	}
	if c.Type != "service" {
		t.Errorf("Type = %q, want service", c.Type)
	}
}

func TestFindComponent_ByFullyQualifiedName(t *testing.T) {
	m := loadTestManifest(t)
	c := m.FindComponent("com.example.app.DataProvider")
	if c == nil {
		t.Fatal("FindComponent returned nil for com.example.app.DataProvider")
	}
	if c.Type != "provider" {
		t.Errorf("Type = %q, want provider", c.Type)
	}
	if c.Authorities != "com.example.app.provider" {
		t.Errorf("Authorities = %q", c.Authorities)
	}
}

func TestFindComponent_NotFound(t *testing.T) {
	m := loadTestManifest(t)
	c := m.FindComponent("NonExistent")
	if c != nil {
		t.Errorf("FindComponent should return nil for NonExistent, got %+v", c)
	}
}

// --- ExportedWithoutPermission tests ---

func TestExportedWithoutPermission(t *testing.T) {
	m := loadTestManifest(t)
	exposed := m.ExportedWithoutPermission()

	// Expected: MainActivity (exported=true, no perm), DeepLinkActivity (implicit export, no perm),
	// MessagingService (exported=true, no perm), BootReceiver (exported=true, no perm),
	// DataProvider (exported=true, no perm)
	names := make(map[string]bool)
	for _, c := range exposed {
		names[c.Name] = true
	}

	expected := []string{".MainActivity", ".DeepLinkActivity", ".MessagingService", ".BootReceiver", ".DataProvider"}
	for _, name := range expected {
		if !names[name] {
			t.Errorf("ExportedWithoutPermission missing %q", name)
		}
	}

	// SyncService has permission, should NOT appear.
	if names[".SyncService"] {
		t.Error("SyncService should not appear (has permission)")
	}
	// SettingsActivity is not exported, should NOT appear.
	if names[".SettingsActivity"] {
		t.Error("SettingsActivity should not appear (not exported)")
	}
}

// --- AllComponents tests ---

func TestAllComponents(t *testing.T) {
	m := loadTestManifest(t)
	all := m.AllComponents()
	// 3 activities + 2 services + 2 receivers + 2 providers = 9
	if len(all) != 9 {
		t.Errorf("AllComponents count = %d, want 9", len(all))
	}
}

// --- ParseManifestBytes tests ---

func TestParseManifestBytes_InvalidXML(t *testing.T) {
	_, err := ParseManifestBytes([]byte("not xml"))
	if err == nil {
		t.Error("ParseManifestBytes should fail on invalid XML")
	}
}

func TestParseManifest_FileNotFound(t *testing.T) {
	_, err := ParseManifest("/nonexistent/AndroidManifest.xml")
	if err == nil {
		t.Error("ParseManifest should fail on missing file")
	}
}

// --- matchComponentName tests ---

func TestMatchComponentName_FullyQualified(t *testing.T) {
	if !matchComponentName("com.example", ".Foo", "com.example.Foo") {
		t.Error("should match fully qualified from relative")
	}
}

func TestMatchComponentName_AbsoluteDeclared(t *testing.T) {
	if !matchComponentName("com.example", "com.other.Bar", "com.other.Bar") {
		t.Error("should match absolute declared name")
	}
}

func TestMatchComponentName_SimpleName(t *testing.T) {
	if !matchComponentName("com.example", "com.example.Baz", "Baz") {
		t.Error("should match simple name")
	}
}

func TestMatchComponentName_NoMatch(t *testing.T) {
	if matchComponentName("com.example", ".Foo", "Bar") {
		t.Error("should not match different name")
	}
}

func TestMatchComponentName_EmptyDeclared(t *testing.T) {
	if matchComponentName("com.example", "", "Foo") {
		t.Error("should not match empty declared name")
	}
}
