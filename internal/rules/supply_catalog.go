package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// VersionCatalogUnusedRule flags aliases declared in gradle/libs.versions.toml
// that are not referenced from any Gradle build script or convention plugin.
type VersionCatalogUnusedRule struct {
	BaseRule
	IgnoredAliases        []string
	ScanConventionPlugins bool
}

// Confidence reports a tier-2 (medium) base confidence — accessor scanning
// uses substring match against the dotted accessor form, which is high-recall
// but can miss exotic reflective uses. The default-inactive policy plus the
// ignoredAliases escape hatch handle that.
func (r *VersionCatalogUnusedRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *VersionCatalogUnusedRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

func (r *VersionCatalogUnusedRule) check(ctx *api.Context) {
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

	scanFiles := r.collectScanFiles(pmi.Graph)
	corpus := readAndConcat(scanFiles)
	if corpus == "" {
		return
	}

	r.emitUnused(ctx, catalogPath, cat.Libraries, "libs", corpus)
	r.emitUnused(ctx, catalogPath, cat.Plugins, "libs.plugins", corpus)
	r.emitUnused(ctx, catalogPath, cat.Bundles, "libs.bundles", corpus)
}

func (r *VersionCatalogUnusedRule) emitUnused(ctx *api.Context, file string, entries []module.CatalogEntry, prefix, corpus string) {
	for _, entry := range entries {
		if r.aliasIgnored(entry.Alias) {
			continue
		}
		accessor := module.AccessorFor(prefix, entry.Alias)
		if accessorReferenced(corpus, accessor) {
			continue
		}
		ctx.Emit(scanner.Finding{
			File:       file,
			Line:       entry.Line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Version catalog alias '%s' (accessor '%s') is unused; remove it from libs.versions.toml or list it under ignoredAliases.", entry.Alias, accessor),
			Confidence: api.ConfidenceHigh,
		})
	}
}

func (r *VersionCatalogUnusedRule) aliasIgnored(alias string) bool {
	for _, pattern := range r.IgnoredAliases {
		if matchAliasGlob(pattern, alias) {
			return true
		}
	}
	return false
}

// matchAliasGlob supports trailing-* and leading-* wildcards. Plain strings
// match exactly. This is intentionally narrower than filepath.Match — alias
// names cannot contain slashes.
func matchAliasGlob(pattern, alias string) bool {
	if pattern == alias {
		return true
	}
	hasPrefixStar := strings.HasPrefix(pattern, "*")
	hasSuffixStar := strings.HasSuffix(pattern, "*")
	core := strings.Trim(pattern, "*")
	switch {
	case hasPrefixStar && hasSuffixStar:
		return strings.Contains(alias, core)
	case hasPrefixStar:
		return strings.HasSuffix(alias, core)
	case hasSuffixStar:
		return strings.HasPrefix(alias, core)
	}
	return false
}

func (r *VersionCatalogUnusedRule) collectScanFiles(graph *module.Graph) []string {
	var files []string
	rootDir := graph.RootDir

	addIfExists := func(path string) {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			files = append(files, path)
		}
	}

	addIfExists(filepath.Join(rootDir, "settings.gradle.kts"))
	addIfExists(filepath.Join(rootDir, "settings.gradle"))

	for _, mod := range graph.Modules {
		addIfExists(filepath.Join(mod.Dir, "build.gradle.kts"))
		addIfExists(filepath.Join(mod.Dir, "build.gradle"))
	}

	if r.ScanConventionPlugins {
		for _, sub := range []string{"build-logic", "buildSrc"} {
			root := filepath.Join(rootDir, sub)
			info, err := os.Stat(root)
			if err != nil || !info.IsDir() {
				continue
			}
			_ = filepath.Walk(root, func(path string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil || fi == nil || fi.IsDir() {
					return nil //nolint:nilerr // Walk callback skip-and-continue: per-entry error means skip this entry
				}
				name := fi.Name()
				if strings.HasSuffix(name, ".kt") || strings.HasSuffix(name, ".kts") || strings.HasSuffix(name, ".gradle") {
					files = append(files, path)
				}
				return nil
			})
		}
	}

	return files
}

func readAndConcat(files []string) string {
	var b strings.Builder
	for _, p := range files {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		b.Write(data)
		b.WriteByte('\n')
	}
	return b.String()
}

// accessorReferenced returns true when the accessor identifier appears in the
// corpus with non-identifier delimiters on both sides — guarding against
// substring matches like 'libs.foo' being satisfied by 'libs.foobar'.
func accessorReferenced(corpus, accessor string) bool {
	idx := 0
	for {
		off := strings.Index(corpus[idx:], accessor)
		if off < 0 {
			return false
		}
		start := idx + off
		end := start + len(accessor)
		if (start == 0 || !isAccessorIdentChar(corpus[start-1])) &&
			(end == len(corpus) || !isAccessorIdentChar(corpus[end])) {
			return true
		}
		idx = end
	}
}

// isAccessorIdentChar treats '.' as part of the accessor as well, so a
// shorter accessor like "libs.okhttp" does not match a longer one like
// "libs.okhttp.core" (which corresponds to a different alias).
func isAccessorIdentChar(b byte) bool {
	switch {
	case b >= 'a' && b <= 'z':
		return true
	case b >= 'A' && b <= 'Z':
		return true
	case b >= '0' && b <= '9':
		return true
	case b == '_' || b == '.':
		return true
	}
	return false
}
