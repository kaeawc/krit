package rules

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"gopkg.in/yaml.v3"
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

// DependenciesInRootProjectRule flags application/runtime dependencies declared
// directly in the root Gradle project instead of in an owning module.
type DependenciesInRootProjectRule struct {
	GradleBase
	BaseRule

	AllowedConfigurations []string
}

func (r *DependenciesInRootProjectRule) Confidence() float64 { return api.ConfidenceHigh }

// Suggested-fix identifiers and canonical titles for the
// DependenciesInRootProject rule. The titles are shared between the rule's
// registry catalog (api.Rule.SuggestedFixes) and the per-finding emit so
// IDE quick-fix menus and the CLI list view show the same label.
const (
	DependenciesInRootProjectMoveSuggestionID     = "moveToOwningModule"
	dependenciesInRootProjectMoveSuggestionTitle  = "Move dependencies into an owning module"
	DependenciesInRootProjectAllowSuggestionID    = "addAllowedConfigurations"
	dependenciesInRootProjectAllowSuggestionTitle = "Add configurations to allowedConfigurations in the root Krit config"
)

func (r *DependenciesInRootProjectRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isRootGradleProjectScript(path) {
		return
	}
	allowed := rootDependencyAllowedConfigurations(r.AllowedConfigurations)
	for _, block := range findRootProjectDependencyBlocks(content, allowed) {
		finding := scanner.Finding{
			File:       path,
			Line:       block.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Root project dependencies block declares %s. Move project dependencies into an owning module or add legitimate root tooling configurations to allowedConfigurations.", strings.Join(block.configurations, ", ")),
			Confidence: r.Confidence(),
		}
		finding.SuggestedFixes = r.suggestedFixesForBlock(path, block.configurations)
		ctx.Emit(finding)
	}
}

func (r *DependenciesInRootProjectRule) suggestedFixesForBlock(gradlePath string, missing []string) []scanner.SuggestedFix {
	joined := strings.Join(missing, ", ")
	suggestions := []scanner.SuggestedFix{
		{
			ID:     DependenciesInRootProjectMoveSuggestionID,
			Title:  dependenciesInRootProjectMoveSuggestionTitle,
			Detail: fmt.Sprintf("Declare %s in the module that owns the code instead of the root project. Root build scripts should configure plugins and conventions; application and runtime dependencies belong to an owning module's build.gradle(.kts).", joined),
		},
	}

	allowSuggestion := scanner.SuggestedFix{
		ID:     DependenciesInRootProjectAllowSuggestionID,
		Title:  dependenciesInRootProjectAllowSuggestionTitle,
		Detail: fmt.Sprintf("Add %s to allowedConfigurations in krit.yml so the listed root-project configurations no longer trigger this rule. Use this when the root dependency block is intentional (e.g. classpath, detektPlugins, lintChecks).", joined),
	}
	if edit, ok := rootDependencyAllowedConfigurationsEdit(gradlePath, r.AllowedConfigurations, missing); ok {
		allowSuggestion.Edits = []scanner.SuggestedEdit{edit}
	}
	return append(suggestions, allowSuggestion)
}

func rootDependencyAllowedConfigurationsEdit(gradlePath string, existingAllowed, missing []string) (scanner.SuggestedEdit, bool) {
	targetPath, content, ok := rootKritConfigForFix(filepath.Dir(gradlePath))
	if !ok {
		return scanner.SuggestedEdit{}, false
	}
	replacement, ok := mergeRootDependencyAllowedConfigurations(content, existingAllowed, missing)
	if !ok {
		return scanner.SuggestedEdit{}, false
	}
	return scanner.SuggestedEdit{
		TargetFile:  targetPath,
		ByteMode:    true,
		StartByte:   0,
		EndByte:     len(content),
		Replacement: replacement,
	}, true
}

func rootKritConfigForFix(root string) (string, []byte, bool) {
	for _, name := range []string{"krit.yml", ".krit.yml"} {
		path := filepath.Join(root, name)
		content, err := os.ReadFile(path)
		if err == nil {
			return path, content, true
		}
		if err != nil && !os.IsNotExist(err) {
			return "", nil, false
		}
	}
	return filepath.Join(root, "krit.yml"), nil, true
}

func mergeRootDependencyAllowedConfigurations(content []byte, existingAllowed, missing []string) (string, bool) {
	header := leadingYAMLCommentHeader(content)
	root := make(map[string]any)
	if len(strings.TrimSpace(string(content))) > 0 {
		if err := yaml.Unmarshal(content, &root); err != nil {
			return "", false
		}
		if root == nil {
			root = make(map[string]any)
		}
	}

	supplyChain := yamlMap(root["supply-chain"])
	root["supply-chain"] = supplyChain
	rule := yamlMap(supplyChain["DependenciesInRootProject"])
	supplyChain["DependenciesInRootProject"] = rule

	allowed := append([]string(nil), yamlStringList(rule["allowedConfigurations"])...)
	allowed = appendMissingStrings(allowed, existingAllowed...)
	allowed = appendMissingStrings(allowed, missing...)
	rule["allowedConfigurations"] = allowed

	out, err := yaml.Marshal(root)
	if err != nil {
		return "", false
	}
	if len(out) == 0 || out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return header + string(out), true
}

func leadingYAMLCommentHeader(content []byte) string {
	text := string(content)
	if !strings.HasPrefix(text, "#") {
		return ""
	}
	offset := 0
	for offset < len(text) {
		next := strings.IndexByte(text[offset:], '\n')
		lineEnd := len(text)
		if next >= 0 {
			lineEnd = offset + next + 1
		}
		line := text[offset:lineEnd]
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return text[:offset]
		}
		offset = lineEnd
	}
	return text
}

func yamlMap(value any) map[string]any {
	if m, ok := value.(map[string]any); ok && m != nil {
		return m
	}
	return make(map[string]any)
}

func yamlStringList(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			items = append(items, s)
		}
	}
	return items
}

func appendMissingStrings(items []string, values ...string) []string {
	seen := make(map[string]bool, len(items)+len(values))
	for _, item := range items {
		seen[item] = true
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		items = append(items, value)
		seen[value] = true
	}
	return items
}

type rootProjectDependencyBlock struct {
	line           int
	configurations []string
}

func rootDependencyAllowedConfigurations(configs []string) map[string]bool {
	allowed := map[string]bool{
		"classpath":     true,
		"detektPlugins": true,
	}
	for _, cfg := range configs {
		cfg = strings.TrimSpace(cfg)
		if cfg != "" {
			allowed[cfg] = true
		}
	}
	return allowed
}

func isRootGradleProjectScript(path string) bool {
	base := filepath.Base(path)
	switch base {
	case "settings.gradle", "settings.gradle.kts":
		return true
	case "build.gradle", "build.gradle.kts":
		dir := filepath.Dir(path)
		for _, settings := range []string{"settings.gradle.kts", "settings.gradle"} {
			if _, err := os.Stat(filepath.Join(dir, settings)); err == nil {
				return true
			}
		}
	}
	return false
}

func findRootProjectDependencyBlocks(content string, allowed map[string]bool) []rootProjectDependencyBlock {
	var blocks []rootProjectDependencyBlock
	depth := 0
	inDependencies := false
	dependenciesDepth := 0
	blockLine := 0
	blockConfigs := make(map[string]bool)

	for i, rawLine := range strings.Split(content, "\n") {
		code := gradleStripStringsAndComments(rawLine)
		if !inDependencies && depth == 0 && gradleLineStartsBlockCall(code, "dependencies") {
			inDependencies = true
			dependenciesDepth = depth
			blockLine = i + 1
			blockConfigs = make(map[string]bool)
		}

		if inDependencies {
			for _, cfg := range gradleConfigurationCalls(code) {
				if cfg == "dependencies" || allowed[cfg] {
					continue
				}
				blockConfigs[cfg] = true
			}
		}

		openBraces, closeBraces := countGradleBraces(code)
		depth += openBraces - closeBraces
		if depth < 0 {
			depth = 0
		}
		if inDependencies && depth <= dependenciesDepth {
			configs := sortedGradleConfigurations(blockConfigs)
			if len(configs) > 0 {
				blocks = append(blocks, rootProjectDependencyBlock{
					line:           blockLine,
					configurations: configs,
				})
			}
			inDependencies = false
		}
	}
	return blocks
}

func gradleLineStartsBlockCall(line, name string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, name) {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, name))
	return strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, "(")
}

func gradleConfigurationCalls(line string) []string {
	var calls []string
	for _, segment := range strings.FieldsFunc(line, func(r rune) bool { return r == '{' || r == ';' }) {
		segment = strings.TrimSpace(segment)
		if segment == "" || strings.HasPrefix(segment, "}") {
			continue
		}
		name := leadingGradleIdentifier(segment)
		if name != "" {
			calls = append(calls, name)
		}
	}
	return calls
}

func leadingGradleIdentifier(line string) string {
	for i, r := range line {
		if i == 0 {
			if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
				continue
			}
			return ""
		}
		if r != '_' && r != '-' && r != '.' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return line[:i]
		}
	}
	return line
}

func sortedGradleConfigurations(configs map[string]bool) []string {
	out := make([]string, 0, len(configs))
	for cfg := range configs {
		out = append(out, cfg)
	}
	sort.Strings(out)
	return out
}

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
var gradleDepConfigLineRe = regexp.MustCompile(`\b(?:implementation|api|compile|testCompile|androidTestCompile|compileOnly|runtimeOnly|testImplementation|testRuntimeOnly|testCompileOnly|androidTestImplementation|androidTestCompileOnly|androidTestRuntimeOnly|debugImplementation|releaseImplementation|kapt|ksp|annotationProcessor)\b`)
var (
	gradleNamedArgGroupColonRe   = regexp.MustCompile(`\bgroup\s*:\s*["']([^"']+)["']`)
	gradleNamedArgNameColonRe    = regexp.MustCompile(`\bname\s*:\s*["']([^"']+)["']`)
	gradleNamedArgVersionColonRe = regexp.MustCompile(`\bversion\s*:\s*["']([^"']+)["']`)
)

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
