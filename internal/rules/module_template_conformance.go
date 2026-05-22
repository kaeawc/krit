package rules

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/kaeawc/krit/internal/config"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

// ModuleTemplateConformanceRule checks that configured feature root modules
// declare the required child modules and Gradle plugins.
type ModuleTemplateConformanceRule struct {
	BaseRule
	Template config.ModuleTemplateConfig
}

// Confidence reports a tier-2 (medium) base confidence. The rule relies on
// Gradle settings/build-script text parsing rather than a full Gradle model.
func (r *ModuleTemplateConformanceRule) Confidence() float64 { return api.ConfidenceMedium }

func (r *ModuleTemplateConformanceRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{NeedsDependencies: true}
}

func (r *ModuleTemplateConformanceRule) check(ctx *api.Context) {
	if ctx.ModuleIndex == nil || ctx.ModuleIndex.Graph == nil {
		return
	}
	tmpl := r.Template
	if tmpl.FeatureRoot == "" || (len(tmpl.RequiredSubmodules) == 0 && len(tmpl.RequiredPlugins) == 0) {
		return
	}

	graph := ctx.ModuleIndex.Graph
	for modulePath, mod := range graph.Modules {
		if mod == nil || !moduleTemplatePathMatches(tmpl.FeatureRoot, modulePath) {
			continue
		}
		buildFile := moduleBuildFile(mod.Dir)
		line := 1
		col := 1
		plugins := map[string]bool{}
		if buildFile != "" {
			plugins = appliedGradlePluginIDs(buildFile)
		} else if mod.Dir != "" {
			buildFile = filepath.Join(mod.Dir, "build.gradle.kts")
		}

		for _, sub := range tmpl.RequiredSubmodules {
			child := modulePath + ":" + strings.TrimPrefix(sub, ":")
			if _, ok := graph.Modules[child]; ok {
				continue
			}
			ctx.Emit(scanner.Finding{
				File:       buildFile,
				Line:       line,
				Col:        col,
				RuleSet:    r.RuleSetName,
				Rule:       r.RuleName,
				Severity:   r.Sev,
				Message:    fmt.Sprintf("Feature module %s is missing required submodule %s.", modulePath, child),
				Confidence: r.Confidence(),
			})
		}

		for _, plugin := range tmpl.RequiredPlugins {
			if plugins[plugin] {
				continue
			}
			ctx.Emit(scanner.Finding{
				File:       buildFile,
				Line:       line,
				Col:        col,
				RuleSet:    r.RuleSetName,
				Rule:       r.RuleName,
				Severity:   r.Sev,
				Message:    fmt.Sprintf("Feature module %s is missing required Gradle plugin '%s'.", modulePath, plugin),
				Confidence: r.Confidence(),
			})
		}
	}
}

func moduleTemplatePathMatches(pattern, modulePath string) bool {
	pattern = strings.TrimPrefix(pattern, ":")
	modulePath = strings.TrimPrefix(modulePath, ":")
	if strings.Count(pattern, ":") != strings.Count(modulePath, ":") {
		return false
	}
	ok, err := path.Match(pattern, modulePath)
	return err == nil && ok
}

func moduleBuildFile(dir string) string {
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

var (
	gradlePluginIDRe    = regexp.MustCompile(`id\s*\(\s*["']([^"']+)["']\s*\)`)
	gradleApplyPluginRe = regexp.MustCompile(`apply\s+plugin:\s*["']([^"']+)["']`)
)

func appliedGradlePluginIDs(file string) map[string]bool {
	data, err := os.ReadFile(file)
	if err != nil {
		return map[string]bool{}
	}
	plugins := make(map[string]bool)
	content := string(data)
	for _, match := range gradlePluginIDRe.FindAllStringSubmatch(content, -1) {
		plugins[match[1]] = true
	}
	for _, match := range gradleApplyPluginRe.FindAllStringSubmatch(content, -1) {
		plugins[match[1]] = true
	}
	return plugins
}
