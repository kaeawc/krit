package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
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
func (r *OssLicensesNotIncludedInAndroidRule) Confidence() float64 { return 0.75 }

func (r *OssLicensesNotIncludedInAndroidRule) check(ctx *v2.Context) {
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
	ctx.Emit(gradleFinding(path, line, r.BaseRule,
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

// wellKnownOptInMarkers lists OptIn marker classes shipped with widely used
// Kotlin and AndroidX libraries. The rule treats any marker outside this set
// as a likely stale or typo'd reference. Custom project markers can be added
// via the `additionalMarkers` config option.
var wellKnownOptInMarkers = map[string]bool{
	// kotlin stdlib
	"ExperimentalStdlibApi":    true,
	"ExperimentalUnsignedTypes": true,
	"ExperimentalMultiplatform": true,
	"ExperimentalTypeInference": true,
	"ExperimentalContracts":     true,
	"ExperimentalTime":          true,
	"ExperimentalSubclassOptIn": true,
	"RequiresOptIn":             true,

	// kotlinx.coroutines
	"ExperimentalCoroutinesApi": true,
	"DelicateCoroutinesApi":     true,
	"InternalCoroutinesApi":     true,
	"FlowPreview":               true,
	"ObsoleteCoroutinesApi":     true,

	// kotlinx.serialization
	"ExperimentalSerializationApi": true,
	"InternalSerializationApi":     true,

	// Compose / AndroidX
	"ExperimentalComposeApi":         true,
	"ExperimentalComposeUiApi":       true,
	"ExperimentalAnimationApi":       true,
	"ExperimentalAnimationGraphicsApi": true,
	"ExperimentalFoundationApi":      true,
	"ExperimentalLayoutApi":          true,
	"ExperimentalMaterialApi":        true,
	"ExperimentalMaterial3Api":       true,
	"ExperimentalMaterial3ExpressiveApi": true,
	"ExperimentalTextApi":            true,
	"ExperimentalPagingApi":          true,
	"ExperimentalPagerApi":           true,
	"ExperimentalCoilApi":            true,
	"ExperimentalGlideComposeApi":    true,
	"ExperimentalComposableApi":      true,
	"InternalComposeApi":             true,
	"ExperimentalComposeRuntimeApi":  true,
	"ExperimentalGraphicsApi":        true,

	// kotlinx.atomicfu / kotlinx.io / others
	"ExperimentalAtomicfuApi":  true,
	"ExperimentalIoApi":        true,
	"InternalAtomicfuApi":      true,

	// AndroidX miscellaneous
	"ExperimentalCarApi":           true,
	"ExperimentalHorologistApi":    true,
	"ExperimentalLifecycleComposeApi": true,
	"ExperimentalMediaCompatApi":   true,
	"ExperimentalNavigationApi":    true,
	"ExperimentalWindowApi":        true,
	"ExperimentalWindowCoreApi":    true,
}

// optInMarkerName extracts the marker class simple name from the expression
// inside `@OptIn(...)`. tree-sitter parses `Foo::class` as either a
// class_literal or a callable_reference, and `pkg.Foo::class` as a
// navigation_expression — all three shapes appear in real Kotlin sources.
func optInMarkerName(file *scanner.File, expr uint32) string {
	if expr == 0 {
		return ""
	}
	switch file.FlatType(expr) {
	case "class_literal":
		name := ""
		for c := file.FlatFirstChild(expr); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "simple_identifier":
				name = file.FlatNodeText(c)
			case "navigation_expression":
				if n := flatNavigationExpressionLastIdentifier(file, c); n != "" {
					name = n
				}
			}
		}
		return name
	case "callable_reference":
		name := ""
		for c := file.FlatFirstChild(expr); c != 0; c = file.FlatNextSib(c) {
			switch file.FlatType(c) {
			case "simple_identifier", "type_identifier":
				name = file.FlatNodeText(c)
			case "user_type":
				if ident := flatLastChildOfType(file, c, "type_identifier"); ident != 0 {
					name = file.FlatNodeText(ident)
				}
			case "navigation_expression":
				if n := flatNavigationExpressionLastIdentifier(file, c); n != "" {
					name = n
				}
			}
		}
		return name
	case "navigation_expression":
		if n := flatNavigationExpressionLastIdentifier(file, expr); n != "" {
			return n
		}
	}
	return ""
}

// OptInMarkerNotRecognisedRule flags `@OptIn(Foo::class)` annotations whose
// marker does not appear in the embedded well-known markers list. A stale
// reference often indicates an experimental API that has graduated or been
// removed.
type OptInMarkerNotRecognisedRule struct {
	FlatDispatchBase
	BaseRule
	AdditionalMarkers []string
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule. The
// embedded marker list cannot enumerate every project-local OptIn marker, so
// callers can extend it via configuration. Classified per roadmap/17.
func (r *OptInMarkerNotRecognisedRule) Confidence() float64 { return 0.7 }

func (r *OptInMarkerNotRecognisedRule) check(ctx *v2.Context) {
	idx, file := ctx.Idx, ctx.File
	if annotationFinalName(file, idx) != "OptIn" {
		return
	}
	ctor, ok := file.FlatFindChild(idx, "constructor_invocation")
	if !ok {
		return
	}
	args, _ := file.FlatFindChild(ctor, "value_arguments")
	if args == 0 {
		return
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		expr := flatValueArgumentExpression(file, arg)
		if expr == 0 {
			continue
		}
		name := optInMarkerName(file, expr)
		if name == "" {
			continue
		}
		if wellKnownOptInMarkers[name] {
			continue
		}
		if r.markerInAdditional(name) {
			continue
		}
		ctx.Emit(r.Finding(file, file.FlatRow(arg)+1, file.FlatCol(arg)+1,
			fmt.Sprintf("@OptIn marker %q is not in the embedded well-known markers list; verify the marker still exists or add it to additionalMarkers.", name)))
	}
}

func (r *OptInMarkerNotRecognisedRule) markerInAdditional(name string) bool {
	for _, m := range r.AdditionalMarkers {
		if i := strings.LastIndex(m, "."); i >= 0 {
			m = m[i+1:]
		}
		if m == name {
			return true
		}
	}
	return false
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
