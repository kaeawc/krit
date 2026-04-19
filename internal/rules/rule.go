package rules

//go:generate go run ../codegen/cmd/krit-gen -inventory ../../build/rule_inventory.json -out . -root ../..

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/scanner"
)

// Rule is the v1 rule interface.
//
// Deprecated: implement rules as native *v2.Rule values registered via
// v2.Register. New rules must use the v2.Context.Emit / v2.Context.EmitAt
// API and never return []scanner.Finding. This interface and all v1 family
// methods (CheckFlatNode, CheckLines, CheckCrossFile, CheckManifest,
// CheckResources, CheckGradle, CheckModuleAware, Finalize) will be deleted
// once all rules are migrated.
type Rule interface {
	Name() string
	Description() string
	RuleSet() string
	Severity() string
	Check(file *scanner.File) []scanner.Finding
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

// ConfidenceOf returns the base confidence for a rule. Rules that do
// not provide a Confidence() method return 0, which tells the dispatcher
// to fall back to the rule-type default.
func ConfidenceOf(r Rule) float64 {
	if cp, ok := r.(interface{ Confidence() float64 }); ok {
		return cp.Confidence()
	}
	return 0
}

// DescriptionOf returns the description for a rule.
func DescriptionOf(r Rule) string {
	return r.Description()
}

// allFilesFilter is the conservative default returned for rules that
// do not expose an OracleFilter() method. Shared so GetOracleFilter does
// not allocate on the hot path.
var allFilesFilter = &OracleFilter{AllFiles: true}

// GetOracleFilter returns the effective OracleFilter for a rule. Rules
// that do not expose an OracleFilter() method default to AllFiles: true
// (conservative — the rule has not been audited so the oracle is always
// run for its files). A provider that returns nil is also treated as the
// conservative default.
func GetOracleFilter(r Rule) *OracleFilter {
	if p, ok := r.(interface{ OracleFilter() *OracleFilter }); ok {
		if f := p.OracleFilter(); f != nil {
			return f
		}
	}
	return allFilesFilter
}

// IsImplemented reports whether a rule has a real implementation path
// behind it — i.e. it satisfies at least one of the concrete
// rule-family method sets (flat dispatch, line, aggregate, cross-file,
// parsed-files, module-aware, manifest, resource, gradle) or declares
// a non-zero Android data dependency (which routes it through the
// Android resource/icon/etc. pipelines rather than the AST dispatcher).
//
// Rules that return false are pure placeholders: their Check() method
// returns nil, they don't implement any of the above, and they don't
// carry any Android data metadata. They inflate the advertised rule
// count without producing findings.
//
// This is the roadmap/17 Phase 6 stub detector — used by --list-rules
// to split the headline count into implemented vs stub.
func IsImplemented(r Rule) bool {
	if _, ok := r.(interface {
		CheckFlatNode(idx uint32, file *scanner.File) []scanner.Finding
	}); ok {
		return true
	}
	if _, ok := r.(interface {
		CheckLines(file *scanner.File) []scanner.Finding
	}); ok {
		return true
	}
	if _, ok := r.(interface {
		AggregateNodeTypes() []string
		CollectFlatNode(idx uint32, file *scanner.File)
		Finalize(file *scanner.File) []scanner.Finding
		Reset()
	}); ok {
		return true
	}
	if _, ok := r.(interface {
		CheckCrossFile(index *scanner.CodeIndex) []scanner.Finding
	}); ok {
		return true
	}
	if _, ok := r.(interface {
		CheckParsedFiles(files []*scanner.File) []scanner.Finding
	}); ok {
		return true
	}
	if _, ok := r.(interface {
		CheckModuleAware() []scanner.Finding
	}); ok {
		return true
	}
	if _, ok := r.(interface {
		CheckManifest(m *Manifest) []scanner.Finding
	}); ok {
		return true
	}
	if _, ok := r.(interface {
		CheckResources(idx *android.ResourceIndex) []scanner.Finding
	}); ok {
		return true
	}
	if _, ok := r.(interface {
		CheckGradle(path string, content string, cfg *android.BuildConfig) []scanner.Finding
	}); ok {
		return true
	}
	if AndroidDependenciesOf(r) != AndroidDepNone {
		return true
	}
	return false
}

// Registry holds all registered rules.
var Registry []Rule

// Register adds a rule to the global registry.
// Panics if the rule has no description — every rule must document what it checks.
func Register(r Rule) {
	if r.Description() == "" {
		panic(fmt.Sprintf("rule %s has no description", r.Name()))
	}
	Registry = append(Registry, r)
}

// BaseRule provides common fields.
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

// FlatDispatchBase provides a default nil implementation for Check.
// Embed in flat-dispatch rule implementations to avoid boilerplate stubs.
type FlatDispatchBase struct{}

func (FlatDispatchBase) Check(file *scanner.File) []scanner.Finding { return nil }

// LineBase provides a default nil implementation for Check.
// Embed in line-rule implementations to avoid boilerplate stubs.
type LineBase struct{}

func (LineBase) Check(file *scanner.File) []scanner.Finding { return nil }

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
