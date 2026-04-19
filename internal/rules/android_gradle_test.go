package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func findGradleRule(t *testing.T, name string) *v2rules.Rule {
	t.Helper()
	for _, r := range v2rules.Registry {
		if r.Needs.Has(v2rules.NeedsGradle) && r.ID == name {
			return r
		}
	}
	t.Fatalf("gradle rule %q not found in v2 Registry (NeedsGradle)", name)
	return nil
}

func runGradleRule(r *v2rules.Rule, path, content string, cfg *android.BuildConfig) []scanner.Finding {
	collector := scanner.NewFindingCollector(0)
	ctx := &v2rules.Context{
		GradlePath:    path,
		GradleContent: content,
		GradleConfig:  cfg,
		Rule:          r,
		Collector:     collector,
	}
	r.Check(ctx)
	return v2rules.ContextFindings(ctx)
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
		rule, ok := r.OriginalV1.(*rules.GradleOldTargetApiRule)
		if !ok {
			t.Fatalf("expected *GradleOldTargetApiRule, got %T", r.OriginalV1)
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
		rule, ok := r.OriginalV1.(*rules.MinSdkTooLowRule)
		if !ok {
			t.Fatalf("expected *MinSdkTooLowRule, got %T", r.OriginalV1)
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
// GradleGetter
// ---------------------------------------------------------------------------

func TestGradleGetter(t *testing.T) {
	r := findGradleRule(t, "GradleGetter")

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

	t.Run("both minSdk and targetSdk triggers", func(t *testing.T) {
		content := `android {
    defaultConfig {
        minSdk = 24
        targetSdk = 34
    }
}
`
		cfg, _ := android.ParseBuildGradleContent(content)
		findings := runGradleRule(r, "build.gradle.kts", content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Line != 4 {
			t.Fatalf("expected line 4, got %d", findings[0].Line)
		}
	})

	t.Run("only minSdk is clean", func(t *testing.T) {
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

	t.Run("only targetSdk is clean", func(t *testing.T) {
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

	t.Run("neither is clean", func(t *testing.T) {
		content := `android {
    defaultConfig {
        applicationId = "com.example.app"
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
// GradleIdeError
// ---------------------------------------------------------------------------

func TestGradleIdeError(t *testing.T) {
	r := findGradleRule(t, "GradleIdeError")

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
		rule, ok := r.OriginalV1.(*rules.NewerVersionAvailableRule)
		if !ok {
			t.Fatalf("expected *NewerVersionAvailableRule, got %T", r.OriginalV1)
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
		"DeprecatedDependency",
		"MavenLocal",
		"MinSdkTooLow",
		"GradleDeprecated",
		"GradleGetter",
		"GradlePath",
		"GradleOverrides",
		"GradleIdeError",
		"AndroidGradlePluginVersion",
		"NewerVersionAvailable",
	}
	for _, name := range expected {
		found := false
		for _, r := range v2rules.Registry {
			if r.Needs.Has(v2rules.NeedsGradle) && r.ID == name {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected Gradle rule %q to be registered in v2 Registry (NeedsGradle)", name)
		}
	}
}
