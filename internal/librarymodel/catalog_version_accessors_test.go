package librarymodel

import "testing"

// TestApplyCatalogVersionAccessorsResolvesEightShapes covers the eight
// SDK / JVM / Kotlin version-accessor patterns the regex registry
// pre-compiles. Each input snippet uses a single Gradle DSL shape
// and the test asserts the corresponding ProjectProfile field is
// populated from the catalog version.
func TestApplyCatalogVersionAccessorsResolvesEightShapes(t *testing.T) {
	catalog := VersionCatalog{
		Versions: map[string]string{
			"compileSdk":          "34",
			"targetSdk":           "33",
			"minSdk":              "24",
			"jvmTarget":           "17",
			"sourceCompatibility": "11",
			"targetCompatibility": "17",
			"languageVersion":     "2.0",
			"apiVersion":          "1.9",
		},
	}

	cases := []struct {
		name    string
		content string
		check   func(t *testing.T, p ProjectProfile)
	}{
		{
			name:    "compileSdk-assignment",
			content: `android { compileSdk = libs.versions.compileSdk.get() }`,
			check: func(t *testing.T, p ProjectProfile) {
				if p.CompileSdkVersion != 34 {
					t.Errorf("CompileSdkVersion = %d, want 34", p.CompileSdkVersion)
				}
			},
		},
		{
			name:    "targetSdk-assignment",
			content: `android { targetSdk = libs.versions.targetSdk.get() }`,
			check: func(t *testing.T, p ProjectProfile) {
				if p.TargetSdkVersion != 33 {
					t.Errorf("TargetSdkVersion = %d, want 33", p.TargetSdkVersion)
				}
			},
		},
		{
			name:    "minSdk-assignment",
			content: `defaultConfig { minSdk = libs.versions.minSdk.get() }`,
			check: func(t *testing.T, p ProjectProfile) {
				if p.MinSdkVersion != 24 {
					t.Errorf("MinSdkVersion = %d, want 24", p.MinSdkVersion)
				}
			},
		},
		{
			name:    "jvmTarget-set",
			content: `kotlinOptions { jvmTarget.set(libs.versions.jvmTarget.get()) }`,
			check: func(t *testing.T, p ProjectProfile) {
				if p.JVM.KotlinJvmTarget.Presence != PresencePresent {
					t.Errorf("KotlinJvmTarget must be populated")
				}
			},
		},
		{
			name:    "sourceCompatibility",
			content: `compileOptions { sourceCompatibility = libs.versions.sourceCompatibility.get() }`,
			check: func(t *testing.T, p ProjectProfile) {
				if p.JVM.SourceCompatibility.Presence != PresencePresent {
					t.Errorf("SourceCompatibility must be populated")
				}
			},
		},
		{
			name:    "targetCompatibility",
			content: `compileOptions { targetCompatibility = libs.versions.targetCompatibility.get() }`,
			check: func(t *testing.T, p ProjectProfile) {
				if p.JVM.TargetCompatibility.Presence != PresencePresent {
					t.Errorf("TargetCompatibility must be populated")
				}
			},
		},
		{
			name: "languageVersion-nearby-KotlinVersion",
			content: `kotlinOptions {
				languageVersion = KotlinVersion.fromVersion(libs.versions.languageVersion.get())
			}`,
			check: func(t *testing.T, p ProjectProfile) {
				if p.Kotlin.LanguageVersion.Presence != PresencePresent {
					t.Errorf("Kotlin.LanguageVersion must be populated")
				}
				if p.Kotlin.K2 != PresencePresent {
					t.Errorf("Kotlin.K2 must be PresencePresent for 2.x languageVersion, got %v", p.Kotlin.K2)
				}
			},
		},
		{
			name: "apiVersion-nearby-KotlinVersion",
			content: `kotlinOptions {
				apiVersion = KotlinVersion.fromVersion(libs.versions.apiVersion.get())
			}`,
			check: func(t *testing.T, p ProjectProfile) {
				if p.Kotlin.APIVersion.Presence != PresencePresent {
					t.Errorf("Kotlin.APIVersion must be populated")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var profile ProjectProfile
			applyCatalogVersionAccessors(&profile, tc.content, catalog)
			tc.check(t, profile)
		})
	}
}

func TestCatalogVersionAccessorRegexesArePackageLevelSingletons(t *testing.T) {
	// Guard against a refactor moving the regex compile back into
	// the function body (the original hot-path bug).
	if catalogAssignCompileSdkRe == nil ||
		catalogAssignTargetSdkRe == nil ||
		catalogAssignMinSdkRe == nil ||
		catalogAssignJvmTargetRe == nil ||
		catalogAssignSourceCompatRe == nil ||
		catalogAssignTargetCompatRe == nil ||
		catalogNearbyLanguageVerRe == nil ||
		catalogNearbyAPIVerRe == nil {
		t.Fatal("catalog version-accessor regexes must be package-level vars")
	}
}
