package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/config"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
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

func TestSpdxIdentifierInvalid_Positive(t *testing.T) {
	findings := runRuleByName(t, "SpdxIdentifierInvalid", `
/*
 * Copyright 2024 Example
 * SPDX-License-Identifier: Apache2
 */
package test

fun currentFeature() = 42
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for invalid SPDX identifier")
	}
}

func TestSpdxIdentifierInvalid_Negative(t *testing.T) {
	findings := runRuleByName(t, "SpdxIdentifierInvalid", `
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

func TestSpdxIdentifierInvalid_Expression(t *testing.T) {
	findings := runRuleByName(t, "SpdxIdentifierInvalid", `
/*
 * SPDX-License-Identifier: (Apache-2.0 OR MIT) AND BSD-3-Clause
 */
package test

fun currentFeature() = 42
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for valid expression, got %d", len(findings))
	}
}

func TestSpdxIdentifierInvalid_With(t *testing.T) {
	findings := runRuleByName(t, "SpdxIdentifierInvalid", `
/*
 * SPDX-License-Identifier: GPL-3.0-or-later WITH Classpath-exception-2.0
 */
package test

fun currentFeature() = 42
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for valid WITH expression, got %d", len(findings))
	}
}

func TestSpdxIdentifierInvalid_UnknownException(t *testing.T) {
	findings := runRuleByName(t, "SpdxIdentifierInvalid", `
/*
 * SPDX-License-Identifier: Apache-2.0 WITH Made-Up-Exception
 */
package test

fun currentFeature() = 42
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for unknown exception, got %d", len(findings))
	}
}

func TestSpdxIdentifierInvalid_AdditionalIdentifiers(t *testing.T) {
	var rule *rules.SpdxIdentifierInvalidRule
	for _, r := range api.Registry {
		if r.ID == "SpdxIdentifierInvalid" {
			rule = r.Implementation.(*rules.SpdxIdentifierInvalidRule)
			break
		}
	}
	if rule == nil {
		t.Fatal("SpdxIdentifierInvalid not registered")
	}
	original := rule.AdditionalIdentifiers
	defer func() { rule.AdditionalIdentifiers = original }()
	rule.AdditionalIdentifiers = []string{"ProjectInternal-1.0"}

	findings := runRuleByName(t, "SpdxIdentifierInvalid", `
/*
 * SPDX-License-Identifier: ProjectInternal-1.0
 */
package test

fun currentFeature() = 42
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings with additionalIdentifiers, got %d", len(findings))
	}
}

func TestOssLicensesNotIncludedInAndroid(t *testing.T) {
	r := findGradleRule(t, "OssLicensesNotIncludedInAndroid")

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "licensing", "oss-licenses-not-included-in-android", "app", "build.gradle.kts")
	negativePath := filepath.Join(root, "negative", "licensing", "oss-licenses-not-included-in-android", "app", "build.gradle.kts")

	read := func(p string) (string, *android.BuildConfig) {
		t.Helper()
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("ReadFile(%s): %v", p, err)
		}
		cfg, err := android.ParseBuildGradleContent(string(data))
		if err != nil {
			t.Fatalf("ParseBuildGradleContent(%s): %v", p, err)
		}
		return string(data), cfg
	}

	t.Run("positive: app module without plugin or LICENSE triggers", func(t *testing.T) {
		content, cfg := read(positivePath)
		findings := runGradleRule(r, positivePath, content, cfg)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Rule != "OssLicensesNotIncludedInAndroid" {
			t.Fatalf("unexpected rule: %s", findings[0].Rule)
		}
	})

	t.Run("negative: oss-licenses-plugin applied is clean", func(t *testing.T) {
		content, cfg := read(negativePath)
		findings := runGradleRule(r, negativePath, content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("negative: LICENSE file present is clean", func(t *testing.T) {
		dir := t.TempDir()
		buildPath := filepath.Join(dir, "build.gradle.kts")
		content := `plugins {
    id("com.android.application")
}
dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
}
`
		if err := os.WriteFile(buildPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "LICENSE"), []byte("Apache-2.0"), 0644); err != nil {
			t.Fatal(err)
		}
		cfg, err := android.ParseBuildGradleContent(content)
		if err != nil {
			t.Fatal(err)
		}
		findings := runGradleRule(r, buildPath, content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("negative: library module is clean", func(t *testing.T) {
		content := `plugins {
    id("com.android.library")
}
dependencies {
    implementation("androidx.core:core-ktx:1.12.0")
}
`
		cfg, err := android.ParseBuildGradleContent(content)
		if err != nil {
			t.Fatal(err)
		}
		findings := runGradleRule(r, "lib/build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("negative: no implementation deps is clean", func(t *testing.T) {
		content := `plugins {
    id("com.android.application")
}
`
		cfg, err := android.ParseBuildGradleContent(content)
		if err != nil {
			t.Fatal(err)
		}
		findings := runGradleRule(r, "app/build.gradle.kts", content, cfg)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
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

func TestSpdxIdentifierMismatchWithProject(t *testing.T) {
	root := fixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "licensing", "spdx-identifier-mismatch-with-project")
	negativeDir := filepath.Join(root, "negative", "licensing", "spdx-identifier-mismatch-with-project")

	t.Run("positive fixture flags SPDX id different from project license", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		loadFixtureRuleConfig(t, filepath.Join(positiveDir, "krit.yml"))
		file, err := scanner.ParseFile(context.Background(), filepath.Join(positiveDir, "Mismatch.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRuleByNameOnFile(t, "SpdxIdentifierMismatchWithProject", file)
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Rule != "SpdxIdentifierMismatchWithProject" {
			t.Fatalf("expected SpdxIdentifierMismatchWithProject finding, got %s", findings[0].Rule)
		}
	})

	t.Run("negative fixture is clean when SPDX id matches project license", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		loadFixtureRuleConfig(t, filepath.Join(negativeDir, "krit.yml"))
		file, err := scanner.ParseFile(context.Background(), filepath.Join(negativeDir, "Match.kt"))
		if err != nil {
			t.Fatalf("ParseFile: %v", err)
		}
		findings := runRuleByNameOnFile(t, "SpdxIdentifierMismatchWithProject", file)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no project license configured is clean", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		var rule *rules.SpdxIdentifierMismatchWithProjectRule
		for _, r := range api.Registry {
			if r.ID == "SpdxIdentifierMismatchWithProject" {
				rule = r.Implementation.(*rules.SpdxIdentifierMismatchWithProjectRule)
				break
			}
		}
		if rule == nil {
			t.Fatal("SpdxIdentifierMismatchWithProject not registered")
		}
		original := rule.ProjectLicense
		defer func() { rule.ProjectLicense = original }()
		rule.ProjectLicense = ""

		rules.ApplyConfig(loadTempConfig(t, `
licensing:
  SpdxIdentifierMismatchWithProject:
    active: true
`))
		findings := runRuleByName(t, "SpdxIdentifierMismatchWithProject", `
/*
 * SPDX-License-Identifier: MIT
 */
package test

fun f() = 1
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("no SPDX header is clean", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()

		rules.ApplyConfig(loadTempConfig(t, `
licensing:
  SpdxIdentifierMismatchWithProject:
    active: true
    projectLicense: Apache-2.0
`))
		findings := runRuleByName(t, "SpdxIdentifierMismatchWithProject", `
/*
 * Copyright 2024 Example
 */
package test

fun f() = 1
`)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})
}

func TestNoticeFileOutOfDate(t *testing.T) {
	root := fixtureRoot(t)
	positiveDir := filepath.Join(root, "positive", "licensing", "notice-file-out-of-date")
	negativeDir := filepath.Join(root, "negative", "licensing", "notice-file-out-of-date")

	t.Run("positive fixture flags missing attribution", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()
		loadFixtureRuleConfig(t, filepath.Join(positiveDir, "krit.yml"))
		findings := runGradleFixture(t, "NoticeFileOutOfDate", filepath.Join(positiveDir, "app", "build.gradle.kts"))
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if findings[0].Rule != "NoticeFileOutOfDate" {
			t.Fatalf("expected NoticeFileOutOfDate finding, got %s", findings[0].Rule)
		}
	})

	t.Run("negative fixture is clean when NOTICE covers attribution", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()
		loadFixtureRuleConfig(t, filepath.Join(negativeDir, "krit.yml"))
		findings := runGradleFixture(t, "NoticeFileOutOfDate", filepath.Join(negativeDir, "app", "build.gradle.kts"))
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d", len(findings))
		}
	})

	t.Run("missing NOTICE file does not trigger", func(t *testing.T) {
		restoreDefaults := snapshotDefaultInactive()
		defer restoreDefaults()
		dir := t.TempDir()
		buildPath := filepath.Join(dir, "build.gradle.kts")
		content := `dependencies {
    implementation("com.example:attrib-required-lib:1.2.3")
}`
		if err := os.WriteFile(buildPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		findings := runGradleFixture(t, "NoticeFileOutOfDate", buildPath)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings (no NOTICE), got %d", len(findings))
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
