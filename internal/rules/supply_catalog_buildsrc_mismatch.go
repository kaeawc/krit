package rules

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// VersionCatalogBuildSrcMismatchRule flags Gradle dependency coordinates
// referenced from buildSrc/ or build-logic/ Kotlin sources whose version
// disagrees with the version pinned in gradle/libs.versions.toml. The
// catalog is treated as the source of truth.
type VersionCatalogBuildSrcMismatchRule struct {
	BaseRule
}

func (r *VersionCatalogBuildSrcMismatchRule) Confidence() float64 { return api.ConfidenceMediumHigh }

func (r *VersionCatalogBuildSrcMismatchRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

// catalogCoord captures one (group:name) entry from the catalog so we can
// emit a finding pointing at the exact line of the conflicting alias.
type catalogCoord struct {
	Alias   string
	Version string
	Line    int
}

func (r *VersionCatalogBuildSrcMismatchRule) check(ctx *api.Context) {
	pmi := ctx.ModuleIndex
	if pmi == nil || pmi.Graph == nil {
		return
	}
	catalogPath := module.FindVersionCatalog(pmi.Graph.RootDir)
	if catalogPath == "" {
		return
	}
	// module.ParseVersionCatalog gives us alias + line; we still need the
	// resolved (group:name)→version coordinates that the shared parser
	// intentionally drops, so do a focused second pass over the same file.
	cat, err := module.ParseVersionCatalog(catalogPath)
	if err != nil {
		return
	}
	coords := readCatalogCoordinates(catalogPath, cat)
	if len(coords) == 0 {
		return
	}

	for _, dir := range buildSrcDirs(pmi.Graph.RootDir) {
		_ = filepath.Walk(dir, func(path string, fi os.FileInfo, walkErr error) error {
			if walkErr != nil || fi == nil || fi.IsDir() {
				return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
			}
			if !strings.HasSuffix(fi.Name(), ".kt") {
				return nil
			}
			r.scanKotlin(ctx, catalogPath, path, coords)
			return nil
		})
	}
}

// readCatalogCoordinates returns a (group:name) → coord map by re-reading the
// catalog file. We line up entries with the alias/line list returned by the
// shared parser so a single source of truth still owns alias enumeration.
func readCatalogCoordinates(path string, cat *module.VersionCatalog) map[string]catalogCoord {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	versions := map[string]string{}
	libraryValues := map[string]string{} // alias → raw library RHS
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := stripTOMLComment(scanner.Text())
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.ToLower(strings.TrimSpace(trimmed[1 : len(trimmed)-1]))
			continue
		}
		eq := strings.Index(trimmed, "=")
		if eq <= 0 {
			continue
		}
		key := strings.Trim(strings.TrimSpace(trimmed[:eq]), "\"'")
		value := strings.TrimSpace(trimmed[eq+1:])
		switch section {
		case "versions":
			versions[key] = unquote(value)
		case "libraries":
			libraryValues[key] = value
		}
	}

	out := map[string]catalogCoord{}
	for _, lib := range cat.Libraries {
		mod, ver := parseLibraryRHS(libraryValues[lib.Alias], versions)
		if mod == "" || ver == "" {
			continue
		}
		out[mod] = catalogCoord{Alias: lib.Alias, Version: ver, Line: lib.Line}
	}
	return out
}

// parseLibraryRHS extracts (group:name, version) from a [libraries] entry's
// right-hand side. Supported forms:
//   - "group:name:version"
//   - { module = "g:n", version = "1.2.3" }
//   - { module = "g:n", version.ref = "alias" }
//   - { group = "g", name = "n", version = "..." | version.ref = "..." }
func parseLibraryRHS(value string, versions map[string]string) (string, string) {
	if value == "" {
		return "", ""
	}
	if value[0] == '"' || value[0] == '\'' {
		s := unquote(value)
		parts := strings.SplitN(s, ":", 3)
		if len(parts) == 3 {
			return parts[0] + ":" + parts[1], parts[2]
		}
		return "", ""
	}
	if value[0] != '{' {
		return "", ""
	}
	fields := splitInlineTable(value)
	mod := fields["module"]
	if mod == "" {
		if g, n := fields["group"], fields["name"]; g != "" && n != "" {
			mod = g + ":" + n
		}
	}
	version := fields["version"]
	if version == "" {
		if ref := fields["version.ref"]; ref != "" {
			version = versions[ref]
		}
	}
	return mod, version
}

// splitInlineTable parses a single-line TOML inline table into key→value
// pairs. Values are returned with quoting stripped.
func splitInlineTable(s string) map[string]string {
	out := map[string]string{}
	inner := strings.TrimSpace(s)
	inner = strings.TrimPrefix(inner, "{")
	inner = strings.TrimSuffix(inner, "}")
	for _, part := range splitTopLevel(inner, ',') {
		eq := strings.Index(part, "=")
		if eq <= 0 {
			continue
		}
		out[strings.TrimSpace(part[:eq])] = unquote(strings.TrimSpace(part[eq+1:]))
	}
	return out
}

func splitTopLevel(s string, sep byte) []string {
	var out []string
	var inSingle, inDouble bool
	last := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case sep:
			if !inSingle && !inDouble {
				out = append(out, s[last:i])
				last = i + 1
			}
		}
	}
	return append(out, s[last:])
}

func unquote(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}

// stripTOMLComment removes a trailing # comment, ignoring # inside quoted
// strings. Mirrors module.stripLineComment but kept private here because it
// is used only by this rule's focused parse.
func stripTOMLComment(line string) string {
	var inSingle, inDouble bool
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '#':
			if !inSingle && !inDouble {
				return line[:i]
			}
		}
	}
	return line
}

func buildSrcDirs(rootDir string) []string {
	var dirs []string
	for _, sub := range []string{"buildSrc", "build-logic"} {
		p := filepath.Join(rootDir, sub)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			dirs = append(dirs, p)
		}
	}
	return dirs
}

func (r *VersionCatalogBuildSrcMismatchRule) scanKotlin(ctx *api.Context, catalogPath, path string, coords map[string]catalogCoord) {
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil || file == nil || file.FlatTree == nil {
		return
	}
	seen := map[string]bool{} // (alias|buildSrcLine) — one finding per literal
	for _, nodeType := range []string{"line_string_literal", "string_literal"} {
		file.FlatWalkNodes(0, nodeType, func(idx uint32) {
			text := file.FlatNodeText(idx)
			s := stripQuotes(text)
			if s == "" || strings.Count(s, ":") != 2 {
				return
			}
			parts := strings.SplitN(s, ":", 3)
			group, name, version := parts[0], parts[1], parts[2]
			if group == "" || name == "" || version == "" {
				return
			}
			coord, ok := coords[group+":"+name]
			if !ok || coord.Version == version {
				return
			}
			row := file.FlatRow(idx) + 1
			key := fmt.Sprintf("%s|%s|%d", coord.Alias, path, row)
			if seen[key] {
				return
			}
			seen[key] = true
			ctx.Emit(scanner.Finding{
				File:       catalogPath,
				Line:       coord.Line,
				Col:        1,
				RuleSet:    r.RuleSetName,
				Rule:       r.RuleName,
				Severity:   r.Sev,
				Message:    fmt.Sprintf("Version catalog alias '%s' pins %s:%s to %s, but %s:%d uses %s. Reconcile to keep the catalog the single source of truth.", coord.Alias, group, name, coord.Version, path, row, version),
				Confidence: r.Confidence(),
			})
		})
	}
}

// stripQuotes removes surrounding "..." or '...' or """...""" quoting from a
// Kotlin string literal's raw text. Tree-sitter returns the literal with its
// delimiters intact.
func stripQuotes(s string) string {
	if strings.HasPrefix(s, `"""`) && strings.HasSuffix(s, `"""`) && len(s) >= 6 {
		return s[3 : len(s)-3]
	}
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}
