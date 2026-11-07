package rules

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kaeawc/krit/internal/librarymodel"
	api "github.com/kaeawc/krit/internal/rules/api"
)

// This file holds the helpers shared by the Kotlin toolchain migration rules
// for Gradle build scripts (KotlinOptionsDsl, JvmDefaultFlag,
// ObsoleteKotlinVersionFlags, KotlinAndroidPluginWithBuiltInKotlin,
// ComposeCompilerRemovedFlags and KaptUsage): a comment- and string-aware view
// of a script, Kotlin/AGP version lookup, and gradle.properties access.

// gradleSource is a position-preserving view of a build script. Both views have
// the same byte length as the script, so an offset in one is an offset in the
// original text.
type gradleSource struct {
	content string
	// text is the script with comments blanked and string literals intact.
	text string
	// code is text with the bodies of string literals blanked too, so a
	// keyword match cannot land inside a literal.
	code       string
	lineStarts []int
}

func newGradleSource(content string) *gradleSource {
	text := []byte(content)
	code := []byte(content)
	blank := func(b []byte, from, to int) {
		for i := from; i < to && i < len(b); i++ {
			if b[i] != '\n' && b[i] != '\r' {
				b[i] = ' '
			}
		}
	}
	n := len(content)
	for i := 0; i < n; {
		c := content[i]
		switch {
		case c == '/' && i+1 < n && content[i+1] == '/':
			j := i
			for j < n && content[j] != '\n' {
				j++
			}
			blank(text, i, j)
			blank(code, i, j)
			i = j
		case c == '/' && i+1 < n && content[i+1] == '*':
			end := n
			if j := strings.Index(content[i+2:], "*/"); j >= 0 {
				end = i + 2 + j + 2
			}
			blank(text, i, end)
			blank(code, i, end)
			i = end
		case c == '"' || c == '\'':
			i = scanGradleString(content, i, code, blank)
		default:
			i++
		}
	}
	starts := []int{0}
	for i := 0; i < n; i++ {
		if content[i] == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &gradleSource{content: content, text: string(text), code: string(code), lineStarts: starts}
}

// scanGradleString blanks the body of the string literal starting at i in code
// and returns the index after the literal. Single-line strings stop at a
// newline so an unbalanced quote cannot swallow the rest of the script.
func scanGradleString(content string, i int, code []byte, blank func([]byte, int, int)) int {
	n := len(content)
	quote := content[i]
	if i+2 < n && content[i+1] == quote && content[i+2] == quote {
		start := i + 3
		bodyEnd, end := n, n
		if j := strings.Index(content[start:], strings.Repeat(string(quote), 3)); j >= 0 {
			bodyEnd = start + j
			end = bodyEnd + 3
		}
		blank(code, start, bodyEnd)
		return end
	}
	start := i + 1
	i++
	for i < n && content[i] != quote && content[i] != '\n' {
		switch {
		case content[i] == '\\' && i+1 < n:
			i += 2
		case quote == '"' && content[i] == '$' && i+1 < n && content[i+1] == '{':
			i = skipGradleTemplate(content, i+2)
		default:
			i++
		}
	}
	end := i
	if end > n {
		end = n
	}
	blank(code, start, end)
	if end < n && content[end] == quote {
		return end + 1
	}
	return end
}

// skipGradleTemplate returns the index after the `}` that closes a `${` whose
// body starts at i.
func skipGradleTemplate(content string, i int) int {
	depth := 1
	for i < len(content) && depth > 0 {
		switch content[i] {
		case '{':
			depth++
		case '}':
			depth--
		case '\n':
			return i
		}
		i++
	}
	return i
}

// lineCol returns the 1-based line and byte column of offset off.
func (s *gradleSource) lineCol(off int) (line, col int) {
	idx := sort.Search(len(s.lineStarts), func(i int) bool { return s.lineStarts[i] > off }) - 1
	if idx < 0 {
		idx = 0
	}
	return idx + 1, off - s.lineStarts[idx] + 1
}

// lineBounds returns the byte range of the line holding off, without its newline.
func (s *gradleSource) lineBounds(off int) (start, end int) {
	idx := sort.Search(len(s.lineStarts), func(i int) bool { return s.lineStarts[i] > off }) - 1
	if idx < 0 {
		idx = 0
	}
	start = s.lineStarts[idx]
	end = len(s.content)
	if idx+1 < len(s.lineStarts) {
		end = s.lineStarts[idx+1] - 1
	}
	if end > start && s.content[end-1] == '\r' {
		end--
	}
	return start, end
}

// lineBoundsWithNewline is lineBounds extended over the line terminator.
func (s *gradleSource) lineBoundsWithNewline(off int) (start, end int) {
	start, _ = s.lineBounds(off)
	idx := sort.Search(len(s.lineStarts), func(i int) bool { return s.lineStarts[i] > off }) - 1
	end = len(s.content)
	if idx+1 < len(s.lineStarts) {
		end = s.lineStarts[idx+1]
	}
	return start, end
}

// matchingBrace returns the index of the `}` closing the `{` at open in code, or -1.
func (s *gradleSource) matchingBrace(open int) int {
	depth := 0
	for i := open; i < len(s.code); i++ {
		switch s.code[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return i
			}
		}
	}
	return -1
}

// enclosingHeader returns the text before the innermost `{` that encloses off
// (the part of that line up to the brace, or the previous code line when the
// brace opens its own line). It returns "" at the top level.
func (s *gradleSource) enclosingHeader(off int) string {
	depth := 0
	for i := off - 1; i >= 0; i-- {
		switch s.code[i] {
		case '}':
			depth++
		case '{':
			if depth > 0 {
				depth--
				continue
			}
			lineStart, _ := s.lineBounds(i)
			header := strings.TrimSpace(s.code[lineStart:i])
			for header == "" && lineStart > 0 {
				lineStart, _ = s.lineBounds(lineStart - 1)
				_, lineEnd := s.lineBounds(lineStart)
				header = strings.TrimSpace(s.code[lineStart:lineEnd])
			}
			return header
		}
	}
	return ""
}

// gradleVersion is a parsed dotted version. known is false when the text held no number.
type gradleVersion struct {
	major, minor, patch int
	known               bool
}

func parseGradleVersion(s string) gradleVersion {
	var parts [3]int
	n := 0
	for _, seg := range strings.Split(strings.TrimSpace(s), ".") {
		if n == 3 {
			break
		}
		digits := 0
		for digits < len(seg) && seg[digits] >= '0' && seg[digits] <= '9' {
			digits++
		}
		if digits == 0 {
			break
		}
		v, err := strconv.Atoi(seg[:digits])
		if err != nil {
			break
		}
		parts[n] = v
		n++
		if digits < len(seg) {
			break
		}
	}
	if n == 0 {
		return gradleVersion{}
	}
	return gradleVersion{major: parts[0], minor: parts[1], patch: parts[2], known: true}
}

// atLeast reports whether the version is known and not older than major.minor.patch.
func (v gradleVersion) atLeast(major, minor, patch int) bool {
	if !v.known {
		return false
	}
	if v.major != major {
		return v.major > major
	}
	if v.minor != minor {
		return v.minor > minor
	}
	return v.patch >= patch
}

// gradleToolchain holds the Kotlin and Android Gradle plugin versions the
// project declares. A field is unknown (known == false) when no version
// literal or catalog entry fixes it; the rules never assume a latest version.
type gradleToolchain struct {
	kotlin gradleVersion
	agp    gradleVersion
}

// gradleToolchainFor resolves the project's Kotlin and AGP versions from the
// Gradle library facts, which cover build scripts and version catalogs. A
// single-script run with no facts reads the script and its catalog directly.
func gradleToolchainFor(ctx *api.Context) gradleToolchain {
	profile := gradleProfileFor(ctx)
	return gradleToolchain{
		kotlin: knownProfileVersion(profile.Kotlin.Compiler),
		agp:    knownProfileVersion(profile.Android.AGP),
	}
}

func gradleProfileFor(ctx *api.Context) librarymodel.ProjectProfile {
	if ctx.LibraryFacts != nil {
		return ctx.LibraryFacts.Profile
	}
	if ctx.GradlePath == "" {
		return librarymodel.ProjectProfile{}
	}
	catalog, _ := librarymodel.LoadVersionCatalogForGradlePath(ctx.GradlePath)
	return librarymodel.ProfileFromGradleContentWithCatalog(ctx.GradlePath, ctx.GradleContent, catalog)
}

func knownProfileVersion(v librarymodel.VersionedPresence) gradleVersion {
	if v.Presence != librarymodel.PresencePresent || v.VersionPolicy != librarymodel.VersionKnown {
		return gradleVersion{}
	}
	return parseGradleVersion(v.Version)
}

// gradleCatalogPluginID returns the plugin id behind `alias(libs.plugins.<accessor>)`,
// or "" when the catalog does not define it.
func gradleCatalogPluginID(ctx *api.Context, accessor string) string {
	catalog, ok := librarymodel.LoadVersionCatalogForGradlePath(ctx.GradlePath)
	if !ok {
		return ""
	}
	want := normalizeCatalogName(accessor)
	for alias, plugin := range catalog.Plugins {
		if normalizeCatalogName(alias) == want {
			return plugin.ID
		}
	}
	return ""
}

// gradleCatalogLibrary returns the group and name behind `libs.<accessor>`.
func gradleCatalogLibrary(ctx *api.Context, accessor string) (group, name string, ok bool) {
	catalog, found := librarymodel.LoadVersionCatalogForGradlePath(ctx.GradlePath)
	if !found {
		return "", "", false
	}
	want := normalizeCatalogName(accessor)
	for alias, lib := range catalog.Libraries {
		if normalizeCatalogName(alias) == want {
			return lib.Group, lib.Name, true
		}
	}
	return "", "", false
}

// normalizeCatalogName makes a catalog alias (`kotlin-android`) and its
// accessor (`kotlin.android`) compare equal.
func normalizeCatalogName(s string) string {
	return strings.ToLower(strings.NewReplacer("-", "", "_", "", ".", "").Replace(s))
}

// gradlePropsFile is a parsed gradle.properties file.
type gradlePropsFile struct {
	path    string
	content string
	entries []gradlePropEntry
}

type gradlePropEntry struct {
	key, value string
	line       int
	// lineStart and lineEnd bound the whole line, including its terminator.
	lineStart, lineEnd int
	// keyStart and keyEnd bound the key text.
	keyStart, keyEnd int
	prevComment      string
}

type gradlePropsCacheEntry struct {
	size    int64
	modTime time.Time
	file    *gradlePropsFile
}

var (
	gradlePropsMu    sync.Mutex
	gradlePropsCache = map[string]gradlePropsCacheEntry{}
)

func readGradleProps(path string) *gradlePropsFile {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return nil
	}
	gradlePropsMu.Lock()
	defer gradlePropsMu.Unlock()
	if hit, ok := gradlePropsCache[path]; ok && hit.size == info.Size() && hit.modTime.Equal(info.ModTime()) {
		return hit.file
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	file := parseGradleProps(path, string(data))
	gradlePropsCache[path] = gradlePropsCacheEntry{size: info.Size(), modTime: info.ModTime(), file: file}
	return file
}

func parseGradleProps(path, content string) *gradlePropsFile {
	file := &gradlePropsFile{path: path, content: content}
	offset := 0
	prev := ""
	for i, raw := range strings.SplitAfter(content, "\n") {
		if raw == "" {
			continue
		}
		line := strings.TrimRight(raw, "\r\n")
		trimmed := strings.TrimSpace(line)
		lineStart, lineEnd := offset, offset+len(raw)
		offset = lineEnd
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
			if strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "!") {
				prev = trimmed
			} else {
				prev = ""
			}
			continue
		}
		sep := strings.IndexAny(line, "=:")
		if sep < 0 {
			prev = ""
			continue
		}
		keyRaw := line[:sep]
		key := strings.TrimSpace(keyRaw)
		keyStart := lineStart + (len(keyRaw) - len(strings.TrimLeft(keyRaw, " \t")))
		file.entries = append(file.entries, gradlePropEntry{
			key: key, value: strings.TrimSpace(line[sep+1:]), line: i + 1,
			lineStart: lineStart, lineEnd: lineEnd,
			keyStart: keyStart, keyEnd: keyStart + len(key),
			prevComment: prev,
		})
		prev = ""
	}
	return file
}

// GradlePropertiesFilesFor lists the gradle.properties files that can change what
// Gradle rules report for the script at scriptPath: the script's directory and each
// parent up to the directory holding settings.gradle(.kts). Without a settings file
// above the script, only its own directory counts. The pipeline folds these files into
// the Gradle findings cache keys.
func GradlePropertiesFilesFor(scriptPath string) []string {
	dir := filepath.Dir(scriptPath)
	var chain []string
	for {
		if fileExists(filepath.Join(dir, "gradle.properties")) {
			chain = append(chain, filepath.Join(dir, "gradle.properties"))
		}
		if fileExists(filepath.Join(dir, "settings.gradle.kts")) || fileExists(filepath.Join(dir, "settings.gradle")) {
			return chain
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	own := filepath.Join(filepath.Dir(scriptPath), "gradle.properties")
	if fileExists(own) {
		return []string{own}
	}
	return nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

// gradlePropertyValue returns the value of key from the nearest gradle.properties.
func gradlePropertyValue(scriptPath, key string) (string, bool) {
	for _, path := range GradlePropertiesFilesFor(scriptPath) {
		file := readGradleProps(path)
		if file == nil {
			continue
		}
		for _, e := range file.entries {
			if e.key == key {
				return e.value, true
			}
		}
	}
	return "", false
}

// gradlePropSuppressed reports whether the comment line above a property
// suppresses ruleID with `# krit:ignore <id>` or `# noinspection <id>`.
func gradlePropSuppressed(e gradlePropEntry, ruleID string) bool {
	c := e.prevComment
	if c == "" {
		return false
	}
	for _, marker := range []string{"krit:ignore", "noinspection"} {
		if i := strings.Index(c, marker); i >= 0 {
			rest := c[i+len(marker):]
			if strings.Contains(rest, ruleID) || strings.Contains(rest, "all") {
				return true
			}
		}
	}
	return false
}

func isGradleSettingsScript(path string) bool {
	base := filepath.Base(path)
	return base == "settings.gradle.kts" || base == "settings.gradle"
}

func isKotlinDslScript(path string) bool { return strings.HasSuffix(path, ".kts") }
