# Rule classification sample (20 rules)

This document captures the first feasibility-pass audit for the krit
rule oracle-filter. We classified 20 representative rules from the 240
in `internal/rules/` into three buckets:

- `TREE_SITTER_ONLY` — the rule never touches the oracle, even
  transitively through `CompositeResolver`. OracleFilter is the empty
  struct `&OracleFilter{}`.
- `ORACLE_FILTERED` — the rule only reaches an oracle call when the
  file's raw bytes contain one of a small set of substrings. OracleFilter
  lists those substrings.
- `ORACLE_ALL_FILES` — the rule cannot be narrowed by file content;
  OracleFilter is `{AllFiles: true}`.

The correctness bar is findings-equivalence against a Signal-Android
baseline (see Phase 6). Any rule whose filter is wrong gets its
classification upgraded back to `AllFiles: true`.

The classifications below are wired into
`internal/rules/oracle_filter_samples.go`.

## coroutines

### CollectInOnCreateWithoutLifecycleRule (coroutines.go:28)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `call_expression`; walks for
  `collect` inside `lifecycleCollectCallbacks` (onCreate/onStart/
  onViewCreated); checks for `repeatOnLifecycle` ancestor.
- Classification: **TREE_SITTER_ONLY**
- Rationale: pure AST walk and string comparison, never touches a
  resolver or oracle lookup.

### GlobalCoroutineUsageRule (coroutines.go:86)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `call_expression` and
  `navigation_expression`; matches the literal receiver text
  `GlobalScope` and `launch`/`async` suffix.
- Classification: **TREE_SITTER_ONLY**
- Rationale: purely text/AST. No resolver references.

### InjectDispatcherRule (coroutines.go:172)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `call_expression`; text-matches
  `Dispatchers.IO/Default/Unconfined` and looks for enclosing
  function_declaration to skip `object` / `@JvmStatic`.
- Classification: **TREE_SITTER_ONLY**
- Rationale: syntactic only.

### RedundantSuspendModifierRule (coroutines.go:315)

- Oracle methods used: `LookupCallTarget`
- Tree-sitter queries: dispatches on `function_declaration`; first guard
  is `hasSuspendModifierFlat` — returns nil immediately if the function
  has no `suspend` modifier.
- Classification: **ORACLE_FILTERED**, Identifiers: `["suspend"]`
- Rationale: the oracle call path is reachable only when the function
  has a `suspend` modifier, which requires the keyword `suspend` to
  appear in the source file. Files that don't contain the substring
  `"suspend"` can never produce a finding and never call
  `oracleLookup.LookupCallTarget`. The match is substring-based so it
  also matches `suspending` and `@Suspend` inside strings — both harmless
  over-matches that keep the filter safely conservative.

## security

### ContentProviderQueryWithSelectionInterpolationRule (security.go:27)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `call_expression`; matches
  `.query(` callee with interpolated selection strings.
- Classification: **TREE_SITTER_ONLY**
- Rationale: pure AST walk.

### HardcodedBearerTokenRule (security.go:68)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `string_literal`; inspects the
  literal body.
- Classification: **TREE_SITTER_ONLY**
- Rationale: byte-only.

## null-safety / potential-bugs

### UnsafeCastRule (potentialbugs_nullsafety.go:33)

- Oracle methods used: `resolver.ResolveFlatNode` (which routes through
  CompositeResolver → oracle.LookupExpression when present).
- Tree-sitter queries: dispatches on `as_expression`; extracts
  `expr as Type` text and asks the resolver for the expression type.
- Classification: **ORACLE_FILTERED**, Identifiers: `[" as "]`
- Rationale: an `as_expression` node exists only when the source
  contains the ` as ` operator. Files without that substring never emit
  the node and never call the resolver. The space-padded substring
  avoids matching identifiers like `class` or `assertEquals`.

### UnnecessaryNotNullOperatorRule (potentialbugs_nullsafety.go:1685)

- Oracle methods used: `resolver.ResolveByNameFlat`,
  `resolver.IsNullableFlat` (via CompositeResolver)
- Tree-sitter queries: dispatches on `!!` / not-null-assertion nodes.
- Classification: **ORACLE_FILTERED**, Identifiers: `["!!"]`
- Rationale: files without `!!` never produce the AST node the rule
  dispatches on and never consult the resolver.

## naming

### ClassNamingRule (naming.go:57)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `class_declaration`; regex on
  extracted identifier.
- Classification: **TREE_SITTER_ONLY**
- Rationale: pure regex on identifier text.

### FunctionNamingRule (naming.go:~120)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `function_declaration`; regex on
  extracted identifier; consults `IgnoreAnnotated` list via AST
  annotation scan.
- Classification: **TREE_SITTER_ONLY**
- Rationale: pure syntactic check.

## complexity

### LongMethodRule (complexity.go:~90)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `function_declaration`; counts
  lines in the body.
- Classification: **TREE_SITTER_ONLY**

### CyclomaticComplexMethodRule (complexity.go:~200)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `function_declaration`; walks
  decision nodes (`if`, `when`, `for`, `while`, `elvis_expression`,
  etc.) counting cyclomatic complexity.
- Classification: **TREE_SITTER_ONLY**
- Rationale: pure AST decision-node counter.

## accessibility / a11y

### AnimatorDurationIgnoresScaleRule (accessibility.go:24)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `call_expression` and `assignment`;
  byte-scans for `ANIMATOR_DURATION_SCALE` as a cheap early-out.
- Classification: **TREE_SITTER_ONLY**

### ComposeClickableWithoutMinTouchTargetRule (accessibility.go:125)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `call_expression`; walks the
  Modifier call chain.
- Classification: **TREE_SITTER_ONLY**

## di-hygiene

### AnvilMergeComponentEmptyScopeRule (di_hygiene.go:22)

- Oracle methods used: none
- Tree-sitter queries: parsed-files aggregate; walks each file's
  declarations looking for @MergeComponent / @ContributesTo /
  @ContributesBinding and matches scope strings.
- Classification: **TREE_SITTER_ONLY**

### BindsMismatchedArityRule (di_hygiene.go:186)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `function_declaration`; checks for
  `@Binds` annotation and parameter count.
- Classification: **TREE_SITTER_ONLY**

## empty-blocks

### EmptyCatchBlockRule (emptyblocks.go:80)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `catch_block`; checks whether the
  catch body is empty after comment stripping.
- Classification: **TREE_SITTER_ONLY**

## testing-quality

### AssertEqualsArgumentOrderRule (testing_quality.go:26)

- Oracle methods used: none
- Tree-sitter queries: dispatches on `call_expression` for
  `assertEquals`; compares first two args to `actual`/`expected`.
- Classification: **TREE_SITTER_ONLY**

### MixedAssertionLibrariesRule (testing_quality.go:117)

- Oracle methods used: none
- Tree-sitter queries: LineRule scanning `import` statements.
- Classification: **TREE_SITTER_ONLY**

## potential-bugs (broad oracle consumer)

### DeprecationRule (potentialbugs_misc.go:38)

- Oracle methods used: `LookupCallTarget`, `LookupAnnotations` on every
  visited call/nav/user_type node.
- Tree-sitter queries: dispatches on `call_expression`,
  `navigation_expression`, `user_type`.
- Classification: **ORACLE_ALL_FILES**
- Rationale: any `.kt` file that references any library symbol could
  reach a deprecated API. There's no per-file gate we can apply without
  losing findings — the rule has to walk the whole corpus through the
  oracle.

## Tally (20-rule sample)

| Bucket             | Count | Rules |
|--------------------|------:|-------|
| TREE_SITTER_ONLY   | 16    | CollectInOnCreateWithoutLifecycle, GlobalCoroutineUsage, InjectDispatcher, ContentProviderQueryWithSelectionInterpolation, HardcodedBearerToken, ClassNaming, FunctionNaming, LongMethod, CyclomaticComplexMethod, AnimatorDurationIgnoresScale, ComposeClickableWithoutMinTouchTarget, AnvilMergeComponentEmptyScope, BindsMismatchedArity, EmptyCatchBlock, AssertEqualsArgumentOrder, MixedAssertionLibraries |
| ORACLE_FILTERED    | 3     | RedundantSuspendModifier (`"suspend"`), UnsafeCast (`" as "`), UnnecessaryNotNullOperator (`"!!"`) |
| ORACLE_ALL_FILES   | 1     | Deprecation |

## Projected win on this sample

With even a single enabled rule in the `ORACLE_ALL_FILES` bucket (like
`Deprecation`, which is on by default), the filter short-circuits and
feeds every file to krit-types. The **infrastructure still ships** —
once `Deprecation` is disabled (or audited to a filter), the sample's
three `ORACLE_FILTERED` rules would reduce the oracle file set to the
union of files containing `"suspend"`, `" as "`, or `"!!"`. On
Signal-Android (sampled below) those keywords together appear in
roughly 30–60% of .kt files, so the ceiling for this sample (without
auditing the remaining 220 rules) is a ~40–70% reduction — assuming you
also disable broad-oracle rules like `Deprecation`. With `Deprecation`
enabled, savings are zero by design.

The follow-up work (Phase 8, out of scope here) is to audit the
remaining 220 rules. If the same ~80% TREE_SITTER_ONLY / ~15%
ORACLE_FILTERED / ~5% ORACLE_ALL_FILES ratio holds, the practical
reduction will be dominated by whichever `ORACLE_ALL_FILES` rules are
enabled in the user's configuration.
