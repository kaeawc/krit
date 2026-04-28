package librarymodel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFactsForProfile_DisablesKnownMissingRoomDependency(t *testing.T) {
	profile := ProjectProfile{
		HasGradle:                    true,
		DependencyExtractionComplete: true,
		Dependencies: []Dependency{
			{Group: "com.squareup.okhttp3", Name: "okhttp", Version: "4.12.0"},
		},
	}
	facts := FactsForProfile(profile)
	if facts.Database.Room.Enabled {
		t.Fatal("Room facts should be disabled when Gradle dependencies are known and Room is absent")
	}
	if facts.Database.SQLDelight.Enabled {
		t.Fatal("SQLDelight facts should be disabled when Gradle dependencies are known and SQLDelight is absent")
	}
}

func TestFactsForProfile_ConservativeWhenDependenciesUnresolved(t *testing.T) {
	profile := ProfileFromGradleContent("build.gradle.kts", `
plugins { id("com.android.application") }
dependencies {
    implementation(libs.androidx.room.runtime)
}
`)
	facts := FactsForProfile(profile)
	if !profile.HasUnresolvedDependencyRefs {
		t.Fatal("expected unresolved version-catalog dependency reference")
	}
	if profile.DependencyExtractionComplete {
		t.Fatal("dependency extraction should not be complete when version-catalog aliases are unresolved")
	}
	if !facts.Database.Room.Enabled {
		t.Fatal("Room facts should stay enabled when dependency aliases are unresolved")
	}
}

func TestFactsForProfile_ConservativeWithConventionPlugin(t *testing.T) {
	profile := ProfileFromGradleContent("build.gradle.kts", `
plugins {
    id("com.android.library")
    id("signal.database-convention")
}
`)
	facts := FactsForProfile(profile)
	if !profile.HasUnresolvedDependencyRefs {
		t.Fatal("expected convention plugin to make dependency extraction partial")
	}
	if profile.DependencyExtractionComplete {
		t.Fatal("dependency extraction should not be complete with convention plugins")
	}
	if !facts.Database.Room.Enabled {
		t.Fatal("Room facts should stay enabled when convention plugin dependencies may be hidden")
	}
}

func TestFactsForProfile_RecognizesRoomAndSQLDelightCoordinates(t *testing.T) {
	profile := ProfileFromGradleContent("build.gradle.kts", `
plugins { id("com.android.library") }
android {
    compileSdk = 35
}
dependencies {
    implementation("androidx.room:room-runtime:2.6.1")
    implementation("app.cash.sqldelight:runtime:2.0.2")
}
`)
	facts := FactsForProfile(profile)
	if profile.CompileSdkVersion != 35 {
		t.Fatalf("CompileSdkVersion = %d, want 35", profile.CompileSdkVersion)
	}
	if !profile.DependencyExtractionComplete {
		t.Fatal("expected direct dependency declarations to be complete enough for absence decisions")
	}
	if !facts.Database.Room.Enabled {
		t.Fatal("Room facts should be enabled for androidx.room")
	}
	if !facts.Database.SQLDelight.Enabled {
		t.Fatal("SQLDelight facts should be enabled for app.cash.sqldelight")
	}
}

func TestProfileFromGradleContent_ExtractsKnownToolingVersions(t *testing.T) {
	profile := ProfileFromGradleContent("build.gradle.kts", `
plugins {
    id("com.android.application") version "8.5.2"
    id("org.jetbrains.kotlin.android") version "2.0.21"
    id("com.google.devtools.ksp") version "2.0.21-1.0.28"
}

android {
    compileSdk = 35
    defaultConfig {
        minSdk = 26
        targetSdk = 35
    }
}

kotlin {
    compilerOptions {
        languageVersion.set(KotlinVersion.KOTLIN_2_0)
        apiVersion.set(KotlinVersion.KOTLIN_2_0)
        jvmTarget.set(JvmTarget.JVM_17)
    }
}

java {
    toolchain {
        languageVersion.set(JavaLanguageVersion.of(17))
    }
}

dependencies {
    ksp("com.google.dagger:hilt-compiler:2.51")
}
`)
	if got := profile.Kotlin.EffectiveCompilerVersion(); got != "2.0.21" {
		t.Fatalf("Kotlin compiler = %q, want 2.0.21", got)
	}
	if got := profile.Kotlin.EffectiveLanguageVersion(); got != "2.0" {
		t.Fatalf("Kotlin language = %q, want 2.0", got)
	}
	if got := profile.KSP.EffectiveVersion(); got != "2.0.21-1.0.28" {
		t.Fatalf("KSP = %q, want 2.0.21-1.0.28", got)
	}
	if len(profile.KSP.Processors) != 1 {
		t.Fatalf("KSP processors = %d, want 1", len(profile.KSP.Processors))
	}
	if got := profile.Android.EffectiveAGPVersion(); got != "8.5.2" {
		t.Fatalf("AGP = %q, want 8.5.2", got)
	}
	if got := profile.Android.EffectiveCompileSDK(); got != "35" {
		t.Fatalf("compileSdk = %q, want 35", got)
	}
	if got := profile.Android.EffectiveTargetSDK(); got != "35" {
		t.Fatalf("targetSdk = %q, want 35", got)
	}
	if got := profile.Android.EffectiveMinSDK(); got != "26" {
		t.Fatalf("minSdk = %q, want 26", got)
	}
	if got := profile.JVM.EffectiveTargetBytecode(); got != "17" {
		t.Fatalf("JVM target = %q, want 17", got)
	}
}

func TestProfileFromGradleContent_AssumesLatestForKnownPresenceUnknownVersions(t *testing.T) {
	profile := ProfileFromGradleContent("build.gradle.kts", `
plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("com.google.devtools.ksp")
}
`)
	if got := profile.Kotlin.EffectiveCompilerVersion(); got != LatestStableKotlinVersion {
		t.Fatalf("Kotlin compiler = %q, want latest stable %q", got, LatestStableKotlinVersion)
	}
	if got := profile.Kotlin.EffectiveLanguageVersion(); got != LatestStableKotlinVersion {
		t.Fatalf("Kotlin language = %q, want latest stable %q", got, LatestStableKotlinVersion)
	}
	if got := profile.KSP.EffectiveVersion(); got != LatestStableKSPVersion {
		t.Fatalf("KSP = %q, want latest stable %q", got, LatestStableKSPVersion)
	}
	if got := profile.Android.EffectiveAGPVersion(); got != LatestStableAGPVersion {
		t.Fatalf("AGP = %q, want latest stable %q", got, LatestStableAGPVersion)
	}
	if got := profile.Android.EffectiveCompileSDK(); got != LatestStableCompileSDK {
		t.Fatalf("compileSdk = %q, want latest stable %q", got, LatestStableCompileSDK)
	}
	if got := profile.Android.EffectiveTargetSDK(); got != LatestStableTargetSDK {
		t.Fatalf("targetSdk = %q, want latest stable %q", got, LatestStableTargetSDK)
	}
	if got := profile.JVM.EffectiveTargetBytecode(); got != LatestStableKotlinJvmTarget {
		t.Fatalf("JVM target = %q, want latest stable %q", got, LatestStableKotlinJvmTarget)
	}
}

func TestProfileFromGradleContent_KSPAbsentWhenNotDetected(t *testing.T) {
	profile := ProfileFromGradleContent("build.gradle.kts", `
plugins {
    id("org.jetbrains.kotlin.android")
}
`)
	if profile.KSP.Tool.Presence != PresenceUnknown {
		t.Fatalf("KSP presence = %v, want unknown", profile.KSP.Tool.Presence)
	}
	if got := profile.KSP.EffectiveVersion(); got != "" {
		t.Fatalf("KSP effective version = %q, want empty when KSP is not detected", got)
	}
}

func TestParseVersionCatalogContent_ResolvesVersionsPluginsLibraries(t *testing.T) {
	catalog := ParseVersionCatalogContent(`
[versions]
build-kotlin = "2.1.20"
glide = "4.16.0" # kept outside strings

[plugins]
kotlin-android = { id = "org.jetbrains.kotlin.android", version.ref = "build-kotlin" }
ksp = { id = "com.google.devtools.ksp", version = "2.1.20-1.0.32" }
spotless = "com.diffplug.spotless:6.25.0"

[libraries]
glide-ksp = { module = "com.github.bumptech.glide:ksp", version.ref = "glide" }
compose-ui = { group = "androidx.compose.ui", name = "ui" }
junit = "junit:junit:4.13.2"

[bundles]
compose = ["compose-ui"]
compose-multiline = [
    "compose-ui",
]
`)
	if got := catalog.Versions["build-kotlin"]; got != "2.1.20" {
		t.Fatalf("catalog version = %q, want 2.1.20", got)
	}
	if got := catalog.Plugins["kotlin-android"].Version; got != "2.1.20" {
		t.Fatalf("kotlin plugin version = %q, want 2.1.20", got)
	}
	if got := catalog.Plugins["ksp"].Version; got != "2.1.20-1.0.32" {
		t.Fatalf("ksp plugin version = %q, want 2.1.20-1.0.32", got)
	}
	if got := catalog.Plugins["spotless"]; got.ID != "com.diffplug.spotless" || got.Version != "6.25.0" {
		t.Fatalf("spotless plugin = %#v, want com.diffplug.spotless:6.25.0", got)
	}
	if got := catalog.Libraries["glide-ksp"]; got.Group != "com.github.bumptech.glide" || got.Name != "ksp" || got.Version != "4.16.0" {
		t.Fatalf("glide-ksp = %#v, want com.github.bumptech.glide:ksp:4.16.0", got)
	}
	if got := catalog.Libraries["compose-ui"]; got.Group != "androidx.compose.ui" || got.Name != "ui" {
		t.Fatalf("compose-ui = %#v, want androidx.compose.ui:ui", got)
	}
	if got := catalog.Libraries["junit"]; got.Group != "junit" || got.Name != "junit" || got.Version != "4.13.2" {
		t.Fatalf("junit = %#v, want junit:junit:4.13.2", got)
	}
	if got := catalog.Bundles["compose"]; len(got) != 1 || got[0] != "compose-ui" {
		t.Fatalf("compose bundle = %#v, want [compose-ui]", got)
	}
	if got := catalog.Bundles["compose-multiline"]; len(got) != 1 || got[0] != "compose-ui" {
		t.Fatalf("compose multiline bundle = %#v, want [compose-ui]", got)
	}
}

func TestProfileFromGradleContentWithCatalog_ResolvesSignalStyleAliases(t *testing.T) {
	catalog := ParseVersionCatalogContent(`
[versions]
build-android-agp = "8.9.0-alpha02"
build-android-compileSdk = "35"
build-android-minSdk = "21"
build-android-targetSdk = "35"
build-kotlin = "2.1.20"
build-kotlin-language = "2.0"
build-kotlin-ksp = "2.1.20-1.0.32"
build-java-target = "17"
glide = "4.16.0"

[plugins]
kotlin-android = { id = "org.jetbrains.kotlin.android", version.ref = "build-kotlin" }
android-application = { id = "com.android.application", version.ref = "build-android-agp" }
ksp = { id = "com.google.devtools.ksp", version.ref = "build-kotlin-ksp" }

[libraries]
glide-ksp = { module = "com.github.bumptech.glide:ksp", version.ref = "glide" }
androidx-core = { module = "androidx.core:core-ktx", version = "1.13.1" }

[bundles]
androidx-core = ["androidx-core"]
`)
	profile := ProfileFromGradleContentWithCatalog("app/build.gradle.kts", `
plugins {
    id("com.android.application")
    id("org.jetbrains.kotlin.android")
    id("com.google.devtools.ksp")
}

android {
    compileSdk = libs.versions.build.android.compileSdk.get().toInt()
    defaultConfig {
        minSdk = libs.versions.build.android.minSdk.get().toInt()
        targetSdk = libs.versions.build.android.targetSdk.get().toInt()
    }
}

kotlin {
    compilerOptions {
        languageVersion.set(KotlinVersion.valueOf("KOTLIN_${libs.versions.build.kotlin.language.get().replace(".", "_")}"))
        jvmTarget.set(JvmTarget.valueOf("JVM_${libs.versions.build.java.target.get()}"))
    }
}

java {
    sourceCompatibility = JavaVersion.toVersion(libs.versions.build.java.target.get())
    targetCompatibility = JavaVersion.toVersion(libs.versions.build.java.target.get())
}

dependencies {
    ksp(libs.glide.ksp)
    implementation(libs.bundles.androidx.core)
}
`, catalog)
	if profile.HasUnresolvedDependencyRefs {
		t.Fatal("all catalog references should resolve")
	}
	if !profile.DependencyExtractionComplete {
		t.Fatal("catalog-resolved dependency extraction should be complete")
	}
	if got := profile.Kotlin.EffectiveCompilerVersion(); got != "2.1.20" {
		t.Fatalf("Kotlin compiler = %q, want 2.1.20", got)
	}
	if got := profile.Kotlin.EffectiveLanguageVersion(); got != "2.0" {
		t.Fatalf("Kotlin language = %q, want 2.0", got)
	}
	if got := profile.KSP.EffectiveVersion(); got != "2.1.20-1.0.32" {
		t.Fatalf("KSP = %q, want 2.1.20-1.0.32", got)
	}
	if got := profile.Android.EffectiveAGPVersion(); got != "8.9.0-alpha02" {
		t.Fatalf("AGP = %q, want 8.9.0-alpha02", got)
	}
	if got := profile.Android.EffectiveCompileSDK(); got != "35" {
		t.Fatalf("compileSdk = %q, want 35", got)
	}
	if got := profile.Android.EffectiveTargetSDK(); got != "35" {
		t.Fatalf("targetSdk = %q, want 35", got)
	}
	if got := profile.Android.EffectiveMinSDK(); got != "21" {
		t.Fatalf("minSdk = %q, want 21", got)
	}
	if got := profile.JVM.EffectiveTargetBytecode(); got != "17" {
		t.Fatalf("JVM target = %q, want 17", got)
	}
	if len(profile.KSP.Processors) != 1 {
		t.Fatalf("KSP processors = %d, want 1", len(profile.KSP.Processors))
	}
	if got := profile.KSP.Processors[0]; got.Group != "com.github.bumptech.glide" || got.Name != "ksp" || got.Version != "4.16.0" {
		t.Fatalf("KSP processor = %#v, want com.github.bumptech.glide:ksp:4.16.0", got)
	}
	if !profile.HasDependency("androidx.core", "core-ktx") {
		t.Fatal("expected bundle dependency to expand to androidx.core:core-ktx")
	}
}

func TestProfileFromGradleContentWithCatalog_UnresolvedMissingAliasStaysConservative(t *testing.T) {
	catalog := ParseVersionCatalogContent(`
[versions]
build-kotlin = "2.1.20"

[plugins]
kotlin-android = { id = "org.jetbrains.kotlin.android", version.ref = "build-kotlin" }
`)
	profile := ProfileFromGradleContentWithCatalog("build.gradle.kts", `
plugins {
    alias(libs.plugins.kotlin.android)
}
dependencies {
    implementation(libs.androidx.room.runtime)
}
`, catalog)
	if !profile.HasUnresolvedDependencyRefs {
		t.Fatal("missing library alias should keep dependency extraction partial")
	}
	if profile.DependencyExtractionComplete {
		t.Fatal("dependency extraction should not be complete with a missing catalog alias")
	}
	if got := profile.Kotlin.EffectiveCompilerVersion(); got != "2.1.20" {
		t.Fatalf("Kotlin compiler = %q, want resolved plugin version", got)
	}
	if len(profile.UnresolvedCatalogAliases) != 1 {
		t.Fatalf("unresolved aliases = %#v, want one missing library alias", profile.UnresolvedCatalogAliases)
	}
	if got := profile.UnresolvedCatalogAliases[0]; got.Kind != CatalogAliasLibrary || got.Alias != "androidx-room-runtime" {
		t.Fatalf("unresolved alias = %#v, want library androidx-room-runtime", got)
	}
}

func TestProfileFromGradleContentWithCatalog_IgnoresCommentedCatalogAliases(t *testing.T) {
	catalog := ParseVersionCatalogContent(`
[versions]
agp = "8.7.0"
screenshot = "0.0.1-alpha07"

[plugins]
android-application = { id = "com.android.application", version.ref = "agp" }
screenshot = { id = "com.android.compose.screenshot", version.ref = "screenshot" }
`)
	profile := ProfileFromGradleContentWithCatalog("build.gradle.kts", `
plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.screenshot)
}
dependencies {
    // classpath(libs.r8)
    /* implementation(libs.androidx.security.crypto) */
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("com.example:fake-url-holder:1.0") {
        because("https://example.test/libs.missing.alias")
    }
}
`, catalog)
	if profile.HasUnresolvedDependencyRefs {
		t.Fatal("commented-out catalog aliases should not make dependency extraction partial")
	}
	if !profile.DependencyExtractionComplete {
		t.Fatal("dependency extraction should remain complete when only comments mention missing aliases")
	}
	if profile.HasDependency("com.android.tools", "r8") {
		t.Fatal("commented-out r8 alias should not be added as a dependency")
	}
	if got := profile.Android.EffectiveAGPVersion(); got != "8.7.0" {
		t.Fatalf("AGP = %q, want Android Gradle plugin version instead of screenshot plugin version", got)
	}
}

func TestParseSettingsVersionCatalogContent_ResolvesProgrammaticLibrariesAndPlugins(t *testing.T) {
	catalog := ParseSettingsVersionCatalogContent(`
dependencyResolutionManagement {
    versionCatalogs {
        libs {
            library("sqldelight-mysql", "app.cash.sqldelight", "mysql-dialect").withoutVersion()
            library("sqldelight-sqlite-dialect", "app.cash.sqldelight", "sqlite-3-18-dialect").version("2.0.0")
            plugin("sqldelight", "app.cash.sqldelight").version("2.0.0")
        }
    }
}
`)
	if got := catalog.Libraries["sqldelight-mysql"]; got.Group != "app.cash.sqldelight" || got.Name != "mysql-dialect" || got.Version != "" {
		t.Fatalf("sqldelight-mysql = %#v, want app.cash.sqldelight:mysql-dialect without version", got)
	}
	if got := catalog.Libraries["sqldelight-sqlite-dialect"]; got.Group != "app.cash.sqldelight" || got.Name != "sqlite-3-18-dialect" || got.Version != "2.0.0" {
		t.Fatalf("sqldelight-sqlite-dialect = %#v, want versioned dialect", got)
	}
	if got := catalog.Plugins["sqldelight"]; got.ID != "app.cash.sqldelight" || got.Version != "2.0.0" {
		t.Fatalf("sqldelight plugin = %#v, want app.cash.sqldelight:2.0.0", got)
	}
}

func TestProfileFromGradlePaths_LoadsMultipleCatalogsFromGradleDir(t *testing.T) {
	root := t.TempDir()
	gradleDir := filepath.Join(root, "gradle")
	appDir := filepath.Join(root, "app")
	appGradleDir := filepath.Join(appDir, "gradle")
	if err := os.MkdirAll(gradleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(appGradleDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(gradleDir, "libs.versions.toml"), `
[versions]
agp = "8.7.0"
kotlin = "2.0.21"
parentOnly = "1.0.0"
override = "1.0.0"

[plugins]
android-application = { id = "com.android.application", version.ref = "agp" }
kotlin-android = { id = "org.jetbrains.kotlin.android", version.ref = "kotlin" }

[libraries]
parent-only = { module = "com.example:parent-only", version.ref = "parentOnly" }
override = { module = "com.example:override", version.ref = "override" }
`)
	writeTestFile(t, filepath.Join(gradleDir, "testing.versions.toml"), `
[versions]
junit = "4.13.2"

[libraries]
junit = { module = "junit:junit", version.ref = "junit" }
`)
	writeTestFile(t, filepath.Join(appGradleDir, "libs.versions.toml"), `
[versions]
override = "2.0.0"

[libraries]
override = { module = "com.example:override", version.ref = "override" }
`)
	buildPath := filepath.Join(appDir, "build.gradle.kts")
	writeTestFile(t, buildPath, `
plugins {
    alias(libs.plugins.android.application)
    alias(libs.plugins.kotlin.android)
}

android {
    compileSdk = 35
}

dependencies {
    testImplementation(libs.junit)
    implementation(libs.parent.only)
    implementation(libs.override)
}
`)
	profile := ProfileFromGradlePaths([]string{buildPath})
	if profile.CatalogCompleteness != CatalogCompletenessMergedTOML {
		t.Fatalf("CatalogCompleteness = %v, want merged TOML", profile.CatalogCompleteness)
	}
	if len(profile.CatalogSources) != 3 {
		t.Fatalf("CatalogSources = %#v, want three TOML sources", profile.CatalogSources)
	}
	if profile.HasUnresolvedDependencyRefs {
		t.Fatal("expected aliases from both version catalogs to resolve")
	}
	if !profile.DependencyExtractionComplete {
		t.Fatal("expected merged catalogs to keep dependency extraction complete")
	}
	if got := profile.Android.EffectiveAGPVersion(); got != "8.7.0" {
		t.Fatalf("AGP = %q, want 8.7.0", got)
	}
	if got := profile.Kotlin.EffectiveCompilerVersion(); got != "2.0.21" {
		t.Fatalf("Kotlin = %q, want 2.0.21", got)
	}
	if !profile.HasDependency("junit", "junit") {
		t.Fatal("expected dependency from secondary version catalog")
	}
	if !profile.HasDependency("com.example", "parent-only") {
		t.Fatal("expected dependency from parent version catalog")
	}
	for _, dep := range profile.Dependencies {
		if dep.Group == "com.example" && dep.Name == "override" && dep.Version != "2.0.0" {
			t.Fatalf("override dependency version = %q, want nearest catalog version 2.0.0", dep.Version)
		}
	}
}

func TestProfileFromGradlePaths_LoadsSettingsVersionCatalogs(t *testing.T) {
	root := t.TempDir()
	fixtureDir := filepath.Join(root, "fixture")
	buildLogicDir := filepath.Join(root, "build-logic-tests", "src", "main", "kotlin")
	if err := os.MkdirAll(filepath.Join(root, "gradle"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(buildLogicDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(root, "gradle", "libs.versions.toml"), `
[versions]
kotlin = "2.2.21"

[plugins]
kotlin-jvm = { id = "org.jetbrains.kotlin.jvm", version.ref = "kotlin" }
`)
	writeTestFile(t, filepath.Join(buildLogicDir, "sqldelightTests.settings.gradle.kts"), `
dependencyResolutionManagement {
    versionCatalogs.register("libs") {
        plugin("sqldelight", "app.cash.sqldelight").version(sqldelightVersion)
    }
}
`)
	writeTestFile(t, filepath.Join(fixtureDir, "settings.gradle"), `
pluginManagement {
    includeBuild("../build-logic-tests")
}
plugins {
    id("sqldelightTests")
}
dependencyResolutionManagement {
    versionCatalogs {
        libs {
            library("sqldelight-mysql", "app.cash.sqldelight", "mysql-dialect").withoutVersion()
            library("sqldelight-module-json", "app.cash.sqldelight", "sqlite-json-module").version(sqldelightVersion)
            plugin("sqldelight", "app.cash.sqldelight").version("2.2.21")
        }
    }
}
`)
	buildPath := filepath.Join(fixtureDir, "build.gradle")
	writeTestFile(t, buildPath, `
plugins {
    alias(libs.plugins.kotlin.jvm)
    alias(libs.plugins.sqldelight)
}

dependencies {
    implementation(libs.sqldelight.mysql)
    implementation(libs.sqldelight.module.json)
}
`)
	profile := ProfileFromGradlePaths([]string{buildPath})
	if profile.HasUnresolvedDependencyRefs {
		t.Fatalf("unexpected unresolved aliases: %#v", profile.UnresolvedCatalogAliases)
	}
	if !profile.HasDependency("app.cash.sqldelight", "mysql-dialect") {
		t.Fatal("expected programmatic settings catalog library")
	}
	if !profile.HasDependency("app.cash.sqldelight", "sqlite-json-module") {
		t.Fatal("expected programmatic settings catalog library with variable version")
	}
	if got := profile.Kotlin.EffectiveCompilerVersion(); got != "2.2.21" {
		t.Fatalf("Kotlin = %q, want parent TOML catalog plugin version", got)
	}
	if profile.CatalogCompleteness != CatalogCompletenessSettingsProgrammatic {
		t.Fatalf("CatalogCompleteness = %v, want settings programmatic", profile.CatalogCompleteness)
	}
}

func TestProfileFromGradlePaths_LoadsBinarySettingsPluginCatalog(t *testing.T) {
	root := t.TempDir()
	fixtureDir := filepath.Join(root, "fixture")
	buildLogicDir := filepath.Join(root, "build-logic")
	pluginSourceDir := filepath.Join(buildLogicDir, "src", "main", "kotlin", "com", "example")
	if err := os.MkdirAll(fixtureDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(pluginSourceDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, filepath.Join(fixtureDir, "settings.gradle.kts"), `
pluginManagement {
    includeBuild("../build-logic")
}
plugins {
    id("com.example.catalog-settings")
}
`)
	writeTestFile(t, filepath.Join(buildLogicDir, "build.gradle.kts"), `
plugins {
    id("java-gradle-plugin")
}
gradlePlugin {
    plugins {
        create("catalogSettings") {
            id = "com.example.catalog-settings"
            implementationClass = "com.example.CatalogSettingsPlugin"
        }
    }
}
`)
	writeTestFile(t, filepath.Join(pluginSourceDir, "CatalogSettingsPlugin.kt"), `
package com.example

class CatalogSettingsPlugin {
    fun apply(settings: Any) {
        dependencyResolutionManagement {
            versionCatalogs {
                create("libs") {
                    library("room-runtime", "androidx.room", "room-runtime").version("2.7.0")
                }
            }
        }
    }
}
`)
	buildPath := filepath.Join(fixtureDir, "build.gradle.kts")
	writeTestFile(t, buildPath, `
dependencies {
    implementation(libs.room.runtime)
}
`)
	profile := ProfileFromGradlePaths([]string{buildPath})
	if profile.HasUnresolvedDependencyRefs {
		t.Fatalf("unexpected unresolved aliases: %#v", profile.UnresolvedCatalogAliases)
	}
	if !profile.HasDependency("androidx.room", "room-runtime") {
		t.Fatal("expected catalog library from included binary settings plugin")
	}
	if profile.CatalogCompleteness != CatalogCompletenessSettingsProgrammatic {
		t.Fatalf("CatalogCompleteness = %v, want settings programmatic", profile.CatalogCompleteness)
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
