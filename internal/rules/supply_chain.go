package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/kaeawc/krit/internal/module"
	v2 "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

const supplyChainRuleSet = "supply-chain"

// CompileSdkMismatchAcrossModulesRule flags Android modules whose compileSdk is
// lower than the maximum compileSdk declared elsewhere in the same Gradle project.
type CompileSdkMismatchAcrossModulesRule struct {
	BaseRule
}

type moduleCompileSDK struct {
	modulePath string
	buildFile  string
	line       int
	compileSDK int
}

var (
	compileSDKLineRe  = regexp.MustCompile(`^\s*compileSdk(?:Version)?\s*[=(]?\s*\d+`)
	compileSDKValueRe = regexp.MustCompile(`^\s*compileSdk(?:Version)?\s*[=(]?\s*(\d+)`)
)

// Confidence reports a tier-2 (medium) base confidence. Supply-chain rule. Detection scans module metadata and dependency
// catalogs for drift and security issues; matches are
// project-structure-sensitive. Classified per roadmap/17.
func (r *CompileSdkMismatchAcrossModulesRule) Confidence() float64 { return 0.75 }

func (r *CompileSdkMismatchAcrossModulesRule) ModuleAwareNeeds() ModuleAwareNeeds {
	return ModuleAwareNeeds{}
}

func (r *CompileSdkMismatchAcrossModulesRule) check(ctx *v2.Context) {
	pmi := ctx.ModuleIndex
	if pmi == nil || pmi.Graph == nil {
		return
	}

	modules := collectAndroidModuleCompileSDKs(pmi.Graph)
	if len(modules) < 2 {
		return
	}

	maxCompileSDK := 0
	distinct := make(map[int]bool)
	for _, mod := range modules {
		distinct[mod.compileSDK] = true
		if mod.compileSDK > maxCompileSDK {
			maxCompileSDK = mod.compileSDK
		}
	}
	if len(distinct) < 2 {
		return
	}

	summary := formatCompileSDKSummary(modules)
	for _, mod := range modules {
		if mod.compileSDK >= maxCompileSDK {
			continue
		}
		ctx.Emit(scanner.Finding{
			File:       mod.buildFile,
			Line:       mod.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Module %s declares compileSdk %d, but another Android module declares %d; align compileSdk across modules to avoid merged builds silently picking the max. Project values: %s.", mod.modulePath, mod.compileSDK, maxCompileSDK, summary),
			Confidence: 0.95,
		})
	}
}

func collectAndroidModuleCompileSDKs(graph *module.ModuleGraph) []moduleCompileSDK {
	modules := make([]moduleCompileSDK, 0, len(graph.Modules))
	for _, mod := range graph.Modules {
		buildFile := gradleBuildFileForModule(mod.Dir)
		if buildFile == "" {
			continue
		}

		data, err := os.ReadFile(buildFile)
		if err != nil {
			continue
		}
		compileSDK, line, ok := findCompileSDKDeclaration(string(data))
		if !ok {
			continue
		}

		modules = append(modules, moduleCompileSDK{
			modulePath: mod.Path,
			buildFile:  buildFile,
			line:       line,
			compileSDK: compileSDK,
		})
	}

	sort.Slice(modules, func(i, j int) bool {
		return modules[i].modulePath < modules[j].modulePath
	})
	return modules
}

func gradleBuildFileForModule(dir string) string {
	for _, name := range []string{"build.gradle.kts", "build.gradle"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

func findCompileSDKLine(content string) int {
	for i, line := range strings.Split(content, "\n") {
		if compileSDKLineRe.MatchString(line) {
			return i + 1
		}
	}
	return 1
}

func findCompileSDKDeclaration(content string) (compileSDK, line int, ok bool) {
	for i, lineText := range strings.Split(content, "\n") {
		match := compileSDKValueRe.FindStringSubmatch(lineText)
		if len(match) < 2 {
			continue
		}
		value, err := strconv.Atoi(match[1])
		if err != nil || value == 0 {
			continue
		}
		return value, i + 1, true
	}
	return 0, 1, false
}

func formatCompileSDKSummary(modules []moduleCompileSDK) string {
	parts := make([]string, 0, len(modules))
	for _, mod := range modules {
		parts = append(parts, fmt.Sprintf("%s=%d", mod.modulePath, mod.compileSDK))
	}
	return strings.Join(parts, ", ")
}
