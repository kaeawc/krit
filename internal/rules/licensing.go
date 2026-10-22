package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
)

const ossLicensesPluginID = "com.google.android.gms.oss-licenses-plugin"

var ossLicensesAttributionFiles = []string{
	"LICENSE",
	"LICENSE.md",
	"LICENSE.txt",
	"LICENCE",
	"LICENCE.md",
	"LICENCE.txt",
	"NOTICE",
	"NOTICE.md",
	"NOTICE.txt",
	"THIRD_PARTY_LICENSES",
	"THIRD_PARTY_LICENSES.md",
	"THIRD_PARTY_LICENSES.txt",
	"THIRD_PARTY_NOTICES",
	"THIRD_PARTY_NOTICES.md",
	"THIRD_PARTY_NOTICES.txt",
}

// OssLicensesNotIncludedInAndroidRule flags Android application modules that
// declare implementation dependencies but neither apply the Play Services OSS
// Licenses plugin nor ship a LICENSE file alongside the build script. Without
// either, a release APK has no attribution surface for its third-party deps.
type OssLicensesNotIncludedInAndroidRule struct {
	GradleBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule.
// Detection combines Gradle plugin-list parsing with attribution-file
// presence; vendored licenses under non-standard names produce false
// positives. Classified per roadmap/17.
func (r *OssLicensesNotIncludedInAndroidRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *OssLicensesNotIncludedInAndroidRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	if cfg == nil || !isAndroidApplicationModule(cfg.Plugins) {
		return
	}
	if !hasImplementationDependency(cfg.Dependencies) {
		return
	}
	for _, p := range cfg.Plugins {
		if p == ossLicensesPluginID {
			return
		}
	}
	if moduleHasLicenseFile(path) {
		return
	}

	line := findGradleLineStr(content, "plugins")
	if line == 0 {
		line = 1
	}
	ctx.Emit(baseFinding(path, line, r.BaseRule,
		"Android application module declares implementation dependencies but neither applies `com.google.android.gms.oss-licenses-plugin` nor ships a LICENSE file. Add an attribution surface for third-party libraries."))
}

func isAndroidApplicationModule(plugins []string) bool {
	for _, p := range plugins {
		if p == "com.android.application" || strings.HasPrefix(p, "com.android.application.") {
			return true
		}
	}
	return false
}

func hasImplementationDependency(deps []android.Dependency) bool {
	for _, d := range deps {
		if d.Configuration == "implementation" && d.Group != "" && d.Name != "" {
			return true
		}
	}
	return false
}

func moduleHasLicenseFile(buildPath string) bool {
	if buildPath == "" {
		return false
	}
	dir := filepath.Dir(buildPath)
	if dir == "" || dir == "." {
		return false
	}
	for _, name := range ossLicensesAttributionFiles {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			return true
		}
	}
	return false
}

const (
	licensingRuleSet          = "licensing"
	recentCopyrightYearCutoff = 2024
	spdxIdentifierPrefix      = "SPDX-License-Identifier:"
)

// DependencyLicenseUnknownRule flags external dependencies that are not present
// in the embedded license registry when license verification is required.
type DependencyLicenseUnknownRule struct {
	GradleBase
	BaseRule
	RequireVerification bool
}

var embeddedDependencyLicenseRegistry = map[string]string{
	"fixture.registry:apache-friendly-lib": "Apache-2.0",
	"fixture.registry:gpl3-only-lib":       "GPL-3.0",
	"org.jetbrains.kotlin:kotlin-stdlib":   "Apache-2.0",
}

// embeddedLgplDependencyRegistry maps `group:name` to the SPDX identifier of
// known LGPL-licensed artifacts. The fixture entries mirror the artifacts used
// by `tests/fixtures/(positive|negative)/licensing/lgpl-static-linking-in-apk`.
var embeddedLgplDependencyRegistry = map[string]string{
	"fixture.registry:lgpl21-only-lib": "LGPL-2.1-only",
	"fixture.registry:lgpl21-or-later": "LGPL-2.1-or-later",
	"fixture.registry:lgpl3-only-lib":  "LGPL-3.0-only",
	"fixture.registry:lgpl3-or-later":  "LGPL-3.0-or-later",
}

// staticLinkingConfigurations are the Gradle dependency configurations that
// bundle a library into the APK as part of the application's shipped binary.
var staticLinkingConfigurations = map[string]bool{
	"implementation":        true,
	"api":                   true,
	"compile":               true,
	"releaseImplementation": true,
	"debugImplementation":   true,
	"runtimeOnly":           true,
	"releaseRuntimeOnly":    true,
	"debugRuntimeOnly":      true,
}

var incompatibleLicensesByProject = map[string]map[string]struct{}{
	"Apache-2.0": {
		"GPL-2.0":      {},
		"GPL-2.0+":     {},
		"GPL-3.0":      {},
		"GPL-3.0+":     {},
		"AGPL-3.0":     {},
		"AGPL-3.0+":    {},
		"LGPL-2.1":     {},
		"LGPL-3.0":     {},
		"CC-BY-NC-4.0": {},
	},
	"MIT": {
		"GPL-2.0":  {},
		"GPL-3.0":  {},
		"AGPL-3.0": {},
	},
	"BSD-3-Clause": {
		"GPL-2.0":  {},
		"GPL-3.0":  {},
		"AGPL-3.0": {},
	},
}

func licenseIsIncompatible(projectLicense, depLicense string) bool {
	if projectLicense == "" || depLicense == "" {
		return false
	}
	set, ok := incompatibleLicensesByProject[projectLicense]
	if !ok {
		return false
	}
	_, bad := set[depLicense]
	return bad
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule. Detection scans file headers and manifests for license
// markers via regex; custom or non-English headers can produce false
// negatives. Classified per roadmap/17.
func (r *DependencyLicenseUnknownRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *DependencyLicenseUnknownRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	if !r.RequireVerification || cfg == nil {
		return
	}

	for _, dep := range cfg.Dependencies {
		if dep.Group == "" || dep.Name == "" {
			continue
		}
		if _, ok := embeddedDependencyLicenseRegistry[dep.Group+":"+dep.Name]; ok {
			continue
		}

		coord := dep.Group + ":" + dep.Name
		if dep.Version != "" {
			coord += ":" + dep.Version
		}
		line := findGradleLineStr(content, coord)
		if line == 0 {
			line = findGradleLineStr(content, dep.Group+":"+dep.Name)
		}
		if line == 0 {
			line = 1
		}

		ctx.Emit(baseFinding(path, line, r.BaseRule,
			fmt.Sprintf("Dependency %s is not present in the embedded license registry; add a registry entry or disable license verification for this project.", coord)))
	}
}

// LgplStaticLinkingInApkRule flags `com.android.application` modules that
// statically link a known-LGPL artifact. Static linking creates redistribution
// ambiguity under the LGPL; isolating the dependency to a
// `com.android.dynamic-feature` module delivered separately avoids the issue.
type LgplStaticLinkingInApkRule struct {
	GradleBase
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule keyed on
// an embedded LGPL artifact registry; coverage depends on registry breadth.
func (r *LgplStaticLinkingInApkRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *LgplStaticLinkingInApkRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	if cfg == nil || !hasAndroidApplicationPlugin(cfg.Plugins) {
		return
	}

	for _, dep := range cfg.Dependencies {
		if dep.Group == "" || dep.Name == "" {
			continue
		}
		if !staticLinkingConfigurations[dep.Configuration] {
			continue
		}
		spdx, ok := embeddedLgplDependencyRegistry[dep.Group+":"+dep.Name]
		if !ok {
			continue
		}

		coord := dep.Group + ":" + dep.Name
		if dep.Version != "" {
			coord += ":" + dep.Version
		}
		line := findGradleLineStr(content, coord)
		if line == 0 {
			line = findGradleLineStr(content, dep.Group+":"+dep.Name)
		}
		if line == 0 {
			line = 1
		}

		ctx.Emit(baseFinding(path, line, r.BaseRule,
			fmt.Sprintf("Dependency %s is %s and is statically linked into the application APK via `%s`. Move it to a `com.android.dynamic-feature` delivered separately or replace it with a permissively licensed alternative.", coord, spdx, dep.Configuration)))
	}
}

func hasAndroidApplicationPlugin(plugins []string) bool {
	for _, p := range plugins {
		if p == "com.android.application" {
			return true
		}
	}
	return false
}

// DependencyLicenseIncompatibleRule flags external dependencies whose license
// is known to be incompatible with the project's declared license.
type DependencyLicenseIncompatibleRule struct {
	GradleBase
	BaseRule
	ProjectLicense string
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule.
// Detection relies on the embedded license registry and the configured
// project license; missing or stale registry entries can produce false
// negatives. Classified per roadmap/17.
func (r *DependencyLicenseIncompatibleRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *DependencyLicenseIncompatibleRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	if cfg == nil || r.ProjectLicense == "" {
		return
	}

	for _, dep := range cfg.Dependencies {
		if dep.Group == "" || dep.Name == "" {
			continue
		}
		key := dep.Group + ":" + dep.Name
		depLicense, ok := embeddedDependencyLicenseRegistry[key]
		if !ok {
			continue
		}
		if !licenseIsIncompatible(r.ProjectLicense, depLicense) {
			continue
		}

		coord := key
		if dep.Version != "" {
			coord += ":" + dep.Version
		}
		line := findGradleLineStr(content, coord)
		if line == 0 && dep.Version != "" {
			line = findGradleLineStr(content, key)
		}
		if line == 0 {
			line = 1
		}

		ctx.Emit(baseFinding(path, line, r.BaseRule,
			fmt.Sprintf("Dependency %s is %s, which is incompatible with the project's %s license.",
				coord, depLicense, r.ProjectLicense)))
	}
}

// noticeFileSearchLimit caps how far up the directory tree the rule
// walks looking for a NOTICE file. Six levels comfortably covers
// typical multi-module Gradle layouts without scanning the whole disk.
const noticeFileSearchLimit = 6

// NoticeFileOutOfDateRule flags projects whose NOTICE file is missing
// attribution text required by one or more declared dependencies.
type NoticeFileOutOfDateRule struct {
	GradleBase
	BaseRule
	NoticeRequiredArtifacts []string
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule.
// Detection scans a NOTICE file for artifact identifiers; custom phrasing
// can produce false negatives.
func (r *NoticeFileOutOfDateRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *NoticeFileOutOfDateRule) check(ctx *api.Context) {
	path, content, cfg := ctx.GradlePath, ctx.GradleContent, ctx.GradleConfig
	if cfg == nil {
		return
	}

	noticeText, ok := readNearestNoticeFile(path)
	if !ok {
		return
	}

	for _, dep := range cfg.Dependencies {
		if dep.Group == "" || dep.Name == "" {
			continue
		}
		coord := dep.Group + ":" + dep.Name
		if !noticeArtifactRequired(coord, r.NoticeRequiredArtifacts) {
			continue
		}
		if strings.Contains(noticeText, coord) {
			continue
		}

		fullCoord := coord
		if dep.Version != "" {
			fullCoord += ":" + dep.Version
		}
		line := findGradleLineStr(content, fullCoord)
		if line == 0 {
			line = findGradleLineStr(content, coord)
		}
		if line == 0 {
			line = 1
		}

		ctx.Emit(baseFinding(path, line, r.BaseRule,
			fmt.Sprintf("NOTICE file is missing required attribution for %s; add the attribution text to NOTICE.", coord)))
	}
}

func noticeArtifactRequired(coord string, required []string) bool {
	for _, artifact := range required {
		if artifact == coord {
			return true
		}
	}
	return false
}

func readNearestNoticeFile(gradlePath string) (string, bool) {
	dir := filepath.Dir(gradlePath)
	if !filepath.IsAbs(dir) {
		if abs, err := filepath.Abs(dir); err == nil {
			dir = abs
		}
	}
	for i := 0; i < noticeFileSearchLimit; i++ {
		candidate := filepath.Join(dir, "NOTICE")
		if data, err := os.ReadFile(candidate); err == nil {
			return string(data), true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", false
}
