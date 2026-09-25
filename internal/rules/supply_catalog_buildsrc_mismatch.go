package rules

import (
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
	cat, err := module.ParseVersionCatalog(catalogPath)
	if err != nil {
		return
	}
	coords := readCatalogCoordinates(cat)
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

// readCatalogCoordinates returns a (group:name) → coord map from the parsed
// catalog.
func readCatalogCoordinates(cat *module.VersionCatalog) map[string]catalogCoord {
	out := map[string]catalogCoord{}
	for _, lib := range cat.Libraries {
		if lib.Module == "" || lib.Version == "" {
			continue
		}
		out[lib.Module] = catalogCoord{Alias: lib.Alias, Version: lib.Version, Line: lib.Line}
	}
	return out
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
