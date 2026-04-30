package rules_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestCopyrightYearOutdated_Positive(t *testing.T) {
	findings := runRuleByName(t, "CopyrightYearOutdated", `
/*
 * Copyright (c) 2018 Krit Authors
 */
package test

fun currentFeature() = 42
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for outdated copyright year")
	}
}

func TestCopyrightYearOutdated_Negative(t *testing.T) {
	findings := runRuleByName(t, "CopyrightYearOutdated", `
/*
 * Copyright (c) 2024 Krit Authors
 */
package test

fun currentFeature() = 42
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestMissingSpdxIdentifier_Positive(t *testing.T) {
	findings := runRuleByName(t, "MissingSpdxIdentifier", `
/*
 * Copyright 2024 Example
 */
package test

fun currentFeature() = 42
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for missing SPDX identifier")
	}
}

func TestMissingSpdxIdentifier_Negative(t *testing.T) {
	findings := runRuleByName(t, "MissingSpdxIdentifier", `
/*
 * Copyright 2024 Example
 * SPDX-License-Identifier: Apache-2.0
 */
package test

fun currentFeature() = 42
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestDependencyLicenseUnknown(t *testing.T) {
	r := findGradleRule(t, "DependencyLicenseUnknown")
	rule, ok := r.Implementation.(*rules.DependencyLicenseUnknownRule)
	if !ok {
		t.Fatalf("expected *rules.DependencyLicenseUnknownRule, got %T", r.Implementation)
	}

	originalRequireVerification := rule.RequireVerification
	defer func() { rule.RequireVerification = originalRequireVerification }()

	root := fixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "licensing", "dependency-license-unknown")
	negativeDir := filepath.Join(root, "negative", "licensing", "dependency-license-unknown")

	t.Run("positive fixture triggers when verification required", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		loadFixtureRuleConfig(t, filepath.Join(positiveDir, "krit.yml"))
		findings := runGradleFixture(t, "DependencyLicenseUnknown", filepath.Join(positiveDir, "app", "build.gradle.kts"))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Rule != "DependencyLicenseUnknown" {
			t.Fatalf("expected DependencyLicenseUnknown finding, got %s", findings[0].Rule)
		}
	})

	t.Run("negative fixture is clean when all dependencies are covered", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		loadFixtureRuleConfig(t, filepath.Join(negativeDir, "krit.yml"))
		findings := runGradleFixture(t, "DependencyLicenseUnknown", filepath.Join(negativeDir, "app", "build.gradle.kts"))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("verification disabled is clean", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		rules.ApplyConfig(loadTempConfig(t, `
licensing:
  DependencyLicenseUnknown:
    requireVerification: false
`))

		content := `dependencies {
    implementation("fixture.registry:proprietary-lib:1.0.0")
}`
		cfg, err := android.ParseBuildGradleContent(content)
		if err != nil {
			t.Fatalf("ParseBuildGradleContent: %v", err)
		}
		r2 := findGradleRule(t, "DependencyLicenseUnknown")
		findings := runGradleRule(r2, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestLgplStaticLinkingInApk(t *testing.T) {
	root := fixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "licensing", "lgpl-static-linking-in-apk")
	negativeDir := filepath.Join(root, "negative", "licensing", "lgpl-static-linking-in-apk")

	t.Run("positive fixture flags LGPL implementation in com.android.application", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		loadFixtureRuleConfig(t, filepath.Join(positiveDir, "krit.yml"))
		findings := runGradleFixture(t, "LgplStaticLinkingInApk", filepath.Join(positiveDir, "app", "build.gradle.kts"))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Rule != "LgplStaticLinkingInApk" {
			t.Fatalf("expected LgplStaticLinkingInApk finding, got %s", findings[0].Rule)
		}
	})

	t.Run("negative com.android.application without LGPL is clean", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		loadFixtureRuleConfig(t, filepath.Join(negativeDir, "krit.yml"))
		findings := runGradleFixture(t, "LgplStaticLinkingInApk", filepath.Join(negativeDir, "app", "build.gradle.kts"))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings on app, got %d", len(findings))
		}
	})

	t.Run("negative dynamic-feature with LGPL is clean", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		loadFixtureRuleConfig(t, filepath.Join(negativeDir, "krit.yml"))
		findings := runGradleFixture(t, "LgplStaticLinkingInApk", filepath.Join(negativeDir, "feature", "maps", "build.gradle.kts"))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings on dynamic-feature, got %d", len(findings))
		}
	})
}

func TestDependencyLicenseIncompatible(t *testing.T) {
	r := findGradleRule(t, "DependencyLicenseIncompatible")
	rule, ok := r.Implementation.(*rules.DependencyLicenseIncompatibleRule)
	if !ok {
		t.Fatalf("expected *rules.DependencyLicenseIncompatibleRule, got %T", r.Implementation)
	}

	originalProjectLicense := rule.ProjectLicense
	defer func() { rule.ProjectLicense = originalProjectLicense }()

	root := fixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "licensing", "dependency-license-incompatible")
	negativeDir := filepath.Join(root, "negative", "licensing", "dependency-license-incompatible")

	t.Run("positive fixture flags GPL-3.0 dep against Apache-2.0 project", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		loadFixtureRuleConfig(t, filepath.Join(positiveDir, "krit.yml"))
		findings := runGradleFixture(t, "DependencyLicenseIncompatible", filepath.Join(positiveDir, "app", "build.gradle.kts"))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Rule != "DependencyLicenseIncompatible" {
			t.Fatalf("expected DependencyLicenseIncompatible finding, got %s", findings[0].Rule)
		}
	})

	t.Run("negative fixture is clean when all dep licenses are compatible", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		loadFixtureRuleConfig(t, filepath.Join(negativeDir, "krit.yml"))
		findings := runGradleFixture(t, "DependencyLicenseIncompatible", filepath.Join(negativeDir, "app", "build.gradle.kts"))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no project license configured is clean", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()
		rule.ProjectLicense = ""

		rules.ApplyConfig(loadTempConfig(t, `
licensing:
  DependencyLicenseIncompatible:
    active: true
`))

		content := `dependencies {
    implementation("fixture.registry:gpl3-only-lib:1.0.0")
}`
		cfg, err := android.ParseBuildGradleContent(content)
		if err != nil {
			t.Fatalf("ParseBuildGradleContent: %v", err)
		}
		r2 := findGradleRule(t, "DependencyLicenseIncompatible")
		findings := runGradleRule(r2, "build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func runGradleFixture(t *testing.T, ruleName, buildPath string) []scanner.Finding {
	t.Helper()
	content, err := os.ReadFile(buildPath)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", buildPath, err)
	}
	cfg, err := android.ParseBuildGradleContent(string(content))
	if err != nil {
		t.Fatalf("ParseBuildGradleContent(%s): %v", buildPath, err)
	}
	return runGradleRule(findGradleRule(t, ruleName), buildPath, string(content), cfg)
}

func loadFixtureRuleConfig(t *testing.T, path string) {
	t.Helper()
	cfg, err := config.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig(%s): %v", path, err)
	}
	rules.ApplyConfig(cfg)
}

func snapshotDefaultInactive() func() {
	snapshot := make(map[string]bool, len(rules.DefaultInactive))
	for name, inactive := range rules.DefaultInactive {
		snapshot[name] = inactive
	}
	return func() {
		clear(rules.DefaultInactive)
		for name, inactive := range snapshot {
			rules.DefaultInactive[name] = inactive
		}
	}
}
