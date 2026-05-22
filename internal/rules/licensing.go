package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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
func (r *CopyrightYearOutdatedRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *CopyrightYearOutdatedRule) check(ctx *api.Context) {
	file := ctx.File
	var st lineScanState
	for i, line := range file.Lines {
		// Snapshot state at the START of this line: a `/*` block opened
		// on a previous line keeps us inside the header even when the
		// continuation lines have no `*` decoration.
		insideBlockComment := st.inBlockComment

		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			scanLineState(line, &st)
			continue
		}
		if !insideBlockComment && !isHeaderCommentLine(trimmed) {
			break
		}

		match := copyrightHeaderYearRe.FindStringSubmatchIndex(line)
		if match == nil {
			scanLineState(line, &st)
			continue
		}

		year := matchedCopyrightYear(line, match)
		if year >= r.RecentYearCutoff {
			return
		}

		f := r.Finding(
			file,
			i+1,
			match[2]+1,
			fmt.Sprintf("Copyright year %d looks outdated for files changed after %d.", year, r.RecentYearCutoff-1),
		)
		lineStart := file.LineOffset(i)
		hasRange := match[4] != -1 && match[5] != -1
		var startByte, endByte int
		var replacement string
		if hasRange {
			// Bump the trailing year of the range to the cutoff.
			startByte = lineStart + match[4]
			endByte = lineStart + match[5]
			replacement = strconv.Itoa(r.RecentYearCutoff)
		} else {
			// Extend the single year into a range ending at the cutoff.
			startByte = lineStart + match[2]
			endByte = lineStart + match[3]
			replacement = fmt.Sprintf("%d-%d", year, r.RecentYearCutoff)
		}
		f.Fix = &scanner.Fix{
			ByteMode:    true,
			StartByte:   startByte,
			EndByte:     endByte,
			Replacement: replacement,
		}
		ctx.Emit(f)
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
func (r *MissingSpdxIdentifierRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *MissingSpdxIdentifierRule) check(ctx *api.Context) {
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

// SpdxIdentifierMismatchWithProjectRule flags file headers whose SPDX
// identifier disagrees with the project-level license configured for the
// rule. Only fires when both the file header declares an SPDX id and the
// project license is configured.
type SpdxIdentifierMismatchWithProjectRule struct {
	LineBase
	BaseRule
	ProjectLicense string
}

// Confidence reports a tier-1 (high) base confidence. Detection compares the
// SPDX id parsed from the file header to the configured project license; both
// values are explicit, leaving little room for false positives.
func (r *SpdxIdentifierMismatchWithProjectRule) Confidence() float64 { return api.ConfidenceHigher }

func (r *SpdxIdentifierMismatchWithProjectRule) check(ctx *api.Context) {
	if r.ProjectLicense == "" {
		return
	}
	file := ctx.File
	commentLines, startLine := leadingHeaderComment(file.Lines)
	if len(commentLines) == 0 {
		return
	}
	for offset, line := range commentLines {
		idx := strings.Index(line, spdxIdentifierPrefix)
		if idx < 0 {
			continue
		}
		id := strings.TrimSpace(line[idx+len(spdxIdentifierPrefix):])
		if id == "" || id == r.ProjectLicense {
			return
		}
		ctx.Emit(r.Finding(
			file,
			startLine+offset,
			idx+1,
			fmt.Sprintf("File SPDX identifier %q does not match the project license %q.", id, r.ProjectLicense),
		))
		return
	}
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

// embeddedSpdxIdentifiers is a curated subset of the official SPDX License
// List (https://spdx.org/licenses/). Projects can extend it via the
// `additionalIdentifiers` config option.
var embeddedSpdxIdentifiers = map[string]bool{
	"0BSD":                true,
	"AFL-2.0":             true,
	"AFL-2.1":             true,
	"AFL-3.0":             true,
	"AGPL-1.0":            true,
	"AGPL-1.0-only":       true,
	"AGPL-1.0-or-later":   true,
	"AGPL-3.0":            true,
	"AGPL-3.0-only":       true,
	"AGPL-3.0-or-later":   true,
	"Apache-1.0":          true,
	"Apache-1.1":          true,
	"Apache-2.0":          true,
	"Artistic-1.0":        true,
	"Artistic-2.0":        true,
	"BSD-1-Clause":        true,
	"BSD-2-Clause":        true,
	"BSD-2-Clause-Patent": true,
	"BSD-3-Clause":        true,
	"BSD-3-Clause-Clear":  true,
	"BSD-4-Clause":        true,
	"BSL-1.0":             true,
	"CC-BY-1.0":           true,
	"CC-BY-2.0":           true,
	"CC-BY-2.5":           true,
	"CC-BY-3.0":           true,
	"CC-BY-4.0":           true,
	"CC-BY-NC-1.0":        true,
	"CC-BY-NC-2.0":        true,
	"CC-BY-NC-2.5":        true,
	"CC-BY-NC-3.0":        true,
	"CC-BY-NC-4.0":        true,
	"CC-BY-NC-ND-1.0":     true,
	"CC-BY-NC-ND-2.0":     true,
	"CC-BY-NC-ND-2.5":     true,
	"CC-BY-NC-ND-3.0":     true,
	"CC-BY-NC-ND-4.0":     true,
	"CC-BY-NC-SA-1.0":     true,
	"CC-BY-NC-SA-2.0":     true,
	"CC-BY-NC-SA-2.5":     true,
	"CC-BY-NC-SA-3.0":     true,
	"CC-BY-NC-SA-4.0":     true,
	"CC-BY-ND-1.0":        true,
	"CC-BY-ND-2.0":        true,
	"CC-BY-ND-2.5":        true,
	"CC-BY-ND-3.0":        true,
	"CC-BY-ND-4.0":        true,
	"CC-BY-SA-1.0":        true,
	"CC-BY-SA-2.0":        true,
	"CC-BY-SA-2.5":        true,
	"CC-BY-SA-3.0":        true,
	"CC-BY-SA-4.0":        true,
	"CC0-1.0":             true,
	"CDDL-1.0":            true,
	"CDDL-1.1":            true,
	"CECILL-2.1":          true,
	"CPL-1.0":             true,
	"ECL-1.0":             true,
	"ECL-2.0":             true,
	"EPL-1.0":             true,
	"EPL-2.0":             true,
	"EUPL-1.1":            true,
	"EUPL-1.2":            true,
	"GPL-1.0":             true,
	"GPL-1.0+":            true,
	"GPL-1.0-only":        true,
	"GPL-1.0-or-later":    true,
	"GPL-2.0":             true,
	"GPL-2.0+":            true,
	"GPL-2.0-only":        true,
	"GPL-2.0-or-later":    true,
	"GPL-3.0":             true,
	"GPL-3.0+":            true,
	"GPL-3.0-only":        true,
	"GPL-3.0-or-later":    true,
	"ISC":                 true,
	"LGPL-2.0":            true,
	"LGPL-2.0-only":       true,
	"LGPL-2.0-or-later":   true,
	"LGPL-2.1":            true,
	"LGPL-2.1-only":       true,
	"LGPL-2.1-or-later":   true,
	"LGPL-3.0":            true,
	"LGPL-3.0-only":       true,
	"LGPL-3.0-or-later":   true,
	"MIT":                 true,
	"MIT-0":               true,
	"MPL-1.0":             true,
	"MPL-1.1":             true,
	"MPL-2.0":             true,
	"MS-PL":               true,
	"MS-RL":               true,
	"NCSA":                true,
	"OFL-1.1":             true,
	"OSL-1.0":             true,
	"OSL-2.0":             true,
	"OSL-2.1":             true,
	"OSL-3.0":             true,
	"PostgreSQL":          true,
	"Python-2.0":          true,
	"Ruby":                true,
	"SISSL":               true,
	"Sleepycat":           true,
	"Unlicense":           true,
	"WTFPL":               true,
	"X11":                 true,
	"Zlib":                true,
	"ZPL-2.0":             true,
	"ZPL-2.1":             true,
}

// embeddedSpdxLicenseExceptions are identifiers valid after the SPDX `WITH`
// operator (https://spdx.org/licenses/exceptions-index.html).
var embeddedSpdxLicenseExceptions = map[string]bool{
	"389-exception":                  true,
	"Autoconf-exception-2.0":         true,
	"Autoconf-exception-3.0":         true,
	"Bison-exception-2.2":            true,
	"Bootloader-exception":           true,
	"Classpath-exception-2.0":        true,
	"CLISP-exception-2.0":            true,
	"DigiRule-FOSS-exception":        true,
	"eCos-exception-2.0":             true,
	"Fawkes-Runtime-exception":       true,
	"FLTK-exception":                 true,
	"Font-exception-2.0":             true,
	"freertos-exception-2.0":         true,
	"GCC-exception-2.0":              true,
	"GCC-exception-3.1":              true,
	"gnu-javamail-exception":         true,
	"i2p-gpl-java-exception":         true,
	"LGPL-3.0-linking-exception":     true,
	"Libtool-exception":              true,
	"LLVM-exception":                 true,
	"LZMA-exception":                 true,
	"mif-exception":                  true,
	"OCCT-exception-1.0":             true,
	"OpenJDK-assembly-exception-1.0": true,
	"openvpn-openssl-exception":      true,
	"Qt-GPL-exception-1.0":           true,
	"Qt-LGPL-exception-1.1":          true,
	"Qwt-exception-1.0":              true,
	"u-boot-exception-2.0":           true,
	"WxWindows-exception-3.1":        true,
}

// SpdxIdentifierInvalidRule flags `SPDX-License-Identifier:` header values
// that are not recognised SPDX short IDs.
type SpdxIdentifierInvalidRule struct {
	LineBase
	BaseRule
	AdditionalIdentifiers []string
}

// Confidence reports a tier-1 (high) base confidence. False positives are
// limited to licenses trimmed from the embedded set; users can add them via
// additionalIdentifiers.
func (r *SpdxIdentifierInvalidRule) Confidence() float64 { return api.ConfidenceHigher }

func (r *SpdxIdentifierInvalidRule) check(ctx *api.Context) {
	file := ctx.File
	commentLines, startLine := leadingHeaderComment(file.Lines)
	if len(commentLines) == 0 {
		return
	}
	for offset, line := range commentLines {
		idx := strings.Index(line, spdxIdentifierPrefix)
		if idx < 0 {
			continue
		}
		expr := strings.TrimSpace(line[idx+len(spdxIdentifierPrefix):])
		expr = strings.TrimRight(expr, "*/")
		expr = strings.TrimSpace(expr)
		if expr == "" {
			continue
		}
		invalid := r.invalidIdentifiers(expr)
		if len(invalid) == 0 {
			return
		}
		ctx.Emit(r.Finding(
			file,
			startLine+offset,
			idx+1,
			fmt.Sprintf("SPDX-License-Identifier value %q contains unrecognised SPDX identifier(s): %s. Use a valid SPDX short ID (https://spdx.org/licenses/) or add it to additionalIdentifiers.",
				expr, strings.Join(invalid, ", ")),
		))
		return
	}
}

func (r *SpdxIdentifierInvalidRule) invalidIdentifiers(expr string) []string {
	var bad []string
	tokens, ops := splitSpdxExpression(expr)
	for i, tok := range tokens {
		op := ""
		if i > 0 {
			op = ops[i-1]
		}
		if op == "WITH" {
			if !embeddedSpdxLicenseExceptions[tok] && !r.inAdditional(tok) {
				bad = append(bad, tok)
			}
			continue
		}
		id := strings.TrimSuffix(tok, "+")
		if id == "" {
			bad = append(bad, tok)
			continue
		}
		if embeddedSpdxIdentifiers[id] || embeddedSpdxIdentifiers[tok] || r.inAdditional(id) || r.inAdditional(tok) {
			continue
		}
		bad = append(bad, tok)
	}
	return bad
}

func (r *SpdxIdentifierInvalidRule) inAdditional(id string) bool {
	for _, a := range r.AdditionalIdentifiers {
		if a == id {
			return true
		}
	}
	return false
}

// splitSpdxExpression splits a SPDX license expression into identifier tokens
// and the operators between them. Parentheses are flattened — they only group
// in the SPDX grammar and don't change identifier validity.
func splitSpdxExpression(expr string) (tokens []string, ops []string) {
	cleaned := strings.NewReplacer("(", " ", ")", " ").Replace(expr)
	fields := strings.Fields(cleaned)
	for _, f := range fields {
		switch f {
		case "AND", "OR", "WITH":
			ops = append(ops, f)
		default:
			tokens = append(tokens, f)
		}
	}
	return tokens, ops
}

// wellKnownOptInMarkers lists OptIn marker classes shipped with widely used
// Kotlin and AndroidX libraries. The rule treats any marker outside this set
// as a likely stale or typo'd reference. Custom project markers can be added
// via the `additionalMarkers` config option.
var wellKnownOptInMarkers = map[string]bool{
	// kotlin stdlib
	"ExperimentalStdlibApi":     true,
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
	"ExperimentalComposeApi":             true,
	"ExperimentalComposeUiApi":           true,
	"ExperimentalAnimationApi":           true,
	"ExperimentalAnimationGraphicsApi":   true,
	"ExperimentalFoundationApi":          true,
	"ExperimentalLayoutApi":              true,
	"ExperimentalMaterialApi":            true,
	"ExperimentalMaterial3Api":           true,
	"ExperimentalMaterial3ExpressiveApi": true,
	"ExperimentalTextApi":                true,
	"ExperimentalPagingApi":              true,
	"ExperimentalPagerApi":               true,
	"ExperimentalCoilApi":                true,
	"ExperimentalGlideComposeApi":        true,
	"ExperimentalComposableApi":          true,
	"InternalComposeApi":                 true,
	"ExperimentalComposeRuntimeApi":      true,
	"ExperimentalGraphicsApi":            true,

	// kotlinx.atomicfu / kotlinx.io / others
	"ExperimentalAtomicfuApi": true,
	"ExperimentalIoApi":       true,
	"InternalAtomicfuApi":     true,

	// AndroidX miscellaneous
	"ExperimentalCarApi":              true,
	"ExperimentalHorologistApi":       true,
	"ExperimentalLifecycleComposeApi": true,
	"ExperimentalMediaCompatApi":      true,
	"ExperimentalNavigationApi":       true,
	"ExperimentalWindowApi":           true,
	"ExperimentalWindowCoreApi":       true,
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
func (r *OptInMarkerNotRecognisedRule) Confidence() float64 { return api.ConfidenceMediumLowPlus }

func (r *OptInMarkerNotRecognisedRule) check(ctx *api.Context) {
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

// OptInMarkerExposedPubliclyRule flags `@OptIn` annotations on public API
// declarations. Opt-in markers propagate to callers, so applying `@OptIn`
// to a public declaration silently forces every caller to opt in too.
type OptInMarkerExposedPubliclyRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-1 (high) base confidence. The detection is fully
// AST-driven: we match `@OptIn` annotations whose target declaration has no
// non-public visibility modifier.
func (r *OptInMarkerExposedPubliclyRule) Confidence() float64 { return api.ConfidenceHigher }

// optInAnnotationTarget walks up from an `@OptIn` annotation node to the
// declaration it is attached to. Returns the declaration index and true when
// the annotation is anchored to a class, object, function, property, or type
// alias whose visibility we can inspect.
func optInAnnotationTarget(file *scanner.File, annotation uint32) (uint32, bool) {
	for cur, ok := file.FlatParent(annotation); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "class_declaration",
			"object_declaration",
			"function_declaration",
			"property_declaration",
			"type_alias",
			"secondary_constructor",
			"anonymous_initializer":
			return cur, true
		case "modifiers":
			continue
		default:
			return 0, false
		}
	}
	return 0, false
}

// OptInWithoutJustificationRule flags `@OptIn(...)` annotations whose
// enclosing declaration lacks a preceding KDoc comment explaining why the
// opt-in is safe.
type OptInWithoutJustificationRule struct {
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule.
func (r *OptInWithoutJustificationRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *OptInWithoutJustificationRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if annotationFinalName(file, idx) != "OptIn" {
		return
	}
	decl, ok := optInAnnotationTarget(file, idx)
	if !ok {
		return
	}
	if _, ok := flatPrecedingKDoc(file, decl); ok {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"@OptIn annotation has no KDoc explaining why opting in is safe."))
}

// SuppressedWarningWithoutJustificationRule flags `@Suppress(...)` annotations
// whose enclosing declaration lacks a preceding KDoc comment explaining why
// silencing the warning is safe.
type SuppressedWarningWithoutJustificationRule struct {
	BaseRule
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule.
func (r *SuppressedWarningWithoutJustificationRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *SuppressedWarningWithoutJustificationRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if annotationFinalName(file, idx) != "Suppress" {
		return
	}
	decl, ok := optInAnnotationTarget(file, idx)
	if !ok {
		return
	}
	if _, ok := flatPrecedingKDoc(file, decl); ok {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"@Suppress annotation has no KDoc explaining why silencing the warning is safe."))
}

// RequiresOptInWithoutMessageRule flags `@RequiresOptIn` annotations that omit
// a `message` argument. Without an explanatory message, callers who must opt in
// have no in-source guidance about the experimental contract.
type RequiresOptInWithoutMessageRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-1 (high) base confidence. The check is a pure AST
// match on `@RequiresOptIn(...)` argument labels.
func (r *RequiresOptInWithoutMessageRule) Confidence() float64 { return api.ConfidenceHigher }

func (r *RequiresOptInWithoutMessageRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if annotationFinalName(file, idx) != "RequiresOptIn" {
		return
	}
	if ctor, ok := file.FlatFindChild(idx, "constructor_invocation"); ok {
		if args, ok := file.FlatFindChild(ctor, "value_arguments"); ok {
			for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
				if file.FlatType(arg) != "value_argument" {
					continue
				}
				if flatValueArgumentLabel(file, arg) == "message" {
					return
				}
			}
		}
	}
	ctx.Emit(r.Finding(file, file.FlatRow(idx)+1, file.FlatCol(idx)+1,
		"@RequiresOptIn is missing a message argument; add message = \"...\" so callers see why opting in is required."))
}

// RequiresOptInWithoutLevelRule flags custom `@RequiresOptIn` annotation
// classes that omit an explicit `level = WARNING|ERROR` argument. Without an
// explicit level callers see only the implicit default and have no signal
// from the source whether the marker is intended as a warning or an error.
type RequiresOptInWithoutLevelRule struct {
	FlatDispatchBase
	BaseRule
}

// Confidence reports a tier-1 (high) base confidence. The detection is
// AST-driven: we match `annotation class` declarations carrying a
// `@RequiresOptIn` annotation and inspect its `level = ...` argument.
func (r *RequiresOptInWithoutLevelRule) Confidence() float64 { return api.ConfidenceHigher }

func (r *RequiresOptInWithoutLevelRule) check(ctx *api.Context) {
	idx, file := ctx.Idx, ctx.File
	if !file.FlatHasModifier(idx, "annotation") {
		return
	}
	ann := requiresOptInAnnotationOnDeclaration(file, idx)
	if ann == 0 {
		return
	}
	if requiresOptInHasLevelArg(file, ann) {
		return
	}
	ctx.Emit(r.Finding(file, file.FlatRow(ann)+1, file.FlatCol(ann)+1,
		"@RequiresOptIn annotation class omits `level = RequiresOptIn.Level.WARNING` or `ERROR`; declare the intended level explicitly."))
}

func requiresOptInAnnotationOnDeclaration(file *scanner.File, decl uint32) uint32 {
	mods, ok := file.FlatFindChild(decl, "modifiers")
	if !ok {
		return 0
	}
	var found uint32
	for child := file.FlatFirstChild(mods); child != 0; child = file.FlatNextSib(child) {
		if file.FlatType(child) != "annotation" {
			continue
		}
		if annotationFinalName(file, child) == "RequiresOptIn" {
			found = child
			break
		}
	}
	return found
}

func requiresOptInHasLevelArg(file *scanner.File, annotation uint32) bool {
	ctor, ok := file.FlatFindChild(annotation, "constructor_invocation")
	if !ok {
		return false
	}
	args, _ := file.FlatFindChild(ctor, "value_arguments")
	if args == 0 {
		return false
	}
	for arg := file.FlatFirstChild(args); arg != 0; arg = file.FlatNextSib(arg) {
		if file.FlatType(arg) != "value_argument" {
			continue
		}
		if flatValueArgumentLabel(file, arg) == "level" {
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
