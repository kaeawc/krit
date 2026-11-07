package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func findGradleRule(t *testing.T, name string) *api.Rule {
	t.Helper()
	for _, r := range api.Registry {
		if r.Needs.Has(api.NeedsGradle) && r.ID == name {
			return r
		}
	}
	t.Fatalf("gradle rule %q not found in rule registry (NeedsGradle)", name)
	return nil
}

func runGradleRule(r *api.Rule, path, content string, cfg *android.BuildConfig) []scanner.Finding {
	collector := scanner.NewFindingCollector(0)
	file := scanner.ParseGradleScript(context.Background(), path, []byte(content), cfg)
	ctx := &api.Context{
		File:          file,
		GradlePath:    path,
		GradleContent: content,
		GradleConfig:  cfg,
		Rule:          r,
		Collector:     collector,
	}
	r.Check(ctx)
	return api.ContextFindings(ctx)
}

func loadTempConfig(t *testing.T, content string) *config.Config {
	t.Helper()
	path := filepath.Join(t.TempDir(), "krit.yml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// ---------------------------------------------------------------------------
// GradlePluginCompatibility
// ---------------------------------------------------------------------------

func TestGradlePluginCompatibility(t *testing.T) {
	r := findGradleRule(t, "GradlePluginCompatibility")

	t.Run("incompatible AGP and Gradle triggers", func(t *testing.T) {
		content := `plugins {
    id("com.android.application") version "8.5.0"
}
// Gradle 8.0
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("compatible versions is clean", func(t *testing.T) {
		content := `plugins {
    id("com.android.application") version "8.5.0"
}
// Gradle 8.7
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no AGP version is clean", func(t *testing.T) {
		content := `plugins {
    id("org.jetbrains.kotlin.jvm")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// StringInteger
// ---------------------------------------------------------------------------

func TestStringInteger(t *testing.T) {
	r := findGradleRule(t, "StringInteger")

	t.Run("quoted minSdk triggers", func(t *testing.T) {
		content := `android {
    defaultConfig {
        minSdk = "24"
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 3 {
			t.Fatalf("expected line 3, got %d", findings[0].Line)
		}
		const want = "Use an integer rather than a string here (replace `\"24\"` with just `24`)"
		if findings[0].Message != want {
			t.Fatalf("got %q, want %q", findings[0].Message, want)
		}
	})

	t.Run("quoted targetSdk triggers", func(t *testing.T) {
		content := `android {
    defaultConfig {
        targetSdkVersion = "34"
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("integer minSdk is clean", func(t *testing.T) {
		content := `android {
    defaultConfig {
        minSdk = 24
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// RemoteVersion
// ---------------------------------------------------------------------------

func TestRemoteVersion(t *testing.T) {
	r := findGradleRule(t, "RemoteVersion")

	t.Run("plus version triggers", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:+")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("latest.release triggers", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:latest.release")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("pinned version is clean", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:1.2.3")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// DynamicVersion
// ---------------------------------------------------------------------------

func TestDynamicVersion(t *testing.T) {
	r := findGradleRule(t, "DynamicVersion")

	t.Run("partial wildcard triggers", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:1.+")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("groovy map notation and artifact suffixes trigger", func(t *testing.T) {
		content := `dependencies {
    implementation group: 'g1', name: 'n1', version: '1.+'
    implementation(group: "g2", name: "n2", version: "2.+")
    implementation 'g3:n3:1.0.+@aar'
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings, got %v", findings)
		}
		byLine := map[int]string{}
		for _, f := range findings {
			byLine[f.Line] = f.Message
		}
		for _, line := range []int{2, 3, 4} {
			if _, ok := byLine[line]; !ok {
				t.Fatalf("no finding on line %d: %v", line, findings)
			}
		}
		if !strings.Contains(byLine[4], "`1.0.+`") {
			t.Fatalf("version should drop the @aar suffix: %q", byLine[4])
		}
	})

	t.Run("deep wildcard triggers", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:2.3.+")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("bare plus not flagged by DynamicVersion", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:+")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings (bare + handled by RemoteVersion), got %d", len(findings))
		}
	})

	t.Run("pinned version is clean", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:1.2.3")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("maven version range triggers", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:[1.0,2.0)")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("latest qualifier other than release/integration triggers", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:latest.snapshot")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("kts named-arg dynamic version triggers", func(t *testing.T) {
		content := `dependencies {
    implementation(group = "com.example", name = "lib", version = "1.+")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 2 {
			t.Fatalf("expected line 2, got %d", findings[0].Line)
		}
	})

	t.Run("kts named-arg pinned version is clean", func(t *testing.T) {
		content := `dependencies {
    implementation(group = "com.example", name = "lib", version = "1.2.3")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("commented-out dynamic coord is clean", func(t *testing.T) {
		content := `dependencies {
    // implementation("com.example:lib:1.+")
    implementation("com.example:lib:1.2.3")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d (commented-out dynamic coord must not be flagged)", len(findings))
		}
	})

	t.Run("commented-out named-arg version is clean", func(t *testing.T) {
		content := `dependencies {
    // implementation(group = "com.example", name = "lib", version = "1.+")
    implementation("com.example:lib:1.2.3")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("local implementation shadow outside dependencies block is clean", func(t *testing.T) {
		content := `fun implementation(coord: String): String = coord
val unused = implementation("com.example:lib:1.+")
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings (local shadow outside dependencies block), got %d", len(findings))
		}
	})

	t.Run("nested dependencies inside buildscript triggers", func(t *testing.T) {
		content := `buildscript {
    dependencies {
        classpath("com.example:plugin:1.+")
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("dedupes same dynamic coord", func(t *testing.T) {
		// Same dep referenced twice on the same line should not double-emit
		// when the named-arg fallback also matches.
		content := `dependencies {
    implementation("com.example:lib:1.+")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// OldTargetApi
// ---------------------------------------------------------------------------

func TestOldTargetApi(t *testing.T) {
	r := findGradleRule(t, "OldTargetApi")

	t.Run("low target triggers", func(t *testing.T) {
		content := `android {
    defaultConfig {
        targetSdk = 29
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("high target is clean", func(t *testing.T) {
		content := `android {
    defaultConfig {
        targetSdk = 34
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no target is clean", func(t *testing.T) {
		content := `android {
    defaultConfig {
        minSdk = 24
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("threshold is configurable", func(t *testing.T) {
		rule, ok := r.Implementation.(*rules.GradleOldTargetAPIRule)
		if !ok {
			t.Fatalf("expected *GradleOldTargetAPIRule, got %T", r.Implementation)
		}
		original := rule.Threshold
		defer func() { rule.Threshold = original }()

		rules.ApplyConfig(loadTempConfig(t, `
android-lint:
  OldTargetApi:
    threshold: 29
`))

		content := `android {
    defaultConfig {
        targetSdk = 29
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings with configured threshold, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// ExpiredTargetSdkVersion
// ---------------------------------------------------------------------------

func TestExpiredTargetSdkVersion(t *testing.T) {
	r := findGradleRule(t, "ExpiredTargetSdkVersion")

	t.Run("targetSdk below floor triggers", func(t *testing.T) {
		content := `android {
    defaultConfig {
        targetSdk = 28
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 || findings[0].Message != "Google Play requires that apps target API level 34 or higher." {
			t.Fatalf("unexpected findings %+v", findings)
		}
	})

	t.Run("targetSdk at floor is clean", func(t *testing.T) {
		content := `android {
    defaultConfig {
        targetSdk = 34
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("targetSdk above floor is clean", func(t *testing.T) {
		content := `android {
    defaultConfig {
        targetSdk = 35
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no targetSdk is clean", func(t *testing.T) {
		content := `android {
    defaultConfig {
        minSdk = 24
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("floor is configurable", func(t *testing.T) {
		rule, ok := r.Implementation.(*rules.ExpiredTargetSdkVersionRule)
		if !ok {
			t.Fatalf("expected *ExpiredTargetSdkVersionRule, got %T", r.Implementation)
		}
		original := rule.Floor
		defer func() { rule.Floor = original }()

		rules.ApplyConfig(loadTempConfig(t, `
android-lint:
  ExpiredTargetSdkVersion:
    floor: 30
`))

		content := `android {
    defaultConfig {
        targetSdk = 31
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings with relaxed floor, got %d", len(findings))
		}
	})

	t.Run("lower configured floor still fires when below", func(t *testing.T) {
		rule, ok := r.Implementation.(*rules.ExpiredTargetSdkVersionRule)
		if !ok {
			t.Fatalf("expected *ExpiredTargetSdkVersionRule, got %T", r.Implementation)
		}
		original := rule.Floor
		defer func() { rule.Floor = original }()

		rules.ApplyConfig(loadTempConfig(t, `
android-lint:
  ExpiredTargetSdkVersion:
    floor: 30
`))

		content := `android {
    defaultConfig {
        targetSdk = 28
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding when below the configured floor, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// DeprecatedDependency
// ---------------------------------------------------------------------------

func TestDeprecatedDependency(t *testing.T) {
	r := findGradleRule(t, "DeprecatedDependency")

	t.Run("support library triggers", func(t *testing.T) {
		content := `dependencies {
    implementation("com.android.support:appcompat-v7:28.0.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("JUnit 4 and Dagger are not deprecated", func(t *testing.T) {
		content := `dependencies {
    testImplementation("junit:junit:4.13.2")
    implementation("com.google.dagger:dagger:2.50")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		if findings := runGradleRule(r, "build.gradle.kts", content, cfg); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("androidx dependency is clean", func(t *testing.T) {
		content := `dependencies {
    implementation("androidx.appcompat:appcompat:1.6.1")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// DuplicatePlatformClasses
// ---------------------------------------------------------------------------

func TestDuplicatePlatformClasses(t *testing.T) {
	r := findGradleRule(t, "DuplicatePlatformClasses")

	t.Run("commons-logging triggers on its line", func(t *testing.T) {
		content := `dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
    implementation("commons-logging:commons-logging:1.2")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 3 {
			t.Fatalf("expected finding on line 3, got %d", findings[0].Line)
		}
	})

	t.Run("single-quote coordinate triggers", func(t *testing.T) {
		content := `dependencies {
    implementation('org.json:json:20170516')
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("apache httpclient triggers", func(t *testing.T) {
		content := `dependencies {
    implementation("org.apache.httpcomponents:httpclient:4.5.13")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("multiple platform duplicates each report", func(t *testing.T) {
		content := `dependencies {
    implementation("xpp3:xpp3:1.1.4c")
    implementation("org.khronos:opengl-api:gl1.1-android-2.1_r1")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})

	t.Run("ordinary dependency is clean", func(t *testing.T) {
		content := `dependencies {
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation("androidx.appcompat:appcompat:1.6.1")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("same name different group is clean", func(t *testing.T) {
		// Only the exact platform coordinate matters; a same-named artifact
		// from a different group must not fire.
		content := `dependencies {
    implementation("com.example.json:json:1.0.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for non-platform group, got %d", len(findings))
		}
	})

	t.Run("commented dependency is clean", func(t *testing.T) {
		content := `dependencies {
    // implementation("commons-logging:commons-logging:1.2")
    implementation("androidx.core:core-ktx:1.12.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for commented-out dependency, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// MavenLocal
// ---------------------------------------------------------------------------

func TestMavenLocal(t *testing.T) {
	r := findGradleRule(t, "MavenLocal")

	t.Run("mavenLocal triggers", func(t *testing.T) {
		content := `repositories {
    mavenLocal()
    mavenCentral()
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 2 {
			t.Fatalf("expected line 2, got %d", findings[0].Line)
		}
	})

	t.Run("no mavenLocal is clean", func(t *testing.T) {
		content := `repositories {
    mavenCentral()
    google()
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// MinSdkTooLow
// ---------------------------------------------------------------------------

func TestMinSdkTooLow(t *testing.T) {
	r := findGradleRule(t, "MinSdkTooLow")

	t.Run("low minSdk triggers", func(t *testing.T) {
		content := `android {
    defaultConfig {
        minSdk = 16
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("high minSdk is clean", func(t *testing.T) {
		content := `android {
    defaultConfig {
        minSdk = 24
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no minSdk is clean", func(t *testing.T) {
		content := `android {
    defaultConfig {
        targetSdk = 34
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("threshold is configurable", func(t *testing.T) {
		rule, ok := r.Implementation.(*rules.MinSdkTooLowRule)
		if !ok {
			t.Fatalf("expected *MinSdkTooLowRule, got %T", r.Implementation)
		}
		original := rule.Threshold
		defer func() { rule.Threshold = original }()

		rules.ApplyConfig(loadTempConfig(t, `
android-lint:
  MinSdkTooLow:
    threshold: 16
`))

		content := `android {
    defaultConfig {
        minSdk = 16
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings with configured threshold, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// GradleDeprecated
// ---------------------------------------------------------------------------

func TestGradleDeprecated(t *testing.T) {
	r := findGradleRule(t, "GradleDeprecated")

	t.Run("compile triggers", func(t *testing.T) {
		content := `dependencies {
    compile("com.example:lib:1.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 2 {
			t.Fatalf("expected line 2, got %d", findings[0].Line)
		}
	})

	t.Run("testCompile triggers", func(t *testing.T) {
		content := `dependencies {
    testCompile("junit:junit:4.13")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("provided triggers", func(t *testing.T) {
		content := `dependencies {
    provided("javax.servlet:servlet-api:3.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("apk triggers", func(t *testing.T) {
		content := `dependencies {
    apk("com.example:lib:1.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("implementation is clean", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:1.0")
    testImplementation("junit:junit:4.13")
    compileOnly("javax.servlet:servlet-api:3.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("multiple deprecated configs", func(t *testing.T) {
		content := `dependencies {
    compile("com.example:a:1.0")
    testCompile("com.example:b:1.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// GroovySetterInKotlinDsl
// ---------------------------------------------------------------------------

func TestGroovySetterInKotlinDsl(t *testing.T) {
	r := findGradleRule(t, "GroovySetterInKotlinDsl")

	t.Run("compileSdkVersion in kts triggers", func(t *testing.T) {
		content := `android {
    compileSdkVersion 34
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 2 {
			t.Fatalf("expected line 2, got %d", findings[0].Line)
		}
	})

	t.Run("minSdkVersion in kts triggers", func(t *testing.T) {
		content := `android {
    defaultConfig {
        minSdkVersion 21
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("compileSdk assignment in kts is clean", func(t *testing.T) {
		content := `android {
    compileSdk = 34
    defaultConfig {
        minSdk = 21
        targetSdk = 34
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("groovy file is ignored", func(t *testing.T) {
		content := `android {
    compileSdkVersion 34
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for .gradle file, got %d", len(findings))
		}
	})

	// Regression: the line scanner only skips whole-comment lines, so the
	// deprecated name inside a string literal or an inline comment produced
	// false positives. The .kts AST path matches only a bare identifier
	// statement followed by the un-parseable positional argument, so neither
	// lookalike fires.
	t.Run("name inside string or inline comment is clean", func(t *testing.T) {
		content := `val note = "compileSdkVersion 34 is the old way"
android {
    compileSdk = 34 // was compileSdkVersion 34
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	// The valid Kotlin call form compileSdkVersion(34) is not the Groovy
	// positional setter and must stay clean.
	t.Run("kotlin call form is clean", func(t *testing.T) {
		content := `android {
    compileSdkVersion(34)
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// GradlePath
// ---------------------------------------------------------------------------

func TestGradlePath(t *testing.T) {
	r := findGradleRule(t, "GradlePath")

	t.Run("absolute path in files triggers", func(t *testing.T) {
		content := `dependencies {
    implementation(files("/usr/local/lib/foo.jar"))
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 2 {
			t.Fatalf("expected line 2, got %d", findings[0].Line)
		}
	})

	t.Run("absolute path in fileTree triggers", func(t *testing.T) {
		content := `dependencies {
    implementation(fileTree("/opt/libs"))
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("backslash in path triggers", func(t *testing.T) {
		content := `dependencies {
    implementation(files("libs\\foo.jar"))
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("file and dir forms trigger, escapes and comments do not", func(t *testing.T) {
		content := `dependencies {
    implementation(fileTree(dir: '/opt/libs', include: ['*.jar']))
    implementation(files("C:/libs/foo.jar"))
    val f = file("/etc/key")
    implementation(files("libs/a\nb.jar")) // C:\\old
    implementation(files("${rootDir}/libs/foo.jar"))
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 3 {
			t.Fatalf("expected 3 findings (lines 2-4), got %v", findings)
		}
	})

	t.Run("relative path is clean", func(t *testing.T) {
		content := `dependencies {
    implementation(files("libs/foo.jar"))
    implementation(fileTree("libs"))
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// GradleOverrides
// ---------------------------------------------------------------------------

func TestGradleOverrides(t *testing.T) {
	r := findGradleRule(t, "GradleOverrides")

	// writeModule creates <module>/build.gradle.kts and, when manifest is not empty, src/main/AndroidManifest.xml.
	writeModule := func(t *testing.T, build, manifest string) (string, string) {
		t.Helper()
		dir := t.TempDir()
		if manifest != "" {
			if err := os.MkdirAll(filepath.Join(dir, "src", "main"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(dir, "src", "main", "AndroidManifest.xml"), []byte(manifest), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return filepath.Join(dir, "build.gradle.kts"), build
	}
	const both = `android {
    defaultConfig {
        minSdk = 24
        targetSdk = 34
    }
}
`
	const manifestWithSdk = `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <uses-sdk android:minSdkVersion="21" android:targetSdkVersion="30"/>
</manifest>`
	const manifestWithoutSdk = `<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <application/>
</manifest>`

	t.Run("manifest values overridden by both triggers per value", func(t *testing.T) {
		path, content := writeModule(t, both, manifestWithSdk)
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, path, content, cfg)
		if len(findings) != 2 {
			t.Fatalf("expected 2 findings, got %d", len(findings))
		}
		if findings[0].Line != 3 || findings[1].Line != 4 {
			t.Fatalf("expected lines 3 and 4, got %d and %d", findings[0].Line, findings[1].Line)
		}
	})

	t.Run("gradle values without manifest declarations are clean", func(t *testing.T) {
		path, content := writeModule(t, both, manifestWithoutSdk)
		cfg, _ := android.ParseBuildGradleContent(content)
		if findings := runGradleRule(r, path, content, cfg); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("module without a manifest is clean", func(t *testing.T) {
		path, content := writeModule(t, both, "")
		cfg, _ := android.ParseBuildGradleContent(content)
		if findings := runGradleRule(r, path, content, cfg); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("only the overridden value is reported", func(t *testing.T) {
		content := `android {
    defaultConfig {
        minSdk = 24
    }
}
`
		path, content := writeModule(t, content, manifestWithSdk)
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, path, content, cfg)
		if len(findings) != 1 || findings[0].Line != 3 {
			t.Fatalf("expected one finding on line 3, got %v", findings)
		}
	})
}

// ---------------------------------------------------------------------------
// ApplyPluginInKotlinDsl
// ---------------------------------------------------------------------------

func TestApplyPluginInKotlinDsl(t *testing.T) {
	r := findGradleRule(t, "ApplyPluginInKotlinDsl")

	t.Run("apply plugin in kts triggers", func(t *testing.T) {
		content := `apply plugin: 'com.android.application'

android {
    compileSdk = 34
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 1 {
			t.Fatalf("expected line 1, got %d", findings[0].Line)
		}
	})

	t.Run("apply with parentheses in kts triggers", func(t *testing.T) {
		content := `apply(plugin = "com.android.application")

android {
    compileSdk = 34
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("nested apply and apply from are clean", func(t *testing.T) {
		content := `subprojects {
    apply(plugin = "com.android.library")
}
apply(from = "plugins.gradle.kts")
`
		cfg, _ := android.ParseBuildGradleContent(content)
		if findings := runGradleRule(r, "build.gradle.kts", content, cfg); len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("plugins block in kts is clean", func(t *testing.T) {
		content := `plugins {
    id("com.android.application")
}

android {
    compileSdk = 34
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("apply plugin in groovy file is ignored", func(t *testing.T) {
		content := `apply plugin: 'com.android.application'

android {
    compileSdkVersion 34
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for .gradle file, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// AndroidGradlePluginVersion
// ---------------------------------------------------------------------------

func TestAndroidGradlePluginVersion(t *testing.T) {
	r := findGradleRule(t, "AndroidGradlePluginVersion")

	t.Run("old AGP triggers", func(t *testing.T) {
		content := `dependencies {
    classpath("com.android.tools.build:gradle:4.2.2")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 2 {
			t.Fatalf("expected line 2, got %d", findings[0].Line)
		}
	})

	t.Run("very old AGP triggers", func(t *testing.T) {
		content := `dependencies {
    classpath("com.android.tools.build:gradle:3.5.4")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("plugins block version triggers and comments do not", func(t *testing.T) {
		content := `plugins {
    id("com.android.application") version "4.2" apply false
}
// classpath 'com.android.tools.build:gradle:3.0.0'
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 || findings[0].Line != 2 {
			t.Fatalf("expected one finding on line 2, got %v", findings)
		}
	})

	t.Run("AGP 7.0.0 is clean", func(t *testing.T) {
		content := `dependencies {
    classpath("com.android.tools.build:gradle:7.0.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("AGP 8.x is clean", func(t *testing.T) {
		content := `dependencies {
    classpath("com.android.tools.build:gradle:8.5.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no AGP dependency is clean", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:lib:1.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// NewerVersionAvailable
// ---------------------------------------------------------------------------

func TestNewerVersionAvailable(t *testing.T) {
	r := findGradleRule(t, "NewerVersionAvailable")

	t.Run("outdated appcompat triggers", func(t *testing.T) {
		content := `dependencies {
    implementation("androidx.appcompat:appcompat:1.5.1")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("up-to-date appcompat is clean", func(t *testing.T) {
		content := `dependencies {
    implementation("androidx.appcompat:appcompat:1.6.1")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("outdated material triggers", func(t *testing.T) {
		content := `dependencies {
    implementation("com.google.android.material:material:1.8.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("outdated kotlin-stdlib triggers", func(t *testing.T) {
		content := `dependencies {
    implementation("org.jetbrains.kotlin:kotlin-stdlib:1.8.22")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("unknown library is clean", func(t *testing.T) {
		content := `dependencies {
    implementation("com.example:unknown-lib:0.1.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("dynamic version skipped", func(t *testing.T) {
		content := `dependencies {
    implementation("androidx.appcompat:appcompat:1.+")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings for dynamic version, got %d", len(findings))
		}
	})

	t.Run("recommended versions are configurable", func(t *testing.T) {
		rule, ok := r.Implementation.(*rules.NewerVersionAvailableRule)
		if !ok {
			t.Fatalf("expected *NewerVersionAvailableRule, got %T", r.Implementation)
		}
		defer func() { rule.RecommendedVersions = nil }()

		rules.ApplyConfig(loadTempConfig(t, `
android-lint:
  NewerVersionAvailable:
    recommendedVersions:
      - "androidx.appcompat:appcompat=1.5.0"
      - "com.example:custom-lib=2.0.0"
`))

		content := `dependencies {
    implementation("androidx.appcompat:appcompat:1.5.1")
    implementation("com.example:custom-lib:1.9.0")
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding with configured recommendations, got %d", len(findings))
		}
		if findings[0].Rule != "NewerVersionAvailable" {
			t.Fatalf("expected NewerVersionAvailable finding, got %s", findings[0].Rule)
		}
	})
}

// ---------------------------------------------------------------------------
// StringShouldBeInt
// ---------------------------------------------------------------------------

func TestStringShouldBeInt(t *testing.T) {
	r := findGradleRule(t, "StringShouldBeInt")

	t.Run("quoted compileSdk triggers", func(t *testing.T) {
		content := `android {
    compileSdk = "34"
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
	})

	t.Run("integer compileSdk is clean", func(t *testing.T) {
		content := `android {
    compileSdk = 34
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

// ---------------------------------------------------------------------------
// Registration sanity
// ---------------------------------------------------------------------------

func TestGradleRulesRegistered(t *testing.T) {
	expected := []string{
		"GradlePluginCompatibility",
		"StringInteger",
		"StringShouldBeInt",
		"DependencyLicenseUnknown",
		"RemoteVersion",
		"DynamicVersion",
		"OldTargetApi",
		"ExpiredTargetSdkVersion",
		"DeprecatedDependency",
		"DuplicatePlatformClasses",
		"MavenLocal",
		"MinSdkTooLow",
		"GradleDeprecated",
		"GradleGetter",
		"GroovySetterInKotlinDsl",
		"GradlePath",
		"GradleOverrides",
		"ApplyPluginInKotlinDsl",
		"AndroidGradlePluginVersion",
		"NewerVersionAvailable",
	}
	for _, name := range expected {
		found := false
		for _, r := range api.Registry {
			if r.Needs.Has(api.NeedsGradle) && r.ID == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected Gradle rule %q to be registered in rule registry (NeedsGradle)", name)
		}
	}
}

func TestGradleDependencyAndDeprecatedDependency_LinesAndSymbolicVersions(t *testing.T) {
	content := `dependencies {
    implementation("androidx.recyclerview:recyclerview:latest.release")
    implementation("androidx.appcompat:appcompat:1.0.0")
    implementation("com.android.support:appcompat-v7:28.0.0")
}
`
	cfg, _ := android.ParseBuildGradleContent(content)
	newer := runGradleRule(findGradleRule(t, "GradleDependency"), "build.gradle.kts", content, cfg)
	if len(newer) != 1 || newer[0].Line != 3 {
		t.Fatalf("GradleDependency: expected one finding on line 3 (latest.release skipped), got %v", newer)
	}
	deprecated := runGradleRule(findGradleRule(t, "DeprecatedDependency"), "build.gradle.kts", content, cfg)
	if len(deprecated) != 1 || deprecated[0].Line != 4 {
		t.Fatalf("DeprecatedDependency: expected one finding on line 4, got %v", deprecated)
	}
}

func TestGradlePluginCompatibility_UsesWrapperVersionAndClasspath(t *testing.T) {
	dir := t.TempDir()
	wrapper := filepath.Join(dir, "gradle", "wrapper")
	if err := os.MkdirAll(wrapper, 0o755); err != nil {
		t.Fatal(err)
	}
	props := "distributionUrl=https\\://services.gradle.org/distributions/gradle-6.1.1-bin.zip\n"
	if err := os.WriteFile(filepath.Join(wrapper, "gradle-wrapper.properties"), []byte(props), 0o644); err != nil {
		t.Fatal(err)
	}
	content := `buildscript {
    dependencies {
        classpath("com.android.tools.build:gradle:8.1.0")
    }
}
`
	cfg, _ := android.ParseBuildGradleContent(content)
	r := findGradleRule(t, "GradlePluginCompatibility")
	findings := runGradleRule(r, filepath.Join(dir, "build.gradle.kts"), content, cfg)
	if len(findings) != 1 || findings[0].Line != 3 {
		t.Fatalf("expected one finding on line 3, got %v", findings)
	}
	if !strings.Contains(findings[0].Message, "requires Gradle >= 8.0") || !strings.Contains(findings[0].Message, "6.1") {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}

	// A compatible wrapper stays clean.
	if err := os.WriteFile(filepath.Join(wrapper, "gradle-wrapper.properties"), []byte("distributionUrl=https\\://x/gradle-8.4-all.zip\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if findings := runGradleRule(r, filepath.Join(dir, "build.gradle.kts"), content, cfg); len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %v", findings)
	}
}

func TestTargetSdkRulesSkipLibraryModules(t *testing.T) {
	content := `plugins { id("com.android.library") }
android {
    defaultConfig {
        targetSdk = 28
    }
}
`
	cfg, _ := android.ParseBuildGradleContent(content)
	for _, name := range []string{"OldTargetApi", "ExpiredTargetSdkVersion"} {
		if findings := runGradleRule(findGradleRule(t, name), "build.gradle.kts", content, cfg); len(findings) != 0 {
			t.Errorf("%s: expected 0 findings for a library module, got %d", name, len(findings))
		}
	}
	app := `plugins { id("com.android.application") }
android {
    defaultConfig {
        targetSdk = 28
    }
}
`
	cfg, _ = android.ParseBuildGradleContent(app)
	for _, name := range []string{"OldTargetApi", "ExpiredTargetSdkVersion"} {
		if findings := runGradleRule(findGradleRule(t, name), "build.gradle.kts", app, cfg); len(findings) != 1 {
			t.Errorf("%s: expected 1 finding for an application module, got %d", name, len(findings))
		}
	}
}

func TestRemoteVersionAnchorsOnDependencyLine(t *testing.T) {
	r := findGradleRule(t, "RemoteVersion")
	content := `dependencies {
    // implementation("com.commented:dep:+")
    implementation("com.example:lib:+")
}
`
	cfg, _ := android.ParseBuildGradleContent(content)
	findings := runGradleRule(r, "build.gradle.kts", content, cfg)
	if len(findings) != 1 || findings[0].Line != 3 {
		t.Fatalf("expected one finding on line 3 and none for the commented dependency, got %v", findings)
	}
}

func TestNewerVersionAvailableAnchorsOnDependencyLine(t *testing.T) {
	r := findGradleRule(t, "NewerVersionAvailable")
	content := `dependencies {
    // implementation("androidx.appcompat:appcompat:0.9.0")
    implementation("androidx.appcompat:appcompat:1.0.0")
}
`
	cfg, _ := android.ParseBuildGradleContent(content)
	findings := runGradleRule(r, "build.gradle.kts", content, cfg)
	if len(findings) != 1 || findings[0].Line != 3 {
		t.Fatalf("expected one finding on line 3, got %v", findings)
	}
}

func TestStringInteger_ConvertedLiteralIsClean(t *testing.T) {
	r := findGradleRule(t, "StringInteger")
	content := `android {
    defaultConfig {
        minSdk = "24".toInt()
        targetSdk = "34".toIntOrNull() ?: 34
    }
}
`
	cfg, _ := android.ParseBuildGradleContent(content)
	if findings := runGradleRule(r, "build.gradle.kts", content, cfg); len(findings) != 0 {
		t.Fatalf("a converted literal is an integer, got %d findings", len(findings))
	}
}

func TestGradleDeprecated_LintReplacements(t *testing.T) {
	r := findGradleRule(t, "GradleDeprecated")
	cases := map[string]string{
		"compile 'a:b:1'":             "`compile` is deprecated; replace with either `api` to maintain current behavior, or `implementation` to improve build performance by not sharing this dependency transitively.",
		"debugCompile 'a:b:1'":        "`debugCompile` is deprecated; replace with either `debugApi` to maintain current behavior, or `debugImplementation` to improve build performance by not sharing this dependency transitively.",
		"testCompile 'a:b:1'":         "`testCompile` is deprecated; replace with `testImplementation`",
		"androidTestCompile 'a:b:1'":  "`androidTestCompile` is deprecated; replace with `androidTestImplementation`",
		"provided 'a:b:1'":            "`provided` is deprecated; replace with `compileOnly`",
		"apk 'a:b:1'":                 "`apk` is deprecated; replace with `runtimeOnly`",
		"releaseProvided('a:b:1')":    "`releaseProvided` is deprecated; replace with `releaseCompileOnly`",
		"implementation 'a:b:1'":      "",
		"compileOnly 'a:b:1'":         "",
		"// compile 'a:b:1'":          "",
		"classpath 'compile:foo:1.0'": "",
	}
	for line, want := range cases {
		content := "dependencies {\n    " + line + "\n}\n"
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle", content, cfg)
		switch {
		case want == "" && len(findings) != 0:
			t.Errorf("%q: expected no finding, got %+v", line, findings)
		case want != "" && (len(findings) != 1 || findings[0].Message != want):
			t.Errorf("%q: got %+v, want %q", line, findings, want)
		}
	}
}

func TestGradleDynamicVersion_LintWording(t *testing.T) {
	r := findGradleRule(t, "GradleDynamicVersion")
	content := "dependencies {\n    implementation 'com.android.support:appcompat-v7:+'\n    implementation 'g:a:1.+'\n    implementation 'g:b:1.0'\n}\n"
	cfg, _ := android.ParseBuildGradleContent(content)
	findings := runGradleRule(r, "build.gradle", content, cfg)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %+v", findings)
	}
	want := "Avoid using + in version numbers; can lead to unpredictable and unrepeatable builds (com.android.support:appcompat-v7:+)"
	if findings[0].Message != want {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

// ---------------------------------------------------------------------------
// GradleGetter (Lint's check)
// ---------------------------------------------------------------------------

func TestGradleGetter(t *testing.T) {
	r := findGradleRule(t, "GradleGetter")
	run := func(path, content string) []scanner.Finding {
		cfg, _ := android.ParseBuildGradleContent(content)
		return runGradleRule(r, path, content, cfg)
	}
	const message = "Bad method name: pick a unique method name which does not conflict with the implicit getters for the defaultConfig properties. For example, try using the prefix compute- instead of get-."

	t.Run("getter calls inside defaultConfig are reported", func(t *testing.T) {
		findings := run("build.gradle", `android {
    defaultConfig {
        versionCode getVersionCode()
        versionName getVersionName()
        minSdkVersion 21
    }
}

def getVersionCode() { return 1 }
def getVersionName() { return "1.0" }
`)
		if len(findings) != 2 || findings[0].Line != 3 || findings[1].Line != 4 || findings[0].Message != message {
			t.Fatalf("expected findings on lines 3 and 4, got %+v", findings)
		}
	})
	t.Run("Kotlin assignment form", func(t *testing.T) {
		findings := run("build.gradle.kts", `android {
    defaultConfig {
        versionCode = getVersionCode()
    }
}
`)
		if len(findings) != 1 || findings[0].Line != 3 {
			t.Fatalf("expected a finding on line 3, got %+v", findings)
		}
	})
	t.Run("other names, other blocks and comments are clean", func(t *testing.T) {
		findings := run("build.gradle", `android {
    defaultConfig {
        versionCode computeVersionCode()
        // versionName getVersionName()
        versionName "1.0"
    }
    productFlavors {
        paid {
            versionCode getVersionCode()
        }
    }
}
versionCode getVersionCode()
`)
		if len(findings) != 0 {
			t.Fatalf("expected no findings, got %+v", findings)
		}
	})
	t.Run("a nested block inside defaultConfig does not hide the following line", func(t *testing.T) {
		findings := run("build.gradle", `android {
    defaultConfig {
        manifestPlaceholders = [key: "v"]
        testInstrumentationRunnerArguments { arg "a", "b" }
        versionName getVersionName()
    }
}
`)
		if len(findings) != 1 || findings[0].Line != 5 {
			t.Fatalf("expected a finding on line 5, got %+v", findings)
		}
	})
}
