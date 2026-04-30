package rules

import (
	"fmt"
	"os"
	"path/filepath"
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
	"org.jetbrains.kotlin:kotlin-stdlib":   "Apache-2.0",
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

// noticeRequiredArtifacts is the embedded registry of artifacts that
// require attribution text in the project's NOTICE file.
var noticeRequiredArtifacts = map[string]struct{}{
	"com.example:attrib-required-lib": {},
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
}

// Confidence reports a tier-2 (medium) base confidence. Licensing rule.
// Detection scans a NOTICE file for artifact identifiers; custom phrasing
// can produce false negatives.
func (r *NoticeFileOutOfDateRule) Confidence() float64 { return 0.75 }

func (r *NoticeFileOutOfDateRule) check(ctx *v2.Context) {
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
		if _, required := noticeRequiredArtifacts[coord]; !required {
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

		ctx.Emit(gradleFinding(path, line, r.BaseRule,
			fmt.Sprintf("NOTICE file is missing required attribution for %s; add the attribution text to NOTICE.", coord)))
	}
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
