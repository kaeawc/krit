package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// MissingGradleChecksumsRule flags dependency locking declarations without a sibling lockfile.
type MissingGradleChecksumsRule struct {
	BaseRule
}

func (r *MissingGradleChecksumsRule) Confidence() float64 { return api.ConfidenceHigher }

func (r *MissingGradleChecksumsRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

func (r *MissingGradleChecksumsRule) check(ctx *api.Context) {
	if ctx.ModuleIndex == nil || ctx.ModuleIndex.Graph == nil {
		return
	}
	for _, path := range gradleLockingScriptFiles(ctx.ModuleIndex.Graph.RootDir, ctx.ModuleIndex.Graph.Modules) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		line := findGradleDependencyLockingLine(string(data))
		if line == 0 {
			continue
		}
		lockfile := filepath.Join(filepath.Dir(path), "gradle.lockfile")
		if info, err := os.Stat(lockfile); err == nil && !info.IsDir() {
			continue
		}
		ctx.Emit(scanner.Finding{
			File:       path,
			Line:       line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    "Gradle dependency locking is declared but no sibling gradle.lockfile was found.",
			Confidence: r.Confidence(),
		})
	}
}

// DependencyVerificationDisabledRule flags Gradle dependency verification set to off or lenient.
type DependencyVerificationDisabledRule struct {
	BaseRule

	AllowLenient bool
}

func (r *DependencyVerificationDisabledRule) Confidence() float64 { return api.ConfidenceVeryHigh }

func (r *DependencyVerificationDisabledRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

func (r *DependencyVerificationDisabledRule) check(ctx *api.Context) {
	if ctx.ModuleIndex == nil || ctx.ModuleIndex.Graph == nil {
		return
	}
	for _, path := range gradlePropertiesFiles(ctx.ModuleIndex.Graph.RootDir, ctx.ModuleIndex.Graph.Modules) {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		for _, prop := range parseGradleProperties(string(data)) {
			if prop.key != "org.gradle.dependency.verification" {
				continue
			}
			value := strings.ToLower(strings.TrimSpace(prop.value))
			if value != "off" && value != "lenient" {
				continue
			}
			if value == "lenient" && r.AllowLenient {
				continue
			}
			ctx.Emit(scanner.Finding{
				File:       path,
				Line:       prop.line,
				Col:        1,
				RuleSet:    r.RuleSetName,
				Rule:       r.RuleName,
				Severity:   r.Sev,
				Message:    fmt.Sprintf("Gradle dependency verification is set to %q. Use strict verification to preserve dependency integrity checks.", value),
				Confidence: r.Confidence(),
			})
		}
	}
}

func gradleLockingScriptFiles(root string, modules map[string]*module.Module) []string {
	candidates := []string{
		filepath.Join(root, "settings.gradle"),
		filepath.Join(root, "settings.gradle.kts"),
		filepath.Join(root, "build.gradle"),
		filepath.Join(root, "build.gradle.kts"),
	}
	for _, mod := range modules {
		candidates = append(candidates, gradleBuildFileForModule(mod.Dir))
	}
	return existingUniqueFiles(candidates)
}

func findGradleDependencyLockingLine(content string) int {
	var dependencyLockingLine int
	var inDependencyLocking bool
	var depth int
	for i, raw := range strings.Split(content, "\n") {
		if isGradleCommentLine(raw) {
			continue
		}
		line := gradleStripStringsAndComments(raw)
		if dependencyLockingLine == 0 && gradleLineOpensNamedBlock(line, "dependencyLocking") {
			dependencyLockingLine = i + 1
			inDependencyLocking = true
		}
		if inDependencyLocking && (strings.Contains(line, "lockAllConfigurations") || strings.Contains(line, "LockMode.STRICT")) {
			return dependencyLockingLine
		}
		openBraces, closeBraces := countGradleBraces(stripGradleLineComment(raw))
		depth += openBraces - closeBraces
		if inDependencyLocking && depth <= 0 {
			inDependencyLocking = false
			dependencyLockingLine = 0
		}
	}
	return 0
}

func gradlePropertiesFiles(root string, modules map[string]*module.Module) []string {
	candidates := []string{
		filepath.Join(root, "gradle.properties"),
		filepath.Join(root, "gradle", "gradle.properties"),
	}
	for _, mod := range modules {
		candidates = append(candidates, filepath.Join(mod.Dir, "gradle.properties"))
	}
	return existingUniqueFiles(candidates)
}

func existingUniqueFiles(candidates []string) []string {
	seen := make(map[string]bool, len(candidates))
	var out []string
	for _, path := range candidates {
		clean := filepath.Clean(path)
		if seen[clean] {
			continue
		}
		seen[clean] = true
		if info, err := os.Stat(clean); err == nil && !info.IsDir() {
			out = append(out, clean)
		}
	}
	sort.Strings(out)
	return out
}

type gradleProperty struct {
	key   string
	value string
	line  int
}

func parseGradleProperties(content string) []gradleProperty {
	var props []gradleProperty
	lines := strings.Split(content, "\n")
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		startLine := i + 1
		for strings.HasSuffix(line, "\\") && i+1 < len(lines) {
			line = strings.TrimSuffix(line, "\\") + strings.TrimSpace(lines[i+1])
			i++
		}
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "!") {
			continue
		}
		key, value, ok := splitGradleProperty(line)
		if !ok {
			continue
		}
		props = append(props, gradleProperty{key: strings.TrimSpace(key), value: unquoteGradlePropertyValue(value), line: startLine})
	}
	return props
}

func splitGradleProperty(line string) (key, value string, ok bool) {
	for i, r := range line {
		if r == '=' || r == ':' || r == ' ' || r == '\t' {
			key = strings.TrimSpace(line[:i])
			value = strings.TrimSpace(line[i+len(string(r)):])
			return key, value, key != ""
		}
	}
	return "", "", false
}

func unquoteGradlePropertyValue(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1]
		}
	}
	return value
}
