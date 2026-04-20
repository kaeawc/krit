package rules

//go:generate go run ../codegen/cmd/krit-gen -inventory ../../build/rule_inventory.json -out . -root ../..

import (
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// Oracle-need declaration moved to the v2.Capabilities bitfield
// (v2.NeedsOracle) and v2.Rule.Oracle. Rules that do not declare
// NeedsOracle are never fed to the oracle's file-selection pass, so
// the old opt-out default (AllFiles: true for unaudited rules) is
// gone. See roadmap/clusters/core-infra/oracle-filter-inversion.md.
//
// Rules still carry a per-rule base confidence via the
// `Confidence() float64` method. ConfidenceOf type-asserts to an
// anonymous interface at the call site.
//
// Tier conventions for Confidence():
//
//   - Tier 1 (high, 0.95): AST structural checks with zero known false
//     positives. Safe for CI enforcement.
//   - Tier 2 (medium, 0.75): heuristic or pattern match with documented
//     edge cases, or rules that lose accuracy without type inference.
//   - Tier 3 (low, 0.50): confirmed false-positive patterns or
//     dependencies on analysis infrastructure the rule does not yet
//     consume.
//
// A rule's Confidence() is treated as the *base* value — individual
// findings may still override by setting Finding.Confidence directly
// (e.g. a rule that is usually high-confidence but drops to medium on
// a specific edge-case branch). The base is applied only to findings
// that leave Confidence unset (zero).

// BaseRule provides common fields embedded in every rule implementation.
// It carries the canonical name/ruleset/severity/description metadata that
// the codegen (krit-gen) reads when emitting zz_registry_gen.go's
// v2.Register(&FooRule{BaseRule: BaseRule{...}}) literals, and it provides
// the Finding() helper rules use to construct emit-boundary
// scanner.Finding values.
type BaseRule struct {
	RuleName    string
	RuleSetName string
	Sev         string
	Desc        string
}

// ruleExcludes stores per-rule file exclusion glob patterns, keyed by rule name.
// Populated by ApplyConfig from the YAML "excludes" field.
var ruleExcludes = make(map[string][]string)

func (b BaseRule) Name() string        { return b.RuleName }
func (b BaseRule) Description() string { return b.Desc }
func (b BaseRule) RuleSet() string     { return b.RuleSetName }
func (b BaseRule) Severity() string    { return b.Sev }

// SetRuleExcludes sets glob patterns for file exclusion on a rule by name.
func SetRuleExcludes(ruleName string, patterns []string) {
	ruleExcludes[ruleName] = patterns
}

// GetRuleExcludes returns the glob patterns for file exclusion for a rule.
func GetRuleExcludes(ruleName string) []string {
	return ruleExcludes[ruleName]
}

// GetAllRuleExcludes returns a snapshot of every rule's exclude globs,
// omitting rules with an empty pattern list. The pipeline Parse phase
// passes this into scanner.BuildSuppressionFilter so the exclude globs
// live on each file's filter rather than being reconsulted per
// rule/file combination by the dispatcher.
func GetAllRuleExcludes() map[string][]string {
	out := make(map[string][]string, len(ruleExcludes))
	for k, v := range ruleExcludes {
		if len(v) == 0 {
			continue
		}
		out[k] = v
	}
	return out
}

// IsFileExcluded checks whether a file path matches any of the rule's exclude patterns.
func IsFileExcluded(filePath string, excludes []string) bool {
	for _, pattern := range excludes {
		if matchExcludePattern(filePath, pattern) {
			return true
		}
	}
	return false
}

// matchExcludePattern matches a file path against a detekt-style glob pattern.
// Supports ** (match any path segments), e.g.:
//   - **/test/**    -> path contains /test/
//   - **/*Test.kt   -> path ends with Test.kt
//   - **/*Spec.kt   -> path ends with Spec.kt
//   - *.kt          -> filename matches *.kt
func matchExcludePattern(filePath, pattern string) bool {
	// Normalize separators
	filePath = filepath.ToSlash(filePath)
	pattern = filepath.ToSlash(pattern)

	// Handle ** prefix patterns
	if strings.HasPrefix(pattern, "**/") {
		suffix := pattern[3:]
		if strings.Contains(suffix, "/**") {
			// e.g., **/test/** — check if path contains /test/
			inner := strings.TrimSuffix(suffix, "/**")
			return strings.Contains(filePath, "/"+inner+"/")
		}
		if strings.HasPrefix(suffix, "*") {
			// e.g., **/*Test.kt — check if path ends with the suffix after *
			return strings.HasSuffix(filePath, suffix[1:])
		}
		// e.g., **/Foo.kt — match exact filename anywhere in path
		return strings.HasSuffix(filePath, "/"+suffix) || filePath == suffix
	}

	// Plain glob against the basename
	matched, _ := filepath.Match(pattern, filepath.Base(filePath))
	return matched
}

// FlatDispatchBase is embedded by flat-dispatch rule implementations.
type FlatDispatchBase struct{}

// LineBase is embedded by line-rule implementations.
type LineBase struct{}

func (b BaseRule) Finding(file *scanner.File, line, col int, msg string) scanner.Finding {
	return scanner.Finding{
		File:     file.Path,
		Line:     line,
		Col:      col,
		RuleSet:  b.RuleSetName,
		Rule:     b.RuleName,
		Severity: b.Sev,
		Message:  msg,
	}
}
