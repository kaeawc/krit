package rules

import (
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// DependencyFromBintrayRule flags Gradle repositories hosted on retired Bintray endpoints.
type DependencyFromBintrayRule struct {
	GradleBase
	BaseRule
}

func (r *DependencyFromBintrayRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *DependencyFromBintrayRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isGradleRepositoryScript(path) {
		return
	}

	for _, repo := range findGradleRepositoryURLs(content) {
		if !isBintrayRepositoryURL(repo.rawURL) {
			continue
		}
		ctx.Emit(scanner.Finding{
			File:       path,
			Line:       repo.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Gradle repository %s points at Bintray, which is shut down. Replace it with a supported Maven repository.", repo.rawURL),
			Confidence: r.Confidence(),
		})
	}
}

// DependencyFromJcenterRule flags Gradle repositories that still use JCenter.
type DependencyFromJcenterRule struct {
	GradleBase
	BaseRule
}

func (r *DependencyFromJcenterRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *DependencyFromJcenterRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isGradleRepositoryScript(path) {
		return
	}

	for _, line := range findGradleRepositoryCallLines(content, "jcenter") {
		ctx.Emit(scanner.Finding{
			File:       path,
			Line:       line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    "Gradle repository uses jcenter(), which was sunset in 2021. Replace it with mavenCentral(), google(), or an explicit supported repository.",
			Confidence: r.Confidence(),
		})
	}
}

// DependencyFromHTTPRule flags plaintext Gradle repository URLs.
type DependencyFromHTTPRule struct {
	GradleBase
	BaseRule

	AllowLoopback bool
	AllowedHosts  []string
	AllowedUrls   []string
}

var gradleRepoDirectCallRe = regexp.MustCompile(`\b(?:maven|ivy)\s*\(`)

func (r *DependencyFromHTTPRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *DependencyFromHTTPRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isGradleRepositoryScript(path) {
		return
	}

	for _, repo := range findGradleRepositoryURLs(content) {
		if !isHTTPGradleRepositoryURL(repo.rawURL) {
			continue
		}
		if r.allowed(repo.rawURL) {
			continue
		}
		ctx.Emit(scanner.Finding{
			File:       path,
			Line:       repo.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Gradle repository %s uses plaintext HTTP. Use HTTPS or add this trusted internal host to the rule allow-list.", repo.rawURL),
			Confidence: r.Confidence(),
		})
	}
}

func (r *DependencyFromHTTPRule) allowed(raw string) bool {
	for _, prefix := range r.AllowedUrls {
		if prefix != "" && strings.HasPrefix(raw, prefix) {
			return true
		}
	}

	parsed, err := url.Parse(raw)
	if err != nil {
		return false
	}
	host := strings.ToLower(parsed.Hostname())
	if r.AllowLoopback && isLoopbackGradleRepositoryHost(host) {
		return true
	}
	for _, allowed := range r.AllowedHosts {
		if strings.EqualFold(host, strings.TrimSpace(allowed)) {
			return true
		}
	}
	return false
}

func isGradleRepositoryScript(path string) bool {
	switch filepath.Base(path) {
	case "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts", "init.gradle", "init.gradle.kts":
		return true
	default:
		return false
	}
}

func isLoopbackGradleRepositoryHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

type gradleRepositoryURL struct {
	rawURL string
	line   int
}

func findGradleRepositoryURLs(content string) []gradleRepositoryURL {
	var findings []gradleRepositoryURL
	var repositoryDepths []int
	var artifactRepositoryDepths []int
	braceDepth := 0

	for i, rawLine := range strings.Split(content, "\n") {
		lineNoComment := stripGradleLineComment(rawLine)
		codeOnly := gradleStripStringsAndComments(rawLine)
		line := i + 1

		opensRepositories := gradleLineOpensNamedBlock(codeOnly, "repositories")
		insideRepositories := len(repositoryDepths) > 0 || opensRepositories

		opensArtifactRepository := insideRepositories && gradleLineOpensNamedBlock(codeOnly, "maven", "ivy")
		insideArtifactRepository := len(artifactRepositoryDepths) > 0 || opensArtifactRepository
		directRepositoryCall := insideRepositories && gradleRepoDirectCallRe.MatchString(codeOnly)
		urlSetter := insideArtifactRepository && gradleLineMentionsURLSetter(codeOnly)

		if directRepositoryCall || urlSetter {
			for _, literal := range gradleStringLiterals(lineNoComment) {
				findings = append(findings, gradleRepositoryURL{rawURL: literal, line: line})
			}
		}

		openBraces, closeBraces := countGradleBraces(lineNoComment)
		for range openBraces {
			braceDepth++
			if opensRepositories {
				repositoryDepths = append(repositoryDepths, braceDepth)
				opensRepositories = false
			} else if opensArtifactRepository {
				artifactRepositoryDepths = append(artifactRepositoryDepths, braceDepth)
				opensArtifactRepository = false
			}
		}
		for range closeBraces {
			for len(artifactRepositoryDepths) > 0 && artifactRepositoryDepths[len(artifactRepositoryDepths)-1] >= braceDepth {
				artifactRepositoryDepths = artifactRepositoryDepths[:len(artifactRepositoryDepths)-1]
			}
			for len(repositoryDepths) > 0 && repositoryDepths[len(repositoryDepths)-1] >= braceDepth {
				repositoryDepths = repositoryDepths[:len(repositoryDepths)-1]
			}
			if braceDepth > 0 {
				braceDepth--
			}
		}
	}

	return findings
}

func findGradleRepositoryCallLines(content, callName string) []int {
	var lines []int
	var repositoryDepths []int
	braceDepth := 0

	for i, rawLine := range strings.Split(content, "\n") {
		lineNoComment := stripGradleLineComment(rawLine)
		codeOnly := gradleStripStringsAndComments(rawLine)
		opensRepositories := gradleLineOpensNamedBlock(codeOnly, "repositories")
		insideRepositories := len(repositoryDepths) > 0 || opensRepositories

		if insideRepositories && gradleLineHasDirectRepositoryCall(codeOnly, callName) {
			lines = append(lines, i+1)
		}

		openBraces, closeBraces := countGradleBraces(lineNoComment)
		for range openBraces {
			braceDepth++
			if opensRepositories {
				repositoryDepths = append(repositoryDepths, braceDepth)
				opensRepositories = false
			}
		}
		for range closeBraces {
			for len(repositoryDepths) > 0 && repositoryDepths[len(repositoryDepths)-1] >= braceDepth {
				repositoryDepths = repositoryDepths[:len(repositoryDepths)-1]
			}
			if braceDepth > 0 {
				braceDepth--
			}
		}
	}

	return lines
}

// gradleBlockOpenerRegexes / gradleDirectCallRegexes precompile the
// per-name regexes used by per-line Gradle scans. Each known name
// (repositories, maven, ivy, dependencyLocking, jcenter) is compiled
// once at init; helpers below look it up on every line. The previous
// inline regexp.MustCompile was a measurable hot path on multi-module
// projects.
//
// New names must be registered here; the helpers panic on lookup
// miss to fail loudly during tests rather than silently re-introducing
// the per-line compile.
var (
	gradleBlockOpenerRegexes = compileGradleNameRegexes(
		[]string{"repositories", "maven", "ivy", "dependencyLocking"},
		func(name string) string {
			return `\b` + regexp.QuoteMeta(name) + `\s*(?:\([^)]*\)\s*)?\{`
		},
	)
	gradleDirectCallFunDefRegexes = compileGradleNameRegexes(
		[]string{"jcenter"},
		func(name string) string {
			return `\bfun\s+` + regexp.QuoteMeta(name) + `\s*\(`
		},
	)
	gradleDirectCallInvocationRegexes = compileGradleNameRegexes(
		[]string{"jcenter"},
		func(name string) string {
			return `(^|[;{\s])` + regexp.QuoteMeta(name) + `\s*(?:\(\s*\)|\{)`
		},
	)
	gradleURLSetterRe = regexp.MustCompile(`\b(?:url|setUrl)\b`)
)

func compileGradleNameRegexes(names []string, pattern func(string) string) map[string]*regexp.Regexp {
	out := make(map[string]*regexp.Regexp, len(names))
	for _, name := range names {
		out[name] = regexp.MustCompile(pattern(name))
	}
	return out
}

func gradleNameRegex(registry map[string]*regexp.Regexp, name, kind string) *regexp.Regexp {
	re, ok := registry[name]
	if !ok {
		panic(fmt.Sprintf("krit internal: gradle %s regex not registered for %q — register it in supply_dependencies.go", kind, name))
	}
	return re
}

func gradleLineOpensNamedBlock(codeOnly string, names ...string) bool {
	for _, name := range names {
		if gradleNameRegex(gradleBlockOpenerRegexes, name, "block-opener").MatchString(codeOnly) {
			return true
		}
	}
	return false
}

func gradleLineHasDirectRepositoryCall(codeOnly, callName string) bool {
	if gradleNameRegex(gradleDirectCallFunDefRegexes, callName, "direct-call fun-def").MatchString(codeOnly) {
		return false
	}
	return gradleNameRegex(gradleDirectCallInvocationRegexes, callName, "direct-call invocation").MatchString(codeOnly)
}

func gradleLineMentionsURLSetter(codeOnly string) bool {
	return gradleURLSetterRe.MatchString(codeOnly)
}

func isHTTPGradleRepositoryURL(raw string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "http://")
}

func isBintrayRepositoryURL(raw string) bool {
	parsed, err := url.Parse(strings.TrimSpace(raw))
	if err == nil {
		host := strings.ToLower(parsed.Hostname())
		return host == "dl.bintray.com" || host == "jcenter.bintray.com" || host == "bintray.com" || strings.HasSuffix(host, ".bintray.com")
	}
	return strings.Contains(strings.ToLower(raw), "bintray.com")
}
