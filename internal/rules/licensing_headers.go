package rules

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
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
