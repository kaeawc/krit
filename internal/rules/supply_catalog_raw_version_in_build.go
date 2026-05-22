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

// VersionCatalogRawVersionInBuildRule flags hardcoded "group:name:version"
// dependency literals inside Gradle build scripts when the same coordinate is
// already declared in the project's libs.versions.toml.
type VersionCatalogRawVersionInBuildRule struct {
	BaseRule
}

func (r *VersionCatalogRawVersionInBuildRule) Confidence() float64 { return api.ConfidenceMediumHigh }

func (r *VersionCatalogRawVersionInBuildRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

func (r *VersionCatalogRawVersionInBuildRule) check(ctx *api.Context) {
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
	coordToAlias := map[string]string{}
	for _, lib := range cat.Libraries {
		if lib.Module != "" {
			coordToAlias[lib.Module] = lib.Alias
		}
	}
	if len(coordToAlias) == 0 {
		return
	}

	for _, mod := range pmi.Graph.Modules {
		for _, name := range []string{"build.gradle.kts", "build.gradle"} {
			r.scanBuildFile(ctx, filepath.Join(mod.Dir, name), coordToAlias)
		}
	}
}

func (r *VersionCatalogRawVersionInBuildRule) scanBuildFile(ctx *api.Context, path string, coordToAlias map[string]string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	inBlock := false
	for i, line := range strings.Split(string(data), "\n") {
		code := maskGradleComments(line, &inBlock)
		for _, lit := range collectStringLiterals(code) {
			coord, version, ok := splitCoordinate(lit.text)
			if !ok {
				continue
			}
			alias, found := coordToAlias[coord]
			if !found {
				continue
			}
			ctx.Emit(scanner.Finding{
				File:       path,
				Line:       i + 1,
				Col:        lit.col,
				RuleSet:    r.RuleSetName,
				Rule:       r.RuleName,
				Severity:   r.Sev,
				Message:    fmt.Sprintf("Dependency '%s:%s' is declared in the version catalog as alias '%s'; replace the literal with the catalog accessor '%s'.", coord, version, alias, module.AccessorFor("libs", alias)),
				Confidence: r.Confidence(),
			})
		}
	}
}

// maskGradleComments masks comment runs ("// ..." and "/* ... */") with
// spaces so column positions of surviving content are preserved. inBlock
// tracks whether the previous line ended inside an open block comment.
func maskGradleComments(line string, inBlock *bool) string {
	out := []byte(line)
	i := 0
	for i < len(out) {
		if *inBlock {
			if i+1 < len(out) && out[i] == '*' && out[i+1] == '/' {
				out[i], out[i+1] = ' ', ' '
				*inBlock = false
				i += 2
				continue
			}
			out[i] = ' '
			i++
			continue
		}
		c := out[i]
		if c == '/' && i+1 < len(out) {
			if out[i+1] == '/' {
				for j := i; j < len(out); j++ {
					out[j] = ' '
				}
				break
			}
			if out[i+1] == '*' {
				out[i], out[i+1] = ' ', ' '
				*inBlock = true
				i += 2
				continue
			}
		}
		if c == '"' {
			j := i + 1
			for j < len(out) {
				if out[j] == '\\' && j+1 < len(out) {
					j += 2
					continue
				}
				if out[j] == '"' {
					break
				}
				j++
			}
			i = j + 1
			continue
		}
		i++
	}
	return string(out)
}

// stringLiteral is a quoted span found inside a single source line, with its
// 1-based start column.
type stringLiteral struct {
	text string
	col  int
}

// collectStringLiterals returns every double-quoted string on a line.
// Backslash escapes are honored. Triple-quoted Kotlin raw strings are not
// handled separately — coordinate matching only fires on the precise
// "g:n:v" shape, so a stray multiline open quote yields at worst a missed
// finding on that line.
func collectStringLiterals(line string) []stringLiteral {
	var out []stringLiteral
	for i := 0; i < len(line); i++ {
		if line[i] != '"' {
			continue
		}
		start := i + 1
		j := start
		for j < len(line) {
			if line[j] == '\\' && j+1 < len(line) {
				j += 2
				continue
			}
			if line[j] == '"' {
				break
			}
			j++
		}
		if j >= len(line) {
			break
		}
		out = append(out, stringLiteral{text: line[start:j], col: start + 1})
		i = j
	}
	return out
}

// splitCoordinate returns ("group:name", "version", true) when s looks like a
// Gradle dependency notation. The group and name segments must be
// identifier-shaped (letters, digits, ., _, -); the version segment must be
// non-empty and contain at least one digit. Anything else returns ok=false.
func splitCoordinate(s string) (string, string, bool) {
	parts := strings.Split(s, ":")
	if len(parts) != 3 {
		return "", "", false
	}
	group, name, version := parts[0], parts[1], parts[2]
	if !isCoordIdent(group) || !isCoordIdent(name) || version == "" {
		return "", "", false
	}
	hasDigit := false
	for i := 0; i < len(version); i++ {
		if version[i] >= '0' && version[i] <= '9' {
			hasDigit = true
			break
		}
	}
	if !hasDigit {
		return "", "", false
	}
	return group + ":" + name, version, true
}

func isCoordIdent(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
		case c >= 'A' && c <= 'Z':
		case c >= '0' && c <= '9':
		case c == '.' || c == '_' || c == '-':
		default:
			return false
		}
	}
	return true
}
