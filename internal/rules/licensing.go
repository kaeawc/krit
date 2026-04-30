package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	v2 "github.com/kaeawc/krit/internal/rules/v2"
)

const (
	licensingRuleSet          = "licensing"
	recentCopyrightYearCutoff = 2024
	spdxIdentifierPrefix      = "SPDX-License-Identifier:"
)

var copyrightHeaderYearRe = regexp.MustCompile(`(?i)copyright(?:\s*\(c\)|\s*©)?[^0-9\n]*((?:19|20)\d{2})(?:\s*[-,]\s*((?:19|20)\d{2}))?`)

// CopyrightYearOutdatedRule flags stale copyright years in the leading file header.
// This iteration intentionally limits itself to header scanning and a fixed cutoff;
// git-backed recency validation can layer on top later.
type CopyrightYearOutdatedRule struct {
	LineBase
	BaseRule
	RecentYearCutoff int
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule. Detection scans file headers and manifests for license
// markers via regex; custom or non-English headers can produce false
// negatives. Classified per roadmap/17.
func (r *CopyrightYearOutdatedRule) Confidence() float64 { return 0.75 }

func (r *CopyrightYearOutdatedRule) check(ctx *v2.Context) {
	file := ctx.File
	for i, line := range file.Lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if !isHeaderCommentLine(trimmed) {
			break
		}

		match := copyrightHeaderYearRe.FindStringSubmatchIndex(line)
		if match == nil {
			continue
		}

		year := matchedCopyrightYear(line, match)
		if year >= r.RecentYearCutoff {
			return
		}

		ctx.Emit(r.Finding(
			file,
			i+1,
			match[2]+1,
			fmt.Sprintf("Copyright year %d looks outdated for files changed after %d.", year, r.RecentYearCutoff-1),
		))
		return
	}
}

func isHeaderCommentLine(trimmed string) bool {
	return strings.HasPrefix(trimmed, "//") ||
		strings.HasPrefix(trimmed, "/*") ||
		strings.HasPrefix(trimmed, "*") ||
		strings.HasPrefix(trimmed, "*/")
}

func matchedCopyrightYear(line string, match []int) int {
	if match[4] != -1 && match[5] != -1 {
		if year, err := strconv.Atoi(line[match[4]:match[5]]); err == nil {
			return year
		}
	}
	year, _ := strconv.Atoi(line[match[2]:match[3]])
	return year
}

// MissingSpdxIdentifierRule flags header comments that omit a SPDX license
// identifier. This first pass only validates that a leading comment includes
// the standard SPDX marker; config-driven license matching can layer on later.
type MissingSpdxIdentifierRule struct {
	LineBase
	BaseRule
	RequiredPrefix string
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule. Detection scans file headers and manifests for license
// markers via regex; custom or non-English headers can produce false
// negatives. Classified per roadmap/17.
func (r *MissingSpdxIdentifierRule) Confidence() float64 { return 0.75 }

func (r *MissingSpdxIdentifierRule) check(ctx *v2.Context) {
	file := ctx.File
	commentLines, startLine := leadingHeaderComment(file.Lines)
	if len(commentLines) == 0 {
		return
	}
	for _, line := range commentLines {
		if strings.Contains(line, r.RequiredPrefix) {
			return
		}
	}
	ctx.Emit(r.Finding(
		file,
		startLine,
		1,
		fmt.Sprintf("File header comment is missing `%s <id>`.", r.RequiredPrefix),
	))
}

func leadingHeaderComment(lines []string) ([]string, int) {
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		switch {
		case strings.HasPrefix(trimmed, "//"):
			var commentLines []string
			for j := i; j < len(lines); j++ {
				current := strings.TrimSpace(lines[j])
				if !strings.HasPrefix(current, "//") {
					break
				}
				commentLines = append(commentLines, lines[j])
			}
			return commentLines, i + 1
		case strings.HasPrefix(trimmed, "/*"):
			var commentLines []string
			for j := i; j < len(lines); j++ {
				commentLines = append(commentLines, lines[j])
				if strings.Contains(lines[j], "*/") {
					return commentLines, i + 1
				}
			}
			return commentLines, i + 1
		default:
			return nil, 0
		}
	}
	return nil, 0
}

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
func (r *DependencyLicenseUnknownRule) Confidence() float64 { return 0.75 }

func (r *DependencyLicenseUnknownRule) check(ctx *v2.Context) {
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

		ctx.Emit(gradleFinding(path, line, r.BaseRule,
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
func (r *LgplStaticLinkingInApkRule) Confidence() float64 { return 0.75 }

func (r *LgplStaticLinkingInApkRule) check(ctx *v2.Context) {
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

		ctx.Emit(gradleFinding(path, line, r.BaseRule,
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
func (r *DependencyLicenseIncompatibleRule) Confidence() float64 { return 0.75 }

func (r *DependencyLicenseIncompatibleRule) check(ctx *v2.Context) {
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

		ctx.Emit(gradleFinding(path, line, r.BaseRule,
			fmt.Sprintf("Dependency %s is %s, which is incompatible with the project's %s license.",
				coord, depLicense, r.ProjectLicense)))
	}
}
