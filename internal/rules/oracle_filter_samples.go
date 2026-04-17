package rules

// This file contains the audited OracleFilter classifications for the
// initial sample of 20 krit rules.
//
// Classification strategy (see tools/krit-types/benchmarks/rule-classification-sample.md
// for the full audit rationale per rule):
//
//   - TREE_SITTER_ONLY: an empty OracleFilter{} — the rule never calls
//     into the oracle, even transitively via the composite resolver.
//   - ORACLE_FILTERED: an OracleFilter with a small set of byte-substring
//     identifiers. The rule's oracle code path is gated behind an AST or
//     content check that is only reachable when at least one of the listed
//     identifiers is present in the file. If the file doesn't contain any
//     of them, the rule still runs (tree-sitter pass) but never calls the
//     oracle.
//   - ORACLE_ALL_FILES: OracleFilter{AllFiles: true} — the rule walks every
//     file broadly and cannot be narrowed without losing findings.
//
// Rules not listed here fall through to the conservative default
// (AllFiles: true) from GetOracleFilter so the 220 un-audited rules keep
// their current behavior.
//
// A rule that wants to opt in wires this via a one-line OracleFilter()
// method referencing one of the precomputed filter values below. The
// classifications are the audit output on a Signal-Android sample and
// MUST be validated by the Phase 6 findings-equivalence gate before
// being trusted for a prod run.

// Shared filter singletons. All of these are value types so the pointers
// are safe to reuse across rule instances.
var (
	// treeSitterOnlyFilter means "never calls the oracle, not even
	// transitively through CompositeResolver". The rule's Check path is
	// pure AST/byte inspection.
	treeSitterOnlyFilter = &OracleFilter{}

	// coroutinesRedundantSuspendFilter gates RedundantSuspendModifierRule
	// on files that declare a `suspend` modifier somewhere. Files without
	// the `suspend` keyword can never produce a finding (the rule's first
	// guard is hasSuspendModifierFlat) and therefore never call
	// oracleLookup.LookupCallTarget.
	coroutinesRedundantSuspendFilter = &OracleFilter{
		Identifiers: []string{"suspend"},
	}

	// nullSafetyAsExpressionFilter gates null-safety cast rules on files
	// that contain an `as` or `as?` cast. Tree-sitter's as_expression
	// always has the literal ` as ` substring in source (the cast target
	// must follow the operator). Files without this substring never emit
	// the AST node the rule dispatches on, so the oracle is never
	// consulted on those files.
	nullSafetyAsExpressionFilter = &OracleFilter{
		Identifiers: []string{" as "},
	}

	// nullSafetyNotNullOperatorFilter gates UnnecessaryNotNullOperator on
	// files that contain the `!!` operator. A file without `!!` cannot
	// produce an unnecessary not-null finding and therefore never calls
	// the resolver on a !! node.
	nullSafetyNotNullOperatorFilter = &OracleFilter{
		Identifiers: []string{"!!"},
	}

	// allFilesAuditedFilter marks rules that have been audited and
	// confirmed to need the oracle on every file (flow-sensitive or
	// broad call-graph walks).
	allFilesAuditedFilter = &OracleFilter{AllFiles: true}
)

// --- coroutines ---

// CollectInOnCreateWithoutLifecycleRule: pure tree-sitter.
// Walks call_expression nodes for "collect" inside lifecycle methods.
// Oracle methods used: none.
func (r *CollectInOnCreateWithoutLifecycleRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// GlobalCoroutineUsageRule: pure tree-sitter.
// Matches on literal text "GlobalScope" — no resolver calls.
func (r *GlobalCoroutineUsageRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// InjectDispatcherRule: pure tree-sitter.
// Matches Dispatchers.IO/Default/Unconfined by text, never calls resolver.
func (r *InjectDispatcherRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// RedundantSuspendModifierRule: ORACLE_FILTERED on "suspend".
// Early returns at hasSuspendModifierFlat — files without `suspend`
// never reach the oracleLookup.LookupCallTarget branch.
func (r *RedundantSuspendModifierRule) OracleFilter() *OracleFilter {
	return coroutinesRedundantSuspendFilter
}

// --- security ---

// ContentProviderQueryWithSelectionInterpolationRule: pure tree-sitter.
// Matches ContentResolver.query(...) by text-based heuristics.
func (r *ContentProviderQueryWithSelectionInterpolationRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// HardcodedBearerTokenRule: pure tree-sitter.
// Regexes the raw string literal bytes.
func (r *HardcodedBearerTokenRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// --- null-safety / potential-bugs ---

// UnsafeCastRule: ORACLE_FILTERED on " as ".
// Rule dispatches on `as_expression` nodes; a file that produces such a
// node must contain the ` as ` substring in source. Confirmed by tree-
// sitter kotlin grammar. Inside the rule the resolver is called via
// r.resolver.ResolveFlatNode, which in composite mode routes to the
// oracle — so we keep the oracle available for files with casts.
func (r *UnsafeCastRule) OracleFilter() *OracleFilter {
	return nullSafetyAsExpressionFilter
}

// UnnecessaryNotNullOperatorRule: ORACLE_FILTERED on "!!".
// Rule fires on the `!!` node; files without the operator never produce
// a finding and never query the resolver.
func (r *UnnecessaryNotNullOperatorRule) OracleFilter() *OracleFilter {
	return nullSafetyNotNullOperatorFilter
}

// --- complexity ---

// LongMethodRule: pure tree-sitter. Counts body lines.
func (r *LongMethodRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// CyclomaticComplexMethodRule: pure tree-sitter. Walks AST for decision nodes.
func (r *CyclomaticComplexMethodRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// --- naming ---

// ClassNamingRule: pure tree-sitter. Regex on extracted identifier.
func (r *ClassNamingRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// FunctionNamingRule: pure tree-sitter. Regex on extracted identifier.
func (r *FunctionNamingRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// --- accessibility / a11y ---

// AnimatorDurationIgnoresScaleRule: pure tree-sitter. Text heuristics.
func (r *AnimatorDurationIgnoresScaleRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// ComposeClickableWithoutMinTouchTargetRule: pure tree-sitter. AST walk only.
func (r *ComposeClickableWithoutMinTouchTargetRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// --- di-hygiene ---

// AnvilMergeComponentEmptyScopeRule: pure tree-sitter. Aggregates modifier text.
func (r *AnvilMergeComponentEmptyScopeRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// BindsMismatchedArityRule: pure tree-sitter. Counts parameters of @Binds.
func (r *BindsMismatchedArityRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// --- empty-blocks ---

// EmptyCatchBlockRule: pure tree-sitter. Checks catch body is empty.
func (r *EmptyCatchBlockRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// --- testing-quality ---

// AssertEqualsArgumentOrderRule: pure tree-sitter. Text match on "assertEquals".
func (r *AssertEqualsArgumentOrderRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// MixedAssertionLibrariesRule: pure tree-sitter. Scans import lines.
func (r *MixedAssertionLibrariesRule) OracleFilter() *OracleFilter {
	return treeSitterOnlyFilter
}

// --- potential-bugs (broad oracle) ---

// DeprecationRule: ORACLE_ALL_FILES. Walks every call_expression,
// navigation_expression, and user_type, and always consults
// LookupCallTarget/LookupAnnotations. Cannot be narrowed without losing
// deprecation findings on any file that imports a deprecated library
// symbol, which may appear in any .kt file in the corpus.
func (r *DeprecationRule) OracleFilter() *OracleFilter {
	return allFilesAuditedFilter
}
