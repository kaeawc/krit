---
name: krit-rule-vetting
description: Use when auditing Krit static-analysis rules for false positives, missing project context, library lookalikes, config mismatches, Java/Kotlin coverage gaps, or comparing findings against real Android/Kotlin projects such as Signal-Android. Covers both rule precision and Gradle/version-catalog false positives.
---

# Krit Rule Vetting

Use this workflow before trusting large finding counts or broadening a rule.

## Recent Rule Bug Patterns To Guard Against

Recent fixes in PRs #646-#671 and Java parity PRs #676-#680 exposed a few repeat failure modes. Check these before proposing or approving a rule implementation change:

- **Lexical confusion from comments and strings**: raw file scans, `strings.Contains`, and simple prefix checks repeatedly flagged text inside `//`, `/* ... */`, KDoc, regular strings, raw triple-quoted strings, and Gradle string literals. Prefer tree-sitter node dispatch. If a line scanner is unavoidable, add a small lexical state helper and tests for line comments, block comments, trailing comments, regular strings with escapes, and raw/triple-quoted strings.
- **Substring evidence instead of token evidence**: rule decisions based on `Contains("foo")`, `HasPrefix`, or regexes without word boundaries caused false positives such as `debugger`, text URLs, local lookalikes, and unrelated identifiers. Match AST identifiers, navigation chains, import paths, or anchored regexes with explicit token boundaries.
- **Missing receiver or owner proof**: call-name matches are not enough for APIs with common method names. Require structural receiver evidence for cases like `System.out/err`, Android `Context`/`Activity`/`Service`, ignored-return fallback calls, lifecycle methods, database APIs, and local lookalikes.
- **Nested scope leakage**: outer-rule decisions must stop at nested `function_declaration`, `lambda_literal`, `anonymous_function`, class/object boundaries, or local declarations when the inner scope can shadow the symbol or add unrelated complexity.
- **Incomplete traversal**: several false negatives came from checking only the first operand/sibling or stopping at the first non-matching ancestor. Walk all relevant operands/siblings, and continue ancestor walks until a real boundary is reached.
- **Parser-shape drift**: Kotlin AST node names change by construct and parser version. Verify the actual flat AST shape for important operators (`x!!`, Elvis, safe calls, raw strings, infix expressions) instead of assuming the old node type.
- **Java parity gaps**: when a rule claims Java support, verify Java files are parsed, dispatched, indexed, suppressed, and covered by Java positive and Java local-lookalike negative tests. Java object creation, method invocations, annotations, constructors, records, fields, and package/FQN ownership need explicit evidence.
- **Config/runtime mismatch**: schema metadata, validation, and runtime matching must agree. Regex options should be validated as regexes using the same anchoring semantics used at runtime.

For any bug fix matching one of these patterns, add the regression test that would have failed before the fix. Do not rely only on broad positive/negative fixtures.

## Start With Evidence

Run the target repo with JSON and, for performance-sensitive work, per-rule stats:

```bash
go build -o krit ./cmd/krit/
./krit -no-cache -perf -perf-rules -f json -q \
  -o /tmp/krit_target.json \
  /path/to/project || true
```

Count and sample the rule:

```bash
jq -r '.findings[].rule' /tmp/krit_target.json | sort | uniq -c | sort -nr | head -40
jq -r '.findings[] | select(.rule=="RuleName") | [.file,.line,.message] | @tsv' /tmp/krit_target.json | head -80
jq -r '.perfRuleStats[] | select(.rule=="RuleName")' /tmp/krit_target.json
```

Before judging findings, verify project config is applied. If uncertain, pass `--config /path/to/project/krit.yml`.

## Check Project Profile Before Vetting

High-volume rules are often inflated by missing project context. Before vetting rule logic, check what Krit knows about the project:

```bash
jq '.projectProfile | {hasGradle, dependencyExtractionComplete, hasUnresolvedDependencyRefs, catalogCompleteness}' /tmp/krit_target.json
```

If `hasGradle` is false or `dependencyExtractionComplete` is false, rules that gate on library presence will use conservative defaults (assume library is present). This is intentional and correct — treat library-absence-based suppressions as unreliable until the profile is complete.

For rules that fire heavily on a project that may not use the relevant library (Room, Compose, Hilt, MockK, etc.):

```bash
jq '.projectProfile.dependencies[] | select(.group | test("room|compose|hilt|mockk|retrofit"))' /tmp/krit_target.json
```

## Top-Volume Rules by Category (Real-Project Baseline)

From scanning Signal-Android, nowinandroid, dd-sdk-android, firebase-android-sdk, and kaeawc projects:

**Testing (highest-volume, most actionable):**
- `TestWithoutAssertion` (1149) — check for custom assertion helpers that don't pattern-match
- `MockWithoutVerify` (862) — check for verify-equivalent patterns like `coVerify`, `inOrder`, slot capture
- `RunBlockingInTest` (554) — only a real issue when coroutines-test is present; check `runTest` migration
- `AssertTrueOnComparison` (322) — straightforward; `assertTrue(a == b)` should be `assertEquals`

**Safety / Nullability:**
- `UnsafeCallOnNullableType` (262) — high precision; sample for platform types and already-guarded branches
- `SwallowedException` (189) — check for intentional swallowing with logging (not a false positive)
- `AssertNullableWithNotNullAssertion` (103) — legitimate but check test-only contexts

**Code style (noisy, often config-controlled):**
- `MagicNumber`, `MaxLineLength`, `MaxChainedCallsOnSameLine` — check project `krit.yml` thresholds before vetting
- `ModuleDeadCode` (15578 in kaeawc) — requires full module graph; confirm `NeedsModuleIndex` is satisfied

**Compose-specific:**
- `ComposeRawTextLiteral`, `ComposeUnstableParameter`, `ComposePainterResourceInLoop` — only meaningful in Compose modules; check library model profile

**Android resources:**
- `LayoutMinTouchTargetInButtonRow`, `NegativeMarginResource`, `DisableBaselineAlignmentResource` — high signal; few false positives

## Vet A Rule

For each candidate rule:

1. Read the registry entry and implementation.
2. Identify its declared capabilities: `NodeTypes`, `Languages`, `Needs*`, `TypeInfo`, `Oracle*`.
3. Inspect existing fixtures and tests.
4. Inspect the actual flat AST for the construct when the fix depends on node shape. Prefer a focused unit test that parses a temporary Kotlin/Java snippet with `scanner.ParseFile` or `scanner.ParseJavaFile`, walks the relevant flat nodes, and asserts the node kind/text/parent chain the rule depends on.
5. Sample real findings in the target repo with surrounding code.
6. Decide whether the rule has enough evidence:
   - real library import/FQN, not just a local class with the same name
   - correct language coverage for Kotlin and Java if the rule claims both
   - correct owner/lifecycle/interface context when names are overloaded
   - correct async/background/event-boundary handling
   - correct project configuration and suppression behavior
   - correct lexical handling for comments, block comments, raw strings, regular strings, Gradle strings, and generated code
   - correct scope boundary handling for lambdas, local functions, anonymous functions, nested classes, and shadowed names
7. Add focused positive and negative tests before changing behavior.
8. Rerun the target repo and compare counts before/after.

## Common False-Positive Patterns

- Local lookalike APIs: `Room`, `Retrofit`, `OkHttpClient`, `HttpClient`, `Dispatchers`, etc.
- Event callbacks mistaken for immediate execution, especially Compose `onX = { ... }` lambdas.
- Lifecycle-name matching without verifying the owner is an Android lifecycle type.
- Main-thread rules that ignore background boundaries such as `Dispatchers.IO`, Rx `subscribeOn`, `SimpleTask.run`, executors, or worker annotations.
- Rules that inspect all node candidates and repeatedly scan whole-file text.
- Project conventions enforced despite config or `.editorconfig` saying otherwise.
- Regex/prefix matches without a trailing token boundary (`debug` matching `debugger`, `test` matching `testing`, etc.).
- Gradle/source helpers that strip only whole-line comments and miss trailing comments or strings.
- Import scanners that stop at the first non-import-looking line instead of threading block-comment state.
- Ancestor walks that stop at the first unrelated call expression instead of continuing to a semantic boundary.
- Operand scans that inspect only the left or first child of an infix/when/condition tree.

## Performance Check

If a precision fix adds source text checks, cache per-file facts instead of rescanning content for every candidate node. Confirm with:

```bash
./krit -no-cache -no-type-oracle -perf -perf-rules -f json -q \
  -o /tmp/krit_perf_after.json /path/to/project || true
jq -r '.perfRuleStats[:30][] | [.rule,.invocations,.durationMs,.avgNs,.sharePct] | @tsv' /tmp/krit_perf_after.json
```

Do not accept a precision fix that moves a rule into the top cost list unless the cost is justified.

## Fixture Standard

Every rule-vetting fix should usually add:

- one positive fixture or test that proves the intended real API/use case
- one negative local-lookalike fixture
- one negative framework-boundary fixture if async, lifecycle, callback, or Compose behavior is involved
- one negative lexical fixture if the rule reads text: line comment, block comment, raw string, regular string, and trailing comment where relevant
- one negative nested-scope fixture if the rule walks bodies or symbol references: lambda, local function, anonymous function, and shadowed declaration where relevant
- Java positive and Java negative fixtures when `Languages` includes Java or the rule is advertised as Java-aware
- one real-project-inspired regression test when a false positive came from Signal or another target

Prefer unit tests for small helpers that implement lexical state, navigation-chain matching, owner evidence, config validation, or cross-file/index lookup. Fixtures prove end-to-end behavior; helper tests make the exact historical bug hard to reintroduce.

## Message Standard

Finding messages should state:

- what specific call/path is risky
- what evidence made the rule believe it runs in that context
- what to move/change
- where not to move code when lifecycle/UI code must remain on the main thread

Avoid vague messages like "move this method off the main thread" when only part of the method is unsafe.
