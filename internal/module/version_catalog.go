package module

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/versioncatalog"
)

// VersionCatalog is a parsed Gradle version catalog (libs.versions.toml).
type VersionCatalog struct {
	Path      string
	Versions  []CatalogEntry
	Libraries []CatalogEntry
	Plugins   []CatalogEntry
	Bundles   []CatalogEntry
}

// CatalogEntry is one alias line (the key) with its source location. For
// [versions] entries, Value is the plain literal string when the version is a
// bare TOML string, and empty for rich/table version forms. For [libraries],
// [plugins], and [bundles] entries, Value is always empty; structured values
// come from Module and Version, and richer information is available in
// internal/versioncatalog if needed.
type CatalogEntry struct {
	Alias   string
	Line    int
	Value   string
	Module  string
	Version string
}

// ParseVersionCatalog parses a libs.versions.toml file.
func ParseVersionCatalog(path string) (*VersionCatalog, error) {
	parsed, err := versioncatalog.ParseFile(path)
	if err != nil {
		return nil, err
	}
	cat := &VersionCatalog{Path: path}
	for _, e := range parsed.Versions {
		value := ""
		if e.Version.Plain {
			value = e.Version.Require
		}
		cat.Versions = append(cat.Versions, CatalogEntry{Alias: e.Alias, Line: e.Pos.Line, Version: e.Version.Effective(), Value: value})
	}
	for _, lib := range parsed.Libraries {
		cat.Libraries = append(cat.Libraries, CatalogEntry{Alias: lib.Alias, Line: lib.Pos.Line, Module: lib.Module(), Version: lib.Resolved})
	}
	for _, p := range parsed.Plugins {
		cat.Plugins = append(cat.Plugins, CatalogEntry{Alias: p.Alias, Line: p.Pos.Line})
	}
	for _, b := range parsed.Bundles {
		cat.Bundles = append(cat.Bundles, CatalogEntry{Alias: b.Alias, Line: b.Pos.Line})
	}
	return cat, nil
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
