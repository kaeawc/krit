package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func runLintGradleRule(t *testing.T, id, path, content string) []scanner.Finding {
	t.Helper()
	cfg, _ := android.ParseBuildGradleContent(content)
	return runGradleRule(findGradleRule(t, id), path, content, cfg)
}

func TestAnnotationProcessorOnCompilePathNamesTheReplacement(t *testing.T) {
	content := `dependencies {
    implementation "com.google.dagger:dagger-compiler:2.0"
    debugImplementation "com.google.dagger:dagger-compiler:2.0"
    testCompile "com.google.dagger:dagger-compiler:2.0"
    releaseApi "com.google.dagger:dagger-compiler:2.0"
    compileOnly "com.google.dagger:dagger-compiler:2.0"
    debugCompileOnly "com.google.dagger:dagger-compiler:2.0"
    kapt "com.google.dagger:dagger-compiler:2.0"
    implementation "com.google.dagger:dagger-compiler"
    implementation "com.google.dagger:dagger:2.0"
}
`
	want := []string{
		"Add annotation processor to processor path using annotationProcessor instead of implementation",
		"Add annotation processor to processor path using debugAnnotationProcessor instead of debugImplementation",
		"Add annotation processor to processor path using testAnnotationProcessor instead of testCompile",
		"Add annotation processor to processor path using releaseAnnotationProcessor instead of releaseApi",
		"Add annotation processor to processor path using annotationProcessor instead of compileOnly",
		"Add annotation processor to processor path using debugAnnotationProcessor instead of debugCompileOnly",
	}
	findings := runLintGradleRule(t, "AnnotationProcessorOnCompilePath", "build.gradle", content)
	if len(findings) != len(want) {
		t.Fatalf("expected %d findings, got %v", len(want), findings)
	}
	for i, f := range findings {
		if f.Message != want[i] || f.Line != i+2 {
			t.Errorf("finding %d = line %d %q, want line %d %q", i, f.Line, f.Message, i+2, want[i])
		}
	}
}

func TestBomWithoutPlatformLooksThroughConfigurationVariants(t *testing.T) {
	content := `dependencies {
    implementation "io.grpc:grpc-bom:1.0"
    debugApi "io.grpc:grpc-bom:1.0"
    testImplementation "io.grpc:grpc-bom:1.0"
    compileOnly "io.grpc:grpc-bom:1.0"
    implementation platform("io.grpc:grpc-bom:1.0")
    implementation enforcedPlatform("io.grpc:grpc-bom:1.0")
    implementation "io.grpc:grpc-core:1.0"
    implementation "io.grpc:grpc-bom"
    implementation(libs.grpc.bom)
}
`
	var lines []int
	for _, f := range runLintGradleRule(t, "BomWithoutPlatform", "build.gradle", content) {
		lines = append(lines, f.Line)
	}
	if len(lines) != 3 || lines[0] != 2 || lines[1] != 3 || lines[2] != 4 {
		t.Fatalf("expected lines 2-4, got %v", lines)
	}
}

func TestUseOfBundledGooglePlayServicesVersionBounds(t *testing.T) {
	content := `dependencies {
    implementation 'com.google.android.gms:play-services:5.0.0'
    implementation 'com.google.android.gms:play-services:6.4.9'
    implementation 'com.google.android.gms:play-services:6.5.0'
    implementation 'com.google.android.gms:play-services:7.0.0'
    implementation 'com.google.android.gms:play-services:12.+'
    implementation 'com.google.android.gms:play-services:5.+'
    implementation 'com.google.android.gms:play-services:+'
    implementation 'com.google.android.gms:play-services:$version'
    implementation 'com.google.android.gms:play-services-base:12.0.1'
}
`
	var lines []int
	for _, f := range runLintGradleRule(t, "UseOfBundledGooglePlayServices", "build.gradle", content) {
		lines = append(lines, f.Line)
	}
	want := []int{4, 5, 6, 8}
	if len(lines) != len(want) {
		t.Fatalf("expected lines %v, got %v", want, lines)
	}
	for i := range want {
		if lines[i] != want[i] {
			t.Fatalf("expected lines %v, got %v", want, lines)
		}
	}
}

func TestHighAppVersionCodeThreshold(t *testing.T) {
	for _, tc := range []struct {
		value string
		want  int
	}{
		{"1999999999", 0}, {"2000000000", 1}, {"2147483647", 1}, {"2147483648", 0}, {"2_000_000_000", 0}, {"'2000000000'", 0},
	} {
		content := "android {\n    defaultConfig {\n        versionCode " + tc.value + "\n    }\n}\n"
		if got := len(runLintGradleRule(t, "HighAppVersionCode", "build.gradle", content)); got != tc.want {
			t.Errorf("versionCode %s: %d findings, want %d", tc.value, got, tc.want)
		}
	}
	content := "android {\n    productFlavors {\n        one {\n            versionCode 2100000000\n        }\n    }\n}\n"
	if got := runLintGradleRule(t, "HighAppVersionCode", "build.gradle", content); len(got) != 0 {
		t.Errorf("a flavor versionCode is not checked, got %v", got)
	}
}

func TestDevModeObsoleteNeedsADevClosure(t *testing.T) {
	groovy := "android {\n    productFlavors {\n        dev {\n            minSdkVersion 21\n        }\n        prod {\n            minSdkVersion 21\n        }\n    }\n}\n"
	if got := runLintGradleRule(t, "DevModeObsolete", "build.gradle", groovy); len(got) != 1 || got[0].Line != 4 {
		t.Fatalf("expected one finding on line 4, got %v", got)
	}
	kts := "android {\n    productFlavors {\n        create(\"dev\") {\n            minSdk = 21\n        }\n    }\n}\n"
	if got := runLintGradleRule(t, "DevModeObsolete", "build.gradle.kts", kts); len(got) != 0 {
		t.Fatalf("create(\"dev\") is not a dev closure for Lint, got %v", got)
	}
}

func TestDataBindingWithoutKaptTracksPluginsInOrder(t *testing.T) {
	const binding = "android {\n    buildFeatures {\n        dataBinding = true\n    }\n}\n"
	for _, tc := range []struct {
		name, content string
		want          int
	}{
		{"kotlin without kapt", "plugins {\n    id(\"org.jetbrains.kotlin.android\")\n}\n" + binding, 1},
		{"kotlin with kapt", "plugins {\n    id(\"org.jetbrains.kotlin.android\")\n    id(\"kotlin-kapt\")\n}\n" + binding, 0},
		{"kapt applied after the block", "plugins {\n    id(\"kotlin-android\")\n}\n" + binding + "plugins {\n    id(\"kotlin-kapt\")\n}\n", 1},
		{"java module", "plugins {\n    id(\"com.android.library\")\n}\n" + binding, 0},
		{"apply plugin", "apply plugin: 'kotlin-android'\nandroid {\n    dataBinding {\n        enabled true\n    }\n}\n", 1},
		{"data binding off", "plugins {\n    id(\"kotlin-android\")\n}\nandroid {\n    buildFeatures {\n        dataBinding = false\n    }\n}\n", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := len(runLintGradleRule(t, "DataBindingWithoutKapt", "build.gradle.kts", tc.content)); got != tc.want {
				t.Fatalf("%d findings, want %d", got, tc.want)
			}
		})
	}
}

func TestNotInterpolatedMessageAndForms(t *testing.T) {
	content := "dependencies {\n    implementation 'a:b:$v'\n    implementation 'a:b:${v}'\n    implementation \"a:b:$v\"\n    implementation platform('a:b:$v')\n    implementation 'a:b:1.0'\n    implementation 'a:b'\n}\n"
	findings := runLintGradleRule(t, "NotInterpolated", "build.gradle", content)
	if len(findings) != 2 || findings[0].Line != 2 || findings[1].Line != 3 {
		t.Fatalf("expected findings on lines 2 and 3, got %v", findings)
	}
	if !strings.HasPrefix(findings[0].Message, "It looks like you are trying to substitute a version variable") {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestLintGradleRulesAreRegisteredAsAndroidLintDefaults(t *testing.T) {
	for _, id := range []string{
		"NotInterpolated", "BomWithoutPlatform", "AnnotationProcessorOnCompilePath", "UseOfBundledGooglePlayServices",
		"DevModeObsolete", "HighAppVersionCode", "DataBindingWithoutKapt",
	} {
		r := findGradleRule(t, id)
		if r.Category != "android-lint" || !r.Needs.Has(api.NeedsGradle) {
			t.Errorf("%s: category %q needs %v", id, r.Category, r.Needs)
		}
	}
}
