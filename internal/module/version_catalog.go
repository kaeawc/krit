package module

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// VersionCatalog is a parsed Gradle version catalog (libs.versions.toml).
type VersionCatalog struct {
	Path      string
	Versions  []CatalogEntry
	Libraries []CatalogEntry
	Plugins   []CatalogEntry
	Bundles   []CatalogEntry
}

// CatalogEntry is one alias line (the key) with its source location.
//
// Value holds the trimmed RHS text. For bare string entries
// (`kotlin = "1.9.0"`) the surrounding quotes are stripped; inline tables
// (`{ module = "...", version.ref = "..." }`) keep their braces so callers
// can distinguish literal values.
//
// For [libraries] entries, Module is the "group:name" coordinate when
// present and Version is the resolved version literal (after dereferencing
// version.ref against [versions]). Both are empty when the form is
// unrecognized. For [versions] entries, Version mirrors Value.
type CatalogEntry struct {
	Alias   string
	Line    int
	Value   string
	Module  string
	Version string
}

// ParseVersionCatalog parses a libs.versions.toml file. Multi-line inline
// tables are not supported because Gradle catalogs in practice keep each entry
// on a single line.
func ParseVersionCatalog(path string) (*VersionCatalog, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cat := &VersionCatalog{Path: path}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var section string
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := stripLineComment(sc.Text())
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
		alias := strings.Trim(strings.TrimSpace(trimmed[:eq]), "\"'")
		if alias == "" {
			continue
		}
		value := strings.TrimSpace(trimmed[eq+1:])
		entry := CatalogEntry{Alias: alias, Line: lineNum, Value: catalogValueForDuplicateCheck(value)}
		switch section {
		case "versions":
			entry.Version = unquoteTOMLString(value)
			cat.Versions = append(cat.Versions, entry)
		case "libraries":
			entry.Module, entry.Version = parseLibraryValue(value)
			cat.Libraries = append(cat.Libraries, entry)
		case "plugins":
			cat.Plugins = append(cat.Plugins, entry)
		case "bundles":
			cat.Bundles = append(cat.Bundles, entry)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	resolveVersionRefs(cat)
	return cat, nil
}

// parseLibraryValue extracts (module, version) from a [libraries] right-hand
// side. Supported forms:
//   - "group:name:version"
//   - { module = "g:n", version = "1.2.3" }
//   - { module = "g:n", version.ref = "alias" }   (resolved via [versions])
//   - { group = "g", name = "n", version = "..." }
//
// Anything unrecognized returns ("", "").
func parseLibraryValue(value string) (string, string) {
	if value == "" {
		return "", ""
	}
	if value[0] == '"' || value[0] == '\'' {
		s := unquoteTOMLString(value)
		if parts := strings.SplitN(s, ":", 3); len(parts) == 3 {
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
			version = "ref:" + ref
		}
	}
	return mod, version
}

// splitInlineTable returns key→value pairs of a single-line TOML inline table.
// Nested inline tables are not supported.
func splitInlineTable(s string) map[string]string {
	out := map[string]string{}
	inner := strings.TrimSuffix(strings.TrimPrefix(strings.TrimSpace(s), "{"), "}")
	for _, part := range splitTopLevel(inner, ',') {
		eq := strings.Index(part, "=")
		if eq <= 0 {
			continue
		}
		out[strings.TrimSpace(part[:eq])] = unquoteTOMLString(part[eq+1:])
	}
	return out
}

// splitTopLevel splits s on sep, ignoring sep inside quoted strings.
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

func unquoteTOMLString(v string) string {
	v = strings.TrimSpace(v)
	if len(v) >= 2 && (v[0] == '"' || v[0] == '\'') && v[len(v)-1] == v[0] {
		return v[1 : len(v)-1]
	}
	return v
}

// resolveVersionRefs replaces "ref:<alias>" placeholders with the literal
// value from [versions]. Unknown refs become empty strings.
func resolveVersionRefs(cat *VersionCatalog) {
	versions := map[string]string{}
	for _, v := range cat.Versions {
		versions[v.Alias] = v.Version
	}
	for i, lib := range cat.Libraries {
		if strings.HasPrefix(lib.Version, "ref:") {
			cat.Libraries[i].Version = versions[strings.TrimPrefix(lib.Version, "ref:")]
		}
	}
}

// stripLineComment removes a trailing # comment, ignoring # inside quoted
// strings. TOML's comment syntax is line-based.
func stripLineComment(line string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		switch c {
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

// FindVersionCatalog returns the path to gradle/libs.versions.toml under
// rootDir, or "" if not present.
func FindVersionCatalog(rootDir string) string {
	path := filepath.Join(rootDir, "gradle", "libs.versions.toml")
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

// catalogValueForDuplicateCheck returns the trimmed RHS for a catalog
// entry: surrounding double or single quotes stripped for bare string
// values, but inline tables (starting with `{`) returned verbatim so
// duplicate-version detection can skip them.
func catalogValueForDuplicateCheck(value string) string {
	if len(value) >= 2 {
		first, last := value[0], value[len(value)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}

// AccessorFor returns the Kotlin accessor string for a catalog alias under
// the given prefix. Hyphens and underscores in the alias become dots, per
// Gradle's accessor convention.
func AccessorFor(prefix, alias string) string {
	if alias == "" {
		return prefix
	}
	dotted := strings.NewReplacer("-", ".", "_", ".").Replace(alias)
	return prefix + "." + dotted
}
