package android

import (
	"testing"
)

const sampleKtsBuild = `
plugins {
    id("com.android.application") version "8.5.0"
    id("org.jetbrains.kotlin.android")
}

android {
    compileSdk = 34

    defaultConfig {
        minSdk = 24
        targetSdk = 34
    }

    buildFeatures {
        compose = true
        viewBinding = true
    }
}

dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("com.android.support:appcompat-v7:28.0.0")
    testImplementation("junit:junit:4.13.2")
    implementation("org.jetbrains.kotlinx:kotlinx-coroutines-core")
}
`

const sampleGroovyBuild = `
apply plugin: 'com.android.library'

android {
    compileSdkVersion 33

    defaultConfig {
        minSdkVersion 21
        targetSdkVersion 33
    }
}

repositories {
    mavenLocal()
}

dependencies {
    implementation "io.reactivex.rxjava2:rxjava:2.2.21"
}
`

// --- ParseBuildGradleContent tests ---

func TestParseKtsBuild(t *testing.T) {
	cfg, err := ParseBuildGradleContent(sampleKtsBuild)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MinSdkVersion != 24 {
		t.Errorf("MinSdkVersion = %d, want 24", cfg.MinSdkVersion)
	}
	if cfg.TargetSdkVersion != 34 {
		t.Errorf("TargetSdkVersion = %d, want 34", cfg.TargetSdkVersion)
	}
	if cfg.CompileSdkVersion != 34 {
		t.Errorf("CompileSdkVersion = %d, want 34", cfg.CompileSdkVersion)
	}
	if !cfg.IsAndroid {
		t.Error("IsAndroid = false, want true")
	}
	if !cfg.BuildFeatures.Compose {
		t.Error("Compose = false, want true")
	}
	if !cfg.BuildFeatures.ViewBinding {
		t.Error("ViewBinding = false, want true")
	}
	if cfg.BuildFeatures.DataBinding {
		t.Error("DataBinding = true, want false")
	}

	// Check plugins.
	wantPlugins := map[string]bool{
		"com.android.application":        true,
		"org.jetbrains.kotlin.android":   true,
	}
	for _, p := range cfg.Plugins {
		delete(wantPlugins, p)
	}
	if len(wantPlugins) > 0 {
		t.Errorf("missing plugins: %v", wantPlugins)
	}

	// Check dependencies.
	if len(cfg.Dependencies) < 3 {
		t.Fatalf("got %d deps, want at least 3", len(cfg.Dependencies))
	}
	found := false
	for _, d := range cfg.Dependencies {
		if d.Group == "androidx.core" && d.Name == "core-ktx" && d.Version == "1.12.0" {
			found = true
		}
	}
	if !found {
		t.Error("did not find androidx.core:core-ktx:1.12.0 dependency")
	}

	// Check no-version dependency.
	foundNoVersion := false
	for _, d := range cfg.Dependencies {
		if d.Group == "org.jetbrains.kotlinx" && d.Name == "kotlinx-coroutines-core" && d.Version == "" {
			foundNoVersion = true
		}
	}
	if !foundNoVersion {
		t.Error("did not find version-less kotlinx-coroutines-core dependency")
	}
}

func TestParseGroovyBuild(t *testing.T) {
	cfg, err := ParseBuildGradleContent(sampleGroovyBuild)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.MinSdkVersion != 21 {
		t.Errorf("MinSdkVersion = %d, want 21", cfg.MinSdkVersion)
	}
	if cfg.TargetSdkVersion != 33 {
		t.Errorf("TargetSdkVersion = %d, want 33", cfg.TargetSdkVersion)
	}
	if cfg.CompileSdkVersion != 33 {
		t.Errorf("CompileSdkVersion = %d, want 33", cfg.CompileSdkVersion)
	}
	if !cfg.IsAndroid {
		t.Error("IsAndroid = false, want true")
	}
}

func TestNonAndroidProject(t *testing.T) {
	content := `
plugins {
    id("org.jetbrains.kotlin.jvm")
}

dependencies {
    implementation("io.ktor:ktor-server-core:2.3.0")
}
`
	cfg, err := ParseBuildGradleContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.IsAndroid {
		t.Error("IsAndroid = true, want false for pure Kotlin/JVM project")
	}
	if len(cfg.Dependencies) != 1 {
		t.Errorf("got %d deps, want 1", len(cfg.Dependencies))
	}
}

func TestEmptyBuildFile(t *testing.T) {
	cfg, err := ParseBuildGradleContent("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MinSdkVersion != 0 {
		t.Errorf("MinSdkVersion = %d, want 0", cfg.MinSdkVersion)
	}
	if cfg.IsAndroid {
		t.Error("IsAndroid = true, want false")
	}
}

// --- Lint tests ---

func TestLintMinSdkTooLow(t *testing.T) {
	cfg, _ := ParseBuildGradleContent(sampleKtsBuild)
	// Threshold 26 → minSdk 24 should trigger.
	findings := LintBuildGradle(sampleKtsBuild, cfg, 26)
	found := false
	for _, f := range findings {
		if f.Rule == "MinSdkTooLow" {
			found = true
			if f.Line == 0 {
				t.Error("MinSdkTooLow line number should not be 0")
			}
		}
	}
	if !found {
		t.Error("expected MinSdkTooLow finding for minSdk=24 with threshold=26")
	}

	// Threshold 21 → should NOT trigger.
	findings = LintBuildGradle(sampleKtsBuild, cfg, 21)
	for _, f := range findings {
		if f.Rule == "MinSdkTooLow" {
			t.Error("MinSdkTooLow should not trigger when minSdk >= threshold")
		}
	}
}

func TestLintDeprecatedDependency(t *testing.T) {
	cfg, _ := ParseBuildGradleContent(sampleKtsBuild)
	findings := LintBuildGradle(sampleKtsBuild, cfg, 1)

	deprecatedCount := 0
	for _, f := range findings {
		if f.Rule == "DeprecatedDependency" {
			deprecatedCount++
		}
	}
	// appcompat-v7 and junit should both be flagged.
	if deprecatedCount < 2 {
		t.Errorf("expected at least 2 DeprecatedDependency findings, got %d", deprecatedCount)
	}
}

func TestLintMavenLocal(t *testing.T) {
	cfg, _ := ParseBuildGradleContent(sampleGroovyBuild)
	findings := LintBuildGradle(sampleGroovyBuild, cfg, 1)

	found := false
	for _, f := range findings {
		if f.Rule == "MavenLocal" {
			found = true
			if f.Line == 0 {
				t.Error("MavenLocal line number should not be 0")
			}
		}
	}
	if !found {
		t.Error("expected MavenLocal finding for mavenLocal()")
	}
}

func TestLintMavenLocalAbsent(t *testing.T) {
	cfg, _ := ParseBuildGradleContent(sampleKtsBuild)
	findings := LintBuildGradle(sampleKtsBuild, cfg, 1)

	for _, f := range findings {
		if f.Rule == "MavenLocal" {
			t.Error("MavenLocal should not trigger when mavenLocal() is absent")
		}
	}
}

func TestLintGradleCompatibility(t *testing.T) {
	content := `
plugins {
    id("com.android.application") version "8.5.0"
}
// Gradle 8.2
`
	cfg, _ := ParseBuildGradleContent(content)
	findings := LintBuildGradle(content, cfg, 1)

	found := false
	for _, f := range findings {
		if f.Rule == "GradleCompatibility" {
			found = true
			if f.Line == 0 {
				t.Error("GradleCompatibility line number should not be 0")
			}
		}
	}
	if !found {
		t.Error("expected GradleCompatibility finding: AGP 8.5 requires Gradle >= 8.7 but found 8.2")
	}
}

func TestLintGradleCompatibilityOK(t *testing.T) {
	content := `
plugins {
    id("com.android.application") version "8.5.0"
}
// Gradle 8.9
`
	cfg, _ := ParseBuildGradleContent(content)
	findings := LintBuildGradle(content, cfg, 1)

	for _, f := range findings {
		if f.Rule == "GradleCompatibility" {
			t.Error("GradleCompatibility should not trigger when Gradle version is sufficient")
		}
	}
}

func TestVersionLessThan(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"8.2", "8.7", true},
		{"8.7", "8.7", false},
		{"8.9", "8.7", false},
		{"7.5", "8.0", true},
		{"8.0", "7.5", false},
	}
	for _, tc := range tests {
		got := versionLessThan(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("versionLessThan(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestMajorMinor(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"8.5.0", "8.5"},
		{"7.4.2", "7.4"},
		{"8.5", "8.5"},
		{"8", ""},
	}
	for _, tc := range tests {
		got := majorMinor(tc.in)
		if got != tc.want {
			t.Errorf("majorMinor(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestBuildFeatures(t *testing.T) {
	content := `
android {
    buildFeatures {
        compose = true
        dataBinding = true
        aidl = true
    }
}
`
	cfg, err := ParseBuildGradleContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.BuildFeatures.Compose {
		t.Error("Compose = false, want true")
	}
	if !cfg.BuildFeatures.DataBinding {
		t.Error("DataBinding = false, want true")
	}
	if !cfg.BuildFeatures.AidL {
		t.Error("AidL = false, want true")
	}
	if cfg.BuildFeatures.ViewBinding {
		t.Error("ViewBinding = true, want false")
	}
	if cfg.BuildFeatures.BuildConfig {
		t.Error("BuildConfig = true, want false")
	}
}

func TestPluginAlias(t *testing.T) {
	content := `
plugins {
    alias(libs.plugins.androidApplication)
    alias(libs.plugins.kotlinAndroid)
}
`
	cfg, err := ParseBuildGradleContent(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Plugins) != 2 {
		t.Errorf("got %d plugins, want 2", len(cfg.Plugins))
	}
}

func TestFindLineNumber(t *testing.T) {
	content := "line1\nline2\n    minSdk = 24\nline4"
	line := findLineNumber(content, minSdkRe)
	if line != 3 {
		t.Errorf("findLineNumber = %d, want 3", line)
	}
}
