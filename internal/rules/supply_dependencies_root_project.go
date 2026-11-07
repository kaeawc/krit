package rules

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"gopkg.in/yaml.v3"
)

// DependenciesInRootProjectRule flags application/runtime dependencies declared
// directly in the root Gradle project instead of in an owning module.
type DependenciesInRootProjectRule struct {
	GradleBase
	BaseRule

	AllowedConfigurations []string
}

func (r *DependenciesInRootProjectRule) Confidence() float64 { return api.ConfidenceHigh }

// Suggested-fix identifiers and canonical titles for the
// DependenciesInRootProject rule. The titles are shared between the rule's
// registry catalog (api.Rule.SuggestedFixes) and the per-finding emit so
// IDE quick-fix menus and the CLI list view show the same label.
const (
	DependenciesInRootProjectMoveSuggestionID     = "moveToOwningModule"
	dependenciesInRootProjectMoveSuggestionTitle  = "Move dependencies into an owning module"
	DependenciesInRootProjectAllowSuggestionID    = "addAllowedConfigurations"
	dependenciesInRootProjectAllowSuggestionTitle = "Add configurations to allowedConfigurations in the root Krit config"
)

func (r *DependenciesInRootProjectRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isRootGradleProjectScript(path) {
		return
	}
	allowed := rootDependencyAllowedConfigurations(r.AllowedConfigurations)
	var rootBlocks []rootProjectDependencyBlock
	if astFile := ctx.GradleAST(); astFile != nil {
		rootBlocks = findRootProjectDependencyBlocksAST(astFile, allowed)
	} else {
		rootBlocks = findRootProjectDependencyBlocks(content, allowed)
	}
	for _, block := range rootBlocks {
		finding := scanner.Finding{
			File:       path,
			Line:       block.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Root project dependencies block declares %s. Move project dependencies into an owning module or add legitimate root tooling configurations to allowedConfigurations.", strings.Join(block.configurations, ", ")),
			Confidence: r.Confidence(),
		}
		finding.SuggestedFixes = r.suggestedFixesForBlock(path, block.configurations)
		ctx.Emit(finding)
	}
}

func (r *DependenciesInRootProjectRule) suggestedFixesForBlock(gradlePath string, missing []string) []scanner.SuggestedFix {
	joined := strings.Join(missing, ", ")
	suggestions := []scanner.SuggestedFix{
		{
			ID:     DependenciesInRootProjectMoveSuggestionID,
			Title:  dependenciesInRootProjectMoveSuggestionTitle,
			Detail: fmt.Sprintf("Declare %s in the module that owns the code instead of the root project. Root build scripts should configure plugins and conventions; application and runtime dependencies belong to an owning module's build.gradle(.kts).", joined),
		},
	}

	allowSuggestion := scanner.SuggestedFix{
		ID:     DependenciesInRootProjectAllowSuggestionID,
		Title:  dependenciesInRootProjectAllowSuggestionTitle,
		Detail: fmt.Sprintf("Add %s to allowedConfigurations in krit.yml so the listed root-project configurations no longer trigger this rule. Use this when the root dependency block is intentional (e.g. classpath, detektPlugins, lintChecks).", joined),
	}
	if edit, ok := rootDependencyAllowedConfigurationsEdit(gradlePath, r.AllowedConfigurations, missing); ok {
		allowSuggestion.Edits = []scanner.SuggestedEdit{edit}
	}
	return append(suggestions, allowSuggestion)
}

func rootDependencyAllowedConfigurationsEdit(gradlePath string, existingAllowed, missing []string) (scanner.SuggestedEdit, bool) {
	targetPath, content, ok := rootKritConfigForFix(filepath.Dir(gradlePath))
	if !ok {
		return scanner.SuggestedEdit{}, false
	}
	replacement, ok := mergeRootDependencyAllowedConfigurations(content, existingAllowed, missing)
	if !ok {
		return scanner.SuggestedEdit{}, false
	}
	return scanner.SuggestedEdit{
		TargetFile:  targetPath,
		ByteMode:    true,
		StartByte:   0,
		EndByte:     len(content),
		Replacement: replacement,
	}, true
}

func rootKritConfigForFix(root string) (string, []byte, bool) {
	for _, name := range []string{"krit.yml", ".krit.yml"} {
		path := filepath.Join(root, name)
		content, err := os.ReadFile(path)
		if err == nil {
			return path, content, true
		}
		if err != nil && !os.IsNotExist(err) {
			return "", nil, false
		}
	}
	return filepath.Join(root, "krit.yml"), nil, true
}

func mergeRootDependencyAllowedConfigurations(content []byte, existingAllowed, missing []string) (string, bool) {
	header := leadingYAMLCommentHeader(content)
	root := make(map[string]any)
	if len(strings.TrimSpace(string(content))) > 0 {
		if err := yaml.Unmarshal(content, &root); err != nil {
			return "", false
		}
		if root == nil {
			root = make(map[string]any)
		}
	}

	supplyChain := yamlMap(root["supply-chain"])
	root["supply-chain"] = supplyChain
	rule := yamlMap(supplyChain["DependenciesInRootProject"])
	supplyChain["DependenciesInRootProject"] = rule

	allowed := append([]string(nil), yamlStringList(rule["allowedConfigurations"])...)
	allowed = appendMissingStrings(allowed, existingAllowed...)
	allowed = appendMissingStrings(allowed, missing...)
	rule["allowedConfigurations"] = allowed

	out, err := yaml.Marshal(root)
	if err != nil {
		return "", false
	}
	if len(out) == 0 || out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	return header + string(out), true
}

func leadingYAMLCommentHeader(content []byte) string {
	text := string(content)
	if !strings.HasPrefix(text, "#") {
		return ""
	}
	offset := 0
	for offset < len(text) {
		next := strings.IndexByte(text[offset:], '\n')
		lineEnd := len(text)
		if next >= 0 {
			lineEnd = offset + next + 1
		}
		line := text[offset:lineEnd]
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") {
			return text[:offset]
		}
		offset = lineEnd
	}
	return text
}

func yamlMap(value any) map[string]any {
	if m, ok := value.(map[string]any); ok && m != nil {
		return m
	}
	return make(map[string]any)
}

func yamlStringList(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	items := make([]string, 0, len(raw))
	for _, item := range raw {
		if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
			items = append(items, s)
		}
	}
	return items
}

func appendMissingStrings(items []string, values ...string) []string {
	seen := make(map[string]bool, len(items)+len(values))
	for _, item := range items {
		seen[item] = true
	}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		items = append(items, value)
		seen[value] = true
	}
	return items
}

type rootProjectDependencyBlock struct {
	line           int
	configurations []string
}

func rootDependencyAllowedConfigurations(configs []string) map[string]bool {
	allowed := map[string]bool{
		"classpath":     true,
		"detektPlugins": true,
	}
	for _, cfg := range configs {
		cfg = strings.TrimSpace(cfg)
		if cfg != "" {
			allowed[cfg] = true
		}
	}
	return allowed
}

func isRootGradleProjectScript(path string) bool {
	base := filepath.Base(path)
	switch base {
	case "settings.gradle", "settings.gradle.kts":
		return true
	case "build.gradle", "build.gradle.kts":
		dir := filepath.Dir(path)
		if !dirHasSettingsScript(dir) || !settingsIncludesSubprojects(dir) {
			return false
		}
		for ancestor := filepath.Dir(dir); ; ancestor = filepath.Dir(ancestor) {
			if dirHasSettingsScript(ancestor) {
				return false
			}
			if filepath.Dir(ancestor) == ancestor {
				break
			}
		}
		return true
	}
	return false
}

var settingsIncludeRe = regexp.MustCompile(`(?m)^\s*include(?:Flat)?\b\s*\(?\s*['"]`)

// settingsIncludesSubprojects reports whether the settings script in dir
// includes other projects. In a single-project build the root build script is
// the only module, so its dependencies are not misplaced.
func settingsIncludesSubprojects(dir string) bool {
	for _, settings := range []string{"settings.gradle.kts", "settings.gradle"} {
		data, err := os.ReadFile(filepath.Join(dir, settings))
		if err != nil {
			continue
		}
		return settingsIncludeRe.MatchString(string(data))
	}
	return false
}

func dirHasSettingsScript(dir string) bool {
	for _, settings := range []string{"settings.gradle.kts", "settings.gradle"} {
		if _, err := os.Stat(filepath.Join(dir, settings)); err == nil {
			return true
		}
	}
	return false
}

func findRootProjectDependencyBlocks(content string, allowed map[string]bool) []rootProjectDependencyBlock {
	var blocks []rootProjectDependencyBlock
	depth := 0
	inDependencies := false
	dependenciesDepth := 0
	blockLine := 0
	blockConfigs := make(map[string]bool)

	for i, rawLine := range strings.Split(content, "\n") {
		code := gradleStripStringsAndComments(rawLine)
		if !inDependencies && depth == 0 && gradleLineStartsBlockCall(code, "dependencies") {
			inDependencies = true
			dependenciesDepth = depth
			blockLine = i + 1
			blockConfigs = make(map[string]bool)
		}

		if inDependencies {
			for _, cfg := range gradleConfigurationCalls(code) {
				if cfg == "dependencies" || allowed[cfg] {
					continue
				}
				blockConfigs[cfg] = true
			}
		}

		openBraces, closeBraces := countGradleBraces(code)
		depth += openBraces - closeBraces
		if depth < 0 {
			depth = 0
		}
		if inDependencies && depth <= dependenciesDepth {
			configs := sortedGradleConfigurations(blockConfigs)
			if len(configs) > 0 {
				blocks = append(blocks, rootProjectDependencyBlock{
					line:           blockLine,
					configurations: configs,
				})
			}
			inDependencies = false
		}
	}
	return blocks
}

// findRootProjectDependencyBlocksAST is the tree-sitter equivalent of
// findRootProjectDependencyBlocks for Kotlin DSL (.kts) scripts. It flags a
// top-level `dependencies { }` call — one with no enclosing block — so the
// configuration set is collected by parser structure. A `dependencies { }`
// nested inside buildscript { }, subprojects { }, allprojects { }, etc. has a
// lambda_literal ancestor and is correctly ignored, and string/comment
// lookalikes never parse as calls.
func findRootProjectDependencyBlocksAST(file *scanner.File, allowed map[string]bool) []rootProjectDependencyBlock {
	var blocks []rootProjectDependencyBlock
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if flatCallNameAny(file, idx) != "dependencies" || !isTopLevelCallAST(file, idx) {
			return
		}
		// A block gated on a CI or task condition is tooling wiring, not a module dependency.
		if rootDependenciesGatedByCondition(file, idx) {
			return
		}
		lambda := flatCallTrailingLambda(file, idx)
		if lambda == 0 {
			return
		}
		if configs := collectRootDependencyConfigsAST(file, lambda, allowed); len(configs) > 0 {
			blocks = append(blocks, rootProjectDependencyBlock{
				line:           file.FlatRow(idx) + 1,
				configurations: configs,
			})
		}
	})
	return blocks
}

// isTopLevelCallAST reports whether a call sits at file scope — reached from
// source_file without crossing a lambda_literal (i.e. not inside any { } block).
func isTopLevelCallAST(file *scanner.File, idx uint32) bool {
	for node := idx; node != 0; {
		parent, ok := file.FlatParent(node)
		if !ok {
			return true
		}
		switch file.FlatType(parent) {
		case "lambda_literal":
			return false
		case "source_file":
			return true
		}
		node = parent
	}
	return true
}

// collectRootDependencyConfigsAST returns the sorted set of configuration names
// invoked as statements inside a dependencies lambda, excluding `dependencies`
// itself and any allowed root-tooling configurations. Only statement-level
// calls count, so a nested argument call such as platform(...) or project(...)
// in implementation(platform(...)) is not mistaken for a configuration.
func collectRootDependencyConfigsAST(file *scanner.File, lambda uint32, allowed map[string]bool) []string {
	found := make(map[string]bool)
	file.FlatWalkNodes(lambda, "call_expression", func(call uint32) {
		parent, ok := file.FlatParent(call)
		if !ok || file.FlatType(parent) != "statements" {
			return
		}
		// Calls in nested lambdas (`forEach { add(...) }`) are not the block's own configurations.
		if owner, ok := file.FlatParent(parent); !ok || owner != lambda {
			return
		}
		// A configuration is a bare call; a receiver call such as `list.filterNot { }` is plain Kotlin.
		if !rootDependencyCallIsBare(file, call) {
			return
		}
		name := flatCallNameAny(file, call)
		if name == "" || name == "dependencies" || allowed[name] || rootDependencyKotlinHelpers[name] {
			return
		}
		found[name] = true
	})
	return sortedGradleConfigurations(found)
}

// rootDependencyKotlinHelpers are receiver-less Kotlin functions that can appear as statements in a
// dependencies block without declaring a configuration.
var rootDependencyKotlinHelpers = map[string]bool{
	"run": true, "with": true, "apply": true, "also": true, "let": true, "repeat": true, "check": true,
	"require": true, "println": true, "error": true, "listOf": true, "mapOf": true, "setOf": true,
}

// rootDependencyCallIsBare reports a call written without a receiver, such as `implementation(x)`.
func rootDependencyCallIsBare(file *scanner.File, call uint32) bool {
	for child := file.FlatFirstChild(call); child != 0; child = file.FlatNextSib(child) {
		switch file.FlatType(child) {
		case "navigation_expression":
			return false
		case "call_expression":
			// Trailing-lambda form: the callee lives on the nested call.
			return rootDependencyCallIsBare(file, child)
		}
	}
	return true
}

var rootDependencyConditionMarkers = []string{"CI", "getenv", "taskNames", "startParameter", "gradle.", "findProperty", "hasProperty", "providers."}

// rootDependenciesGatedByCondition reports a dependencies call under an if or when whose condition reads CI or build-task state.
func rootDependenciesGatedByCondition(file *scanner.File, idx uint32) bool {
	for cur, ok := file.FlatParent(idx); ok; cur, ok = file.FlatParent(cur) {
		switch file.FlatType(cur) {
		case "if_expression", "when_expression":
			cond, _, _ := composeIfParts(file, cur)
			text := file.FlatNodeText(cur)
			if cond != 0 {
				text = file.FlatNodeText(cond)
			}
			for _, marker := range rootDependencyConditionMarkers {
				if strings.Contains(text, marker) {
					return true
				}
			}
		}
	}
	return false
}

func gradleLineStartsBlockCall(line, name string) bool {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, name) {
		return false
	}
	rest := strings.TrimSpace(strings.TrimPrefix(trimmed, name))
	return strings.HasPrefix(rest, "{") || strings.HasPrefix(rest, "(")
}

func gradleConfigurationCalls(line string) []string {
	var calls []string
	for _, segment := range strings.FieldsFunc(line, func(r rune) bool { return r == '{' || r == ';' }) {
		segment = strings.TrimSpace(segment)
		if segment == "" || strings.HasPrefix(segment, "}") {
			continue
		}
		name := leadingGradleIdentifier(segment)
		if name != "" {
			calls = append(calls, name)
		}
	}
	return calls
}

func leadingGradleIdentifier(line string) string {
	for i, r := range line {
		if i == 0 {
			if r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' {
				continue
			}
			return ""
		}
		if r != '_' && r != '-' && r != '.' && (r < '0' || r > '9') && (r < 'A' || r > 'Z') && (r < 'a' || r > 'z') {
			return line[:i]
		}
	}
	return line
}

func sortedGradleConfigurations(configs map[string]bool) []string {
	out := make([]string, 0, len(configs))
	for cfg := range configs {
		out = append(out, cfg)
	}
	sort.Strings(out)
	return out
}
