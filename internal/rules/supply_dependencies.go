package rules

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// DependencySnapshotInReleaseRule flags release dependencies pinned to SNAPSHOT versions.
type DependencySnapshotInReleaseRule struct {
	GradleBase
	BaseRule

	AllowedSnapshots []string
	SuppressUntil    string
}

func (r *DependencySnapshotInReleaseRule) Confidence() float64 { return api.ConfidenceHigher }

func (r *DependencySnapshotInReleaseRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isGradleRepositoryScript(path) || r.suppressedByDate() {
		return
	}

	for _, dep := range findGradleSnapshotDependencies(content) {
		if r.allowed(dep.group, dep.name) {
			continue
		}
		ctx.Emit(scanner.Finding{
			File:       path,
			Line:       dep.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Dependency %s:%s uses snapshot version %s. Use a released version for reproducible builds.", dep.group, dep.name, dep.version),
			Confidence: r.Confidence(),
		})
	}
}

func (r *DependencySnapshotInReleaseRule) suppressedByDate() bool {
	if strings.TrimSpace(r.SuppressUntil) == "" {
		return false
	}
	until, err := time.Parse("2006-01-02", strings.TrimSpace(r.SuppressUntil))
	if err != nil {
		return false
	}
	return time.Now().Before(until.Add(24 * time.Hour))
}

func (r *DependencySnapshotInReleaseRule) allowed(group, name string) bool {
	coord := group + ":" + name
	for _, pattern := range r.AllowedSnapshots {
		if gradleCoordinatePatternMatches(strings.TrimSpace(pattern), coord) {
			return true
		}
	}
	return false
}

// DependencyWithoutGroupRule flags legacy name:version dependency coordinates.
type DependencyWithoutGroupRule struct {
	GradleBase
	BaseRule
}

func (r *DependencyWithoutGroupRule) Confidence() float64 { return api.ConfidenceHigher }

func (r *DependencyWithoutGroupRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isGradleRepositoryScript(path) {
		return
	}

	for _, dep := range findGradleDependenciesWithoutGroup(content) {
		ctx.Emit(scanner.Finding{
			File:       path,
			Line:       dep.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Dependency coordinate %q omits the group. Use a full group:name:version coordinate.", dep.coordinate),
			Confidence: r.Confidence(),
		})
	}
}

var gradleDepConfigLineRe = regexp.MustCompile(`\b(?:implementation|api|compile|testCompile|androidTestCompile|compileOnly|runtimeOnly|testImplementation|testRuntimeOnly|testCompileOnly|androidTestImplementation|androidTestCompileOnly|androidTestRuntimeOnly|debugImplementation|releaseImplementation|kapt|ksp|annotationProcessor)\b`)
var (
	gradleNamedArgGroupColonRe   = regexp.MustCompile(`\bgroup\s*:\s*["']([^"']+)["']`)
	gradleNamedArgNameColonRe    = regexp.MustCompile(`\bname\s*:\s*["']([^"']+)["']`)
	gradleNamedArgVersionColonRe = regexp.MustCompile(`\bversion\s*:\s*["']([^"']+)["']`)
)

type gradleSnapshotDependency struct {
	group   string
	name    string
	version string
	line    int
}

type gradleDependencyWithoutGroup struct {
	coordinate string
	line       int
}

func findGradleSnapshotDependencies(content string) []gradleSnapshotDependency {
	var out []gradleSnapshotDependency
	for _, ref := range gradleDependencyBlockLines(content) {
		codeOnly := gradleStripStringsAndComments(ref.text)
		if !gradleDepConfigLineRe.MatchString(codeOnly) {
			continue
		}
		for _, literal := range gradleStringLiterals(stripGradleLineComment(ref.text)) {
			group, name, version, ok := parseGradleCoordinate(literal)
			if !ok || !strings.HasSuffix(strings.ToUpper(version), "-SNAPSHOT") {
				continue
			}
			out = append(out, gradleSnapshotDependency{group: group, name: name, version: version, line: ref.num})
		}
		if group, name, version, ok := parseKTSNamedDependency(ref.text); ok && strings.HasSuffix(strings.ToUpper(version), "-SNAPSHOT") {
			out = append(out, gradleSnapshotDependency{group: group, name: name, version: version, line: ref.num})
		} else if group, name, version, ok := parseGroovyNamedDependency(ref.text); ok && strings.HasSuffix(strings.ToUpper(version), "-SNAPSHOT") {
			out = append(out, gradleSnapshotDependency{group: group, name: name, version: version, line: ref.num})
		}
	}
	return out
}

func findGradleDependenciesWithoutGroup(content string) []gradleDependencyWithoutGroup {
	var out []gradleDependencyWithoutGroup
	for _, ref := range gradleDependencyBlockLines(content) {
		codeOnly := gradleStripStringsAndComments(ref.text)
		if !gradleDepConfigLineRe.MatchString(codeOnly) {
			continue
		}
		for _, literal := range gradleStringLiterals(stripGradleLineComment(ref.text)) {
			if isGradleCoordinateWithoutGroup(literal) {
				out = append(out, gradleDependencyWithoutGroup{coordinate: literal, line: ref.num})
			}
		}
	}
	return out
}

func parseGradleCoordinate(raw string) (group, name, version string, ok bool) {
	parts := strings.Split(raw, ":")
	if len(parts) < 3 {
		return "", "", "", false
	}
	// Gradle coordinates can be 3- or 4-part:
	//   group:name:version
	//   group:name:version:classifier   (or  group:name:version@ext after split)
	// The previous version picked parts[len-1] and would return the
	// classifier ("sources") as the version on a 4-part coord, then
	// supply-chain rules would compare the wrong string against advisory
	// data.
	group = strings.TrimSpace(parts[0])
	name = strings.TrimSpace(parts[1])
	version = strings.TrimSpace(parts[2])
	return group, name, version, group != "" && name != "" && version != ""
}

func isGradleCoordinateWithoutGroup(raw string) bool {
	parts := strings.Split(raw, ":")
	if len(parts) != 2 {
		return false
	}
	return strings.TrimSpace(parts[0]) != "" && strings.TrimSpace(parts[1]) != ""
}

func parseKTSNamedDependency(line string) (group, name, version string, ok bool) {
	g := ktsNamedArgGroupRe.FindStringSubmatch(line)
	n := ktsNamedArgNameRe.FindStringSubmatch(line)
	v := ktsNamedArgVersionRe.FindStringSubmatch(line)
	if len(g) != 2 || len(n) != 2 || len(v) != 2 {
		return "", "", "", false
	}
	return g[1], n[1], v[1], true
}

func parseGroovyNamedDependency(line string) (group, name, version string, ok bool) {
	g := gradleNamedArgGroupColonRe.FindStringSubmatch(line)
	n := gradleNamedArgNameColonRe.FindStringSubmatch(line)
	v := gradleNamedArgVersionColonRe.FindStringSubmatch(line)
	if len(g) != 2 || len(n) != 2 || len(v) != 2 {
		return "", "", "", false
	}
	return g[1], n[1], v[1], true
}

func gradleCoordinatePatternMatches(pattern, coord string) bool {
	if pattern == "" {
		return false
	}
	if pattern == "*" || pattern == coord {
		return true
	}
	if !strings.Contains(pattern, "*") {
		return false
	}
	parts := strings.Split(pattern, "*")
	pos := 0
	for i, part := range parts {
		if part == "" {
			continue
		}
		idx := strings.Index(coord[pos:], part)
		if idx < 0 || (i == 0 && idx != 0) {
			return false
		}
		pos += idx + len(part)
	}
	return strings.HasSuffix(pattern, "*") || pos == len(coord)
}

func gradleStringLiterals(line string) []string {
	var literals []string
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		if r != '"' && r != '\'' {
			i += size
			continue
		}
		quote := r
		start := i + size
		i += size
		escaped := false
		var b strings.Builder
		for i < len(line) {
			r, size = utf8.DecodeRuneInString(line[i:])
			if escaped {
				b.WriteRune(r)
				escaped = false
				i += size
				continue
			}
			if r == '\\' {
				escaped = true
				i += size
				continue
			}
			if r == quote {
				literals = append(literals, b.String())
				i += size
				break
			}
			b.WriteString(line[i : i+size])
			i += size
		}
		if i <= start {
			break
		}
	}
	return literals
}

func countGradleBraces(line string) (openBraces, closeBraces int) {
	inQuote := rune(0)
	escaped := false
	for i := 0; i < len(line); {
		r, size := utf8.DecodeRuneInString(line[i:])
		if inQuote != 0 {
			if escaped {
				escaped = false
			} else if r == '\\' {
				escaped = true
			} else if r == inQuote {
				inQuote = 0
			}
			i += size
			continue
		}
		switch r {
		case '"', '\'':
			inQuote = r
		case '{':
			openBraces++
		case '}':
			closeBraces++
		}
		i += size
	}
	return openBraces, closeBraces
}
