package rules

import (
	"fmt"
	"path/filepath"
	"strings"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

type ApplyPluginTwiceRule struct {
	GradleBase
	BaseRule
}

func (r *ApplyPluginTwiceRule) Confidence() float64 { return api.ConfidenceHigh }

func (r *ApplyPluginTwiceRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isGradleBuildScript(path) {
		return
	}
	var duplicates []applyPluginTwiceFinding
	if astFile := ctx.GradleAST(); astFile != nil {
		duplicates = findApplyPluginTwiceAST(astFile)
	} else {
		duplicates = findApplyPluginTwice(content)
	}
	for _, duplicate := range duplicates {
		finding := scanner.Finding{
			File:       path,
			Line:       duplicate.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("Plugin %q is applied in both plugins { } and apply(plugin = ...). Keep a single application form.", duplicate.id),
			Confidence: r.Confidence(),
		}
		finding.Fix = deleteContentLineFix(content, duplicate.line)
		ctx.Emit(finding)
	}
}

// deleteContentLineFix returns a byte-mode Fix that removes the 1-indexed
// line from a raw text buffer. Used by Gradle rules that operate on
// ctx.GradleContent rather than a parsed scanner.File. Returns nil when
// the line number is out of range.
func deleteContentLineFix(content string, line int) *scanner.Fix {
	if line < 1 {
		return nil
	}
	currentLine := 1
	start := 0
	for start < len(content) && currentLine < line {
		if content[start] == '\n' {
			currentLine++
		}
		start++
	}
	if currentLine != line {
		return nil
	}
	end := start
	for end < len(content) && content[end] != '\n' {
		end++
	}
	if end < len(content) && content[end] == '\n' {
		end++
	} else if start > 0 && content[start-1] == '\n' {
		start--
	}
	return &scanner.Fix{
		ByteMode:    true,
		StartByte:   start,
		EndByte:     end,
		Replacement: "",
	}
}

type applyPluginTwiceFinding struct {
	id   string
	line int
}

type gradlePluginOccurrence struct {
	id   string
	line int
}

func findApplyPluginTwice(content string) []applyPluginTwiceFinding {
	declared := map[string]gradlePluginOccurrence{}
	var applied []gradlePluginOccurrence
	depth := 0
	inPluginsBlock := false
	pluginsBlockDepth := 0

	for i, rawLine := range strings.Split(content, "\n") {
		lineNo := i + 1
		code := gradleStripStringsAndComments(rawLine)
		rawNoComment := stripGradleLineComment(rawLine)
		if !inPluginsBlock && gradlePluginsBlockStarts(code) {
			inPluginsBlock = true
			pluginsBlockDepth = depth
		}
		if inPluginsBlock {
			for _, occurrence := range gradlePluginBlockOccurrences(rawNoComment, lineNo) {
				if _, ok := declared[occurrence.id]; !ok {
					declared[occurrence.id] = occurrence
				}
			}
		}
		applied = append(applied, gradleApplyPluginOccurrences(rawNoComment, lineNo)...)

		openBraces, closeBraces := countGradleBraces(code)
		depth += openBraces - closeBraces
		if depth < 0 {
			depth = 0
		}
		if inPluginsBlock && depth <= pluginsBlockDepth {
			inPluginsBlock = false
		}
	}

	seen := map[string]bool{}
	var findings []applyPluginTwiceFinding
	for _, occurrence := range applied {
		if seen[occurrence.id] {
			continue
		}
		if _, ok := declared[occurrence.id]; ok {
			findings = append(findings, applyPluginTwiceFinding(occurrence))
			seen[occurrence.id] = true
		}
	}
	return findings
}

// findApplyPluginTwiceAST is the tree-sitter equivalent of
// findApplyPluginTwice for Kotlin DSL (.kts) scripts. It walks the flat AST
// instead of brace-counting lines, so string-literal lookalikes, comments,
// and chained `apply false` declarations are handled by parser structure
// rather than lexical heuristics.
func findApplyPluginTwiceAST(file *scanner.File) []applyPluginTwiceFinding {
	declared := map[string]gradlePluginOccurrence{}
	var applied []gradlePluginOccurrence

	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		switch flatCallExpressionName(file, idx) {
		case "plugins":
			collectPluginsBlockIDsAST(file, idx, declared)
		case "apply":
			if id, ok := applyPluginIDAST(file, idx); ok {
				applied = append(applied, gradlePluginOccurrence{id: id, line: file.FlatRow(idx) + 1})
			}
		}
	})

	seen := map[string]bool{}
	var findings []applyPluginTwiceFinding
	for _, occurrence := range applied {
		if seen[occurrence.id] {
			continue
		}
		if _, ok := declared[occurrence.id]; ok {
			findings = append(findings, applyPluginTwiceFinding(occurrence))
			seen[occurrence.id] = true
		}
	}
	return findings
}

// collectPluginsBlockIDsAST records every plugin id declared inside the
// trailing lambda of a `plugins { }` call. `id("x")` yields its literal id;
// `kotlin("jvm")` is normalized through kotlinGradlePluginID. Declarations
// marked `apply false` are skipped — they are not applied in this script.
func collectPluginsBlockIDsAST(file *scanner.File, pluginsCall uint32, declared map[string]gradlePluginOccurrence) {
	lambda := flatCallTrailingLambda(file, pluginsCall)
	if lambda == 0 {
		return
	}
	file.FlatWalkNodes(lambda, "call_expression", func(idx uint32) {
		var id string
		switch flatCallExpressionName(file, idx) {
		case "id":
			id = pluginCallStringArgAST(file, idx)
		case "kotlin":
			id = kotlinGradlePluginID(pluginCallStringArgAST(file, idx))
		default:
			return
		}
		if id == "" || pluginDeclarationIsApplyFalseAST(file, idx) {
			return
		}
		if _, ok := declared[id]; !ok {
			declared[id] = gradlePluginOccurrence{id: id, line: file.FlatRow(idx) + 1}
		}
	})
}

// pluginCallStringArgAST returns the first positional string-literal argument
// of a plugin call (`id("x")`, `kotlin("jvm")`), or "" when the argument is
// absent or interpolated.
func pluginCallStringArgAST(file *scanner.File, call uint32) string {
	arg := flatPositionalValueArgument(file, flatCallKeyArguments(file, call), 0)
	return flatStringLiteralExprContent(file, flatValueArgumentExpression(file, arg))
}

// applyPluginIDAST returns the plugin id of an `apply(plugin = "x")` call, or
// false when the call is not that form.
func applyPluginIDAST(file *scanner.File, call uint32) (string, bool) {
	arg := flatNamedValueArgument(file, flatCallKeyArguments(file, call), "plugin")
	id := flatStringLiteralExprContent(file, flatValueArgumentExpression(file, arg))
	return id, id != ""
}

// pluginDeclarationIsApplyFalseAST reports whether a plugin declaration call
// is the left operand of an `... apply false` infix expression, which marks
// the plugin as declared-but-not-applied in this script.
func pluginDeclarationIsApplyFalseAST(file *scanner.File, call uint32) bool {
	for node := call; node != 0; {
		parent, ok := file.FlatParent(node)
		if !ok {
			return false
		}
		switch file.FlatType(parent) {
		case "infix_expression":
			if infixIsApplyFalse(file, parent) {
				return true
			}
		case "statements", "lambda_literal":
			return false
		}
		node = parent
	}
	return false
}

func infixIsApplyFalse(file *scanner.File, infix uint32) bool {
	if !infixOperatorIs(file, infix, "apply") {
		return false
	}
	for child := file.FlatFirstChild(infix); child != 0; child = file.FlatNextSib(child) {
		if isBooleanLiteralFalse(file, child) {
			return true
		}
	}
	return false
}

func gradlePluginsBlockStarts(line string) bool {
	compact := strings.Join(strings.Fields(line), "")
	return strings.Contains(compact, "plugins{")
}

func gradlePluginBlockOccurrences(line string, lineNo int) []gradlePluginOccurrence {
	if strings.Contains(line, "apply false") {
		return nil
	}
	var occurrences []gradlePluginOccurrence
	for _, match := range gradlePluginIDCallRe.FindAllStringSubmatch(line, -1) {
		occurrences = append(occurrences, gradlePluginOccurrence{id: match[1], line: lineNo})
	}
	for _, match := range gradlePluginIDGroovyRe.FindAllStringSubmatch(line, -1) {
		occurrences = append(occurrences, gradlePluginOccurrence{id: match[1], line: lineNo})
	}
	for _, match := range gradleKotlinPluginCallRe.FindAllStringSubmatch(line, -1) {
		if id := kotlinGradlePluginID(match[1]); id != "" {
			occurrences = append(occurrences, gradlePluginOccurrence{id: id, line: lineNo})
		}
	}
	return occurrences
}

func gradleApplyPluginOccurrences(line string, lineNo int) []gradlePluginOccurrence {
	if !strings.HasPrefix(strings.TrimSpace(line), "apply") {
		return nil
	}
	var occurrences []gradlePluginOccurrence
	for _, match := range gradleApplyPluginKtsRe.FindAllStringSubmatch(line, -1) {
		occurrences = append(occurrences, gradlePluginOccurrence{id: match[1], line: lineNo})
	}
	for _, match := range gradleApplyPluginGroovyRe.FindAllStringSubmatch(line, -1) {
		occurrences = append(occurrences, gradlePluginOccurrence{id: match[1], line: lineNo})
	}
	return occurrences
}

func kotlinGradlePluginID(name string) string {
	switch strings.TrimSpace(name) {
	case "android":
		return "org.jetbrains.kotlin.android"
	case "jvm":
		return "org.jetbrains.kotlin.jvm"
	case "multiplatform":
		return "org.jetbrains.kotlin.multiplatform"
	case "kapt":
		return "org.jetbrains.kotlin.kapt"
	case "plugin.serialization", "serialization":
		return "org.jetbrains.kotlin.plugin.serialization"
	default:
		return ""
	}
}

type ConfigurationsAllSideEffectRule struct {
	GradleBase
	BaseRule

	AllowInConventionPlugins bool
}

func (r *ConfigurationsAllSideEffectRule) Confidence() float64 { return api.ConfidenceMediumLowPlus }

func (r *ConfigurationsAllSideEffectRule) check(ctx *api.Context) {
	path, content := ctx.GradlePath, ctx.GradleContent
	if !isGradleRepositoryScript(path) {
		return
	}
	if r.AllowInConventionPlugins && isConventionPluginBuildPath(path) {
		return
	}
	var blocks []configurationsAllSideEffect
	if astFile := ctx.GradleAST(); astFile != nil {
		blocks = findConfigurationsAllSideEffectsAST(astFile)
	} else {
		blocks = findConfigurationsAllSideEffects(content)
	}
	for _, block := range blocks {
		ctx.Emit(scanner.Finding{
			File:       path,
			Line:       block.line,
			Col:        1,
			RuleSet:    r.RuleSetName,
			Rule:       r.RuleName,
			Severity:   r.Sev,
			Message:    fmt.Sprintf("configurations.all mutates dependency resolution via %s. Prefer configuring a named configuration or a convention plugin.", block.mutator),
			Confidence: r.Confidence(),
		})
	}
}

type configurationsAllSideEffect struct {
	line    int
	mutator string
}

func isConventionPluginBuildPath(path string) bool {
	normalized := filepath.ToSlash(path)
	return strings.HasPrefix(normalized, "build-logic/") ||
		strings.HasPrefix(normalized, "buildSrc/") ||
		strings.Contains(normalized, "/build-logic/") ||
		strings.Contains(normalized, "/buildSrc/")
}

func findConfigurationsAllSideEffects(content string) []configurationsAllSideEffect {
	var findings []configurationsAllSideEffect
	depth := 0
	inBlock := false
	blockDepth := 0
	blockLine := 0
	blockMutator := ""

	for i, rawLine := range strings.Split(content, "\n") {
		code := gradleStripStringsAndComments(rawLine)
		if !inBlock && configurationsAllBlockStarts(code) {
			inBlock = true
			blockDepth = depth
			blockLine = i + 1
			blockMutator = ""
		}
		if inBlock && blockMutator == "" {
			blockMutator = configurationsAllSideEffectMutator(code)
		}

		openBraces, closeBraces := countGradleBraces(code)
		depth += openBraces - closeBraces
		if depth < 0 {
			depth = 0
		}
		if inBlock && depth <= blockDepth {
			if blockMutator != "" {
				findings = append(findings, configurationsAllSideEffect{
					line:    blockLine,
					mutator: blockMutator,
				})
			}
			inBlock = false
		}
	}
	return findings
}

func configurationsAllBlockStarts(line string) bool {
	compact := strings.Join(strings.Fields(line), "")
	return strings.Contains(compact, "configurations.all{") ||
		strings.Contains(compact, "configurations.all({")
}

func configurationsAllSideEffectMutator(line string) string {
	compact := strings.Join(strings.Fields(line), "")
	switch {
	case strings.Contains(compact, "resolutionStrategy{") ||
		strings.Contains(compact, "resolutionStrategy.") ||
		strings.Contains(compact, "resolutionStrategy("):
		return "resolutionStrategy"
	case strings.Contains(compact, "dependencies.add("):
		return "dependencies.add"
	case strings.Contains(compact, ".exclude(") || strings.HasPrefix(compact, "exclude("):
		return "exclude"
	case strings.Contains(compact, ".force(") || strings.HasPrefix(compact, "force("):
		return "force"
	default:
		return ""
	}
}

// findConfigurationsAllSideEffectsAST is the tree-sitter equivalent of
// findConfigurationsAllSideEffects for Kotlin DSL (.kts) scripts. It matches a
// `configurations.all { }` call by parser structure — the callee is `all`
// invoked directly on the bare `configurations` receiver — so the
// `configurations.matching { }.all { }` form (whose receiver is a call, not the
// identifier) is correctly excluded, and string/comment lookalikes never parse
// as calls.
func findConfigurationsAllSideEffectsAST(file *scanner.File) []configurationsAllSideEffect {
	var findings []configurationsAllSideEffect
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if flatCallExpressionName(file, idx) != "all" || flatReceiverNameFromCall(file, idx) != "configurations" {
			return
		}
		lambda := flatCallTrailingLambda(file, idx)
		if lambda == 0 {
			return
		}
		if mutator := configurationsAllMutatorAST(file, lambda); mutator != "" {
			findings = append(findings, configurationsAllSideEffect{
				line:    file.FlatRow(idx) + 1,
				mutator: mutator,
			})
		}
	})
	return findings
}

// configurationsAllMutatorAST reports the dependency-resolution mutator invoked
// inside a configurations.all lambda, or "" when the block only reads state.
// A bare reference such as `println(resolutionStrategy)` is not a call on the
// mutator and is correctly ignored. Priority mirrors the line scanner:
// resolutionStrategy > dependencies.add > exclude > force.
func configurationsAllMutatorAST(file *scanner.File, lambda uint32) string {
	var hasRS, hasDepAdd, hasExclude, hasForce bool
	file.FlatWalkNodes(lambda, "call_expression", func(call uint32) {
		name := flatCallExpressionName(file, call)
		recv := flatReceiverNameFromCall(file, call)
		switch {
		case name == "resolutionStrategy" || recv == "resolutionStrategy":
			hasRS = true
		case name == "add" && recv == "dependencies":
			hasDepAdd = true
		case name == "exclude":
			hasExclude = true
		case name == "force":
			hasForce = true
		}
	})
	switch {
	case hasRS:
		return "resolutionStrategy"
	case hasDepAdd:
		return "dependencies.add"
	case hasExclude:
		return "exclude"
	case hasForce:
		return "force"
	default:
		return ""
	}
}
