package rules

//go:generate go run ../codegen/cmd/krit-gen -inventory ../../build/rule_inventory.json -out . -root ../..

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/scanner"
)

// Rule is the v1 rule interface. Retained as the embedded constraint in
// the Android family type aliases (GradleFamily/ManifestFamily/
// ResourceFamily) and their per-family RegisterX glue. All active
// dispatch is v2-native; these types are scaffolding the Android rule
// files still embed for metadata plumbing.
type Rule interface {
	Name() string
	Description() string
	RuleSet() string
	Severity() string
}

// FixableRule is optionally implemented by rules that can auto-fix findings.
// Rules implementing this return findings with Fix populated.
type FixableRule interface {
	Rule
	IsFixable() bool
}

// OracleFilter declares when a rule needs oracle type information.
//
// The oracle (krit-types.jar) is an expensive whole-corpus analysis step.
// Many rules only query the oracle when a file contains a specific
// keyword or identifier — for those files we can skip oracle analysis
// entirely. OracleFilter lets each rule declare that condition so krit
// can compute the union of "files any enabled rule cares about" and
// pass that subset to the oracle instead of all .kt files.
//
// Semantics:
//
//   - nil OracleFilter on a rule means krit has not classified this rule
//     yet. The safe default applied by GetOracleFilter is AllFiles: true
//     (conservative — always feed the file to the oracle). This preserves
//     correctness for the 220 unaudited rules.
//
//   - AllFiles: true means this rule always wants the oracle. Used by
//     rules that do whole-file or flow-sensitive analysis and cannot be
//     narrowed by a content check.
//
//   - A non-empty Identifiers slice means the rule only needs the oracle
//     when at least one of the listed identifiers appears anywhere in the
//     file's raw bytes (substring match). Use this for rules whose early
//     bail-out is gated on a specific keyword or API name (e.g. "suspend",
//     "as ", "GlobalScope").
//
//   - An empty OracleFilter{} (both Identifiers nil and AllFiles false)
//     means the rule never needs the oracle (tree-sitter only). Use this
//     for purely syntactic rules.
//
// The correctness invariant is: if a file is NOT in the union of filter
// matches, then no enabled rule will query the oracle on that file. A
// rule's filter is CORRECT if and only if every oracle-path code branch
// inside the rule is gated on the presence of one of the declared
// identifiers (or the AllFiles flag).
type OracleFilter struct {
	// Identifiers is a list of raw byte substrings whose presence in the
	// file content indicates this rule may query the oracle. Matching uses
	// bytes.Contains — the substring check is cheap and conservative:
	// false positives waste some oracle work but false negatives lose
	// findings. Keep the substrings distinctive enough to avoid accidental
	// matches (e.g. "runBlocking" rather than "run").
	Identifiers []string

	// AllFiles = true means this rule always needs the oracle, regardless
	// of file content. When any enabled rule sets this, the filter result
	// is the full file set and CollectOracleFiles returns nil to short-
	// circuit filtering (no benefit).
	AllFiles bool
}

// NeverNeedsOracle returns true when a filter declares the rule is purely
// tree-sitter and will never consult the oracle.
func (f *OracleFilter) NeverNeedsOracle() bool {
	if f == nil {
		return false
	}
	return !f.AllFiles && len(f.Identifiers) == 0
}

// Rules that want to declare an oracle filter or base confidence do so
// structurally by adding `OracleFilter() *OracleFilter` or
// `Confidence() float64` methods. GetOracleFilter / ConfidenceOf
// type-assert to anonymous interfaces at the call site.
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

// Registry retained as the write-side of the Android family RegisterX
// glue (GradleRules/ManifestRules/ResourceRules). No production code
// reads from it; runtime dispatch goes through v2.Registry. Kept so the
// Android rule registration path still compiles.
var Registry []Rule

// Register adds a rule to the global registry.
// Panics if the rule has no description — every rule must document what it checks.
func Register(r Rule) {
	if r.Description() == "" {
		panic(fmt.Sprintf("rule %s has no description", r.Name()))
	}
	Registry = append(Registry, r)
}

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
