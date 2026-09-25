# Compiler-backed rule candidates

This page tracks which Krit rules should be backed by the Kotlin compiler, and how.
It is a planning document: each candidate still has to pass the
[rule-scope](rule-scope.md) qualification before it becomes a new built-in rule.

## Where a rule should live

A rule can be implemented at three levels. Pick the cheapest one that is correct:

1. **Project a compiler diagnostic.** If K2 already emits a warning for the defect,
   Krit projects it: the oracle retains the diagnostic factory and the rule turns
   it into a finding at compiler-verdict confidence (0.95). This path is
   on by default and needs almost no checker code. See `internal/rules/projection.go`.
2. **Krit-authored FIR checker.** The compiler does not diagnose the defect, but the
   rule needs resolved facts that source analysis cannot get reliably: types,
   nullability, resolved call targets, supertypes across modules, suspend markers.
   Checkers live in `tools/krit-fir/.../checkers/` and currently surface only
   through the opt-in `--fir` pass (see [The `--fir` pass](#the---fir-pass)).
3. **Go rule on the source AST.** Imports and the local syntax tree are enough.

A Go rule that declares `NeedsTypeInfo`, `NeedsResolver`, or `NeedsOracle*` and runs
at medium confidence is a candidate to move up a level: it needs types but infers
them from source. A useful external signal is detekt's `@RequiresTypeResolution`
marker. If detekt only ships a rule with type resolution, Krit's version belongs
at level 1 or 2.

## Which compiler diagnostics can be projected

Only **warnings** can be projected. The oracle's message collectors drop
error-severity diagnostics, and code that compiles never contains them anyway.
The inventory below was verified against kotlinc 2.3.21 (the compiler embedded
in `krit-fir.jar`) with the same checker set the oracle runs. To re-verify
after a compiler upgrade, compile a probe file with the embedded compiler:

```bash
java -cp tools/krit-fir/build/libs/krit-fir.jar \
  org.jetbrains.kotlin.cli.jvm.K2JVMCompiler -no-stdlib -no-reflect \
  -cp <kotlin-stdlib.jar> -d /tmp/out \
  -Xrender-internal-diagnostic-names -Xreport-all-warnings Probe.kt
```

`-Xrender-internal-diagnostic-names` prints each diagnostic's factory name, and the
rendered text is the template the krit-fir collector matches by prefix.

### Already projected

`UNREACHABLE_CODE`, `USELESS_ELVIS`, `CAST_NEVER_SUCCEEDS`,
`UNNECESSARY_NOT_NULL_ASSERTION`, `UNNECESSARY_SAFE_CALL`, `SENSELESS_COMPARISON`,
`USELESS_CAST`, and `DEPRECATION`.

### Projectable warnings that upgrade an existing rule

| Factory | Rule | Notes |
|---|---|---|
| `DEPRECATION` | `Deprecation` | Projected. Covers type references, typealiases, properties, inherited overrides, and Java `@Deprecated` members, and matches overloads exactly. The source path is name-based and misses most of these. On a probe with one deprecation of each kind, recall went from 3/7 (krit-fir) and 2/7 (krit-types) to 7/7 on both. |

Known gap, shared by every projection rule: projection adds compiler-confirmed
findings, but when the compiler reports nothing, the rule's source heuristic
still runs. For `Deprecation` this means a same-file overload (a deprecated
`over(s: String)` next to `over(i: Int)`) can still be flagged by the
name-based path. Treating a missing diagnostic as proof of "not deprecated"
requires knowing that the oracle's diagnostics for the file are complete. That
is a follow-up.

One recall gap specific to `Deprecation`: when the deprecated symbol is the
*receiver* of a call, as in `Old.f()` or `Old.CONST` where `Old` itself is
deprecated, K2 anchors the diagnostic on `Old`. No dispatched node claims that
token: the call's name token is `f`, and the `Old.f` navigation is skipped
because its parent is the call. The compiler's diagnostic is therefore
dropped. References that are never dispatched as calls, navigations, or types
(`::oldFn`, bare identifiers, string templates, operator and index
conventions, imports) are not covered by either path.

### Projectable warnings with no Krit rule yet

Each of these would be a new rule and needs [rule-scope](rule-scope.md)
qualification first. The compiler verdict makes them free of false positives
by construction.

| Factory | Rendered message (prefix) | Candidate value |
|---|---|---|
| `DUPLICATE_BRANCH_CONDITION_IN_WHEN` | `Duplicate branch condition in 'when'.` | High: the second branch is dead code. detekt: `DuplicateCaseInWhenExpression`. |
| `SENSELESS_NULL_IN_WHEN` | `Expression under 'when' is never equal to null.` | High: dead `null ->` branch. Could fold into `UnnecessaryNotNullCheck`. |
| `DIVISION_BY_ZERO` | `Division by zero.` | High but rare. |
| `REDUNDANT_ELSE_IN_WHEN` | `'when' is exhaustive so 'else' is redundant here.` | Medium: dead `else` that also hides new subtypes from exhaustiveness. |
| `UNCHECKED_CAST` | `Unchecked cast of '…' to '…'.` | Medium: noisy in generic code, often intentional. |
| `UNNECESSARY_LATEINIT` | `'lateinit' is unnecessary: definitely initialized in constructors.` | Medium. |
| `DEPRECATED_IDENTITY_EQUALS` | `Identity equality for arguments of types '…' and '…' is deprecated.` | Medium: `===` on value types. Narrower than `AvoidReferentialEquality`. |
| `REDUNDANT_CALL_OF_CONVERSION_METHOD` | `Redundant call of conversion method.` | Low (style). |
| `NON_FINAL_MEMBER_IN_FINAL_CLASS` | `'open' has no effect on a final class.` | Low (style). |
| `REDUNDANT_NULLABLE` | `Redundant '?'.` | Low (style). |
| `FINAL_UPPER_BOUND` | `Type '…' is final, so the value of the type parameter is predetermined.` | Low (style). |
| `NOTHING_TO_INLINE` | `Expected performance impact from inlining is insignificant.` | Low (style). |
| `PLATFORM_CLASS_MAPPED_TO_KOTLIN` | `This class is not recommended for use in Kotlin.` | Low (style). |

`USELESS_ELVIS_RIGHT_IS_NULL` and `USELESS_IS_CHECK` are deliberately not retained.
The krit-types backend doesn't project the first, and the second renders a
template identical to `IMPOSSIBLE_IS_CHECK`, so a prefix match cannot tell them
apart.

### Adding a projection

1. Probe the factory with the embedded compiler (see above) and confirm it is a
   warning under the oracle's default checker set.
2. Retain it on **both** backends: add the rendered message to
   `OracleDiagnosticMessageCollector` in krit-fir, which matches by prefix or by
   an anchored pattern when the template starts with a variable such as a symbol
   name, and add the factory name to `retainedDiagnosticFactories` in krit-types.
   Keep `DiagnosticFactoriesTest` in step.
3. Check that the template cannot collide with another factory's rendering (the
   `USELESS_IS_CHECK`/`IMPOSSIBLE_IS_CHECK` trap), and add a krit-fir test that
   the neighboring factory is not claimed.
4. Project it from the rule with `projectDiagnostic`. Claim each diagnostic at
   exactly one dispatched node. When the rule dispatches on several node types
   that can cover the same span, add an `Accept` check that anchors on the
   token the compiler reports.
5. Bump `oracle.CacheVersion`. The backends now emit different facts for the
   same source.
6. Add a cross-backend test in `tests/parity` asserting that both backends
   report the diagnostic, and add the rule to the compiler-parity map when the
   mapping is one-to-one.

krit-types collects diagnostics for every analyzed file. It used to skip files
that contained no null-safety or control-flow tokens, but no token predicts a
deprecation, so that pre-gate was removed together with the `DEPRECATION`
projection.

### Not projectable

- **Errors:** `NO_ELSE_IN_WHEN` (non-exhaustive `when` on a sealed, enum, or
  Boolean subject), `EQUALITY_NOT_APPLICABLE`, `NO_RETURN_IN_FUNCTION_WITH_BLOCK_BODY`.
  Compiling code never contains them. Exhaustiveness rules such as `NonExhaustiveWhen`
  and `ElseCaseInsteadOfExhaustiveWhen` target shapes the compiler accepts, so
  they stay source rules.
- **Not emitted by the oracle's checker set:** `UNUSED_VARIABLE`, `UNUSED_PARAMETER`,
  `UNUSED_EXPRESSION`, `VARIABLE_NEVER_READ`, and `USELESS_CALL_ON_NOT_NULL` (K2
  extended checkers, which the oracle leaves off).
- **IDE inspections, not compiler diagnostics:** `REDUNDANT_EXPLICIT_TYPE` and
  similar.

## Krit FIR checkers to build

These rules have no compiler diagnostic, and their Go versions infer types from
source today. They are grouped by the compiler capability they need.

**Nullability and data flow:** `CanBeNonNullable`, `NullableToStringCall`,
`MapGetWithNotNullAssertionOperator`, `CastNullableToNonNullableType`.

**Resolved call targets and annotations:** `IgnoredReturnValue` (resolved callee plus
its annotations), `ForbiddenMethodCall` (resolved fully qualified name), `ImplicitDefaultLocale`
(resolved receiver), `MissingUseCall` (subtype of `Closeable`).

**Type hierarchy:** `RecyclerAdapterWithoutDiffUtil`, `RecyclerAdapterStableIdsDefault`,
`ImageLoadedAtFullSizeInList` (subtype of `RecyclerView.Adapter`),
`DontDowncastCollectionTypes`, `ObjectExtendsThrowable`, `ErrorUsageWithThrowable`,
`OverrideSignatureMismatch`, `AbstractMemberNotImplemented`. One shared
is-subtype-of helper unlocks most of this group.

**Coroutine semantics:** `RedundantSuspendModifier`, `WithContextInSuspendFunctionNoop`,
`SuspendFunSwallowedCancellation`, `MainDispatcherInLibraryCode`. New candidates:
`SuspendFunWithFlowReturnType`, `SleepInsteadOfDelay`.

**Security receiver typing:** `SqlInjectionRawQuery`, `RoomRawQueryStringConcat`,
`JdbcStatementExecute`, `ProcessBuilderShellArg`, `RuntimeExecUnsafeShape`,
`ContentProviderQueryWithSelectionInterpolation`. All of them need to know
whether the receiver really is a `SQLiteDatabase`, `Statement`, or
`ProcessBuilder`. Resolved receiver types remove the name-collision false
positives, and simple intra-procedural checks on the argument can build on that.

**Control flow:** `UnreachableCatchBlock` (a catch of a subtype after its supertype).

## Suggested order

1. Projection batch: `DEPRECATION` (done), then the high-value no-rule
   warnings above, once each passes rule-scope.
2. Security receiver typing: highest severity and the worst false-positive rate
   in source analysis today.
3. `IgnoredReturnValue` and `RedundantSuspendModifier`: rules the wider
   ecosystem only ships with type resolution.
4. The type-hierarchy group, built on one shared is-subtype-of helper.

To port a rule, follow the [FIR checker authoring checklist](fir-checker-authoring.md).
A built-in checker is a Kotlin singleton object implementing `FirRule` in
`tools/krit-fir/src/main/kotlin/dev/jasonpearson/krit/fir/checkers/<category>/<RuleId>.kt`.
Its `ruleId` is the exact Go catalog ID; it contributes an `ExpressionCheckers`
and/or `DeclarationCheckers` set and calls `report(source, message)` for findings.
The plugin discovers these objects recursively from its jar or classes directory
and merges every K2 2.3.21 checker-set property. A new checker needs only its
own file and test data: no registry, diagnostic factory, or Go mapping edit.
The `check` request selects rule IDs and may pass per-rule `ruleConfigs` options;
`config()` reads the current compile's options. Oracle analysis explicitly
selects zero built-in rule checkers while retaining its existing oracle checkers.
The external Kotlin rule API is a separate subsystem.

Every promoted rule should add positive and negative fixtures in
`compiler-tests` and independent property, differential, and fuzz coverage.
`FixtureParityTest` also checks each checker against its Go rule's
`tests/fixtures` positive and negative fixtures.
The existing `tests/parity` grids illustrate compile-failure guards that stop
negative cases from passing vacuously; extending their shared lists is not
required to register a checker.

## The `--fir` pass

`krit --fir` (off by default; `--no-fir` turns it back off) runs the FIR
checkers after the Go rules and lets the compiler decide wherever it can.
`krit --daemon --fir` forwards the flag, and `--no-fir-daemon`, to the daemon,
which runs the same pass.

**Compile context.** The checkers compile the whole module the way the oracle
does: every Kotlin file under the JVM source roots (`src/<set>/kotlin` and
`java`, with non-JVM Kotlin Multiplatform sets such as `jsMain` and `iosMain`
left out), the scanned files, and `oracle.classpath` plus the `CLASSPATH`
environment variable, on top of the bundled Kotlin stdlib. Only `.kt` files are
checked: Kotlin scripts (`build.gradle.kts`, `settings.gradle.kts`, ...) and
scanned files from a non-JVM source set are never sent to the checkers, and
`-v` counts them as excluded.

**Which files FIR decides.** For each checked file the response says whether
the compiler analyzed it cleanly. A file is *gated* when it has an
error-severity compiler diagnostic (an unresolved reference, often from a
library missing from `oracle.classpath`), when a location-less compiler error
affects the whole compilation, or when the compiler crashed. `-v` lists the
gated files with the first error.

**The verdict.** For every file that is not gated, and every rule the jar has a
checker for, FIR's findings are the final findings:

- A Go finding that a FIR finding overlaps (by byte range, else on the same
  line) is kept as Go reported it, with its message, range, and autofix.
- A Go finding with no FIR counterpart is dropped: the compiler says the code
  is clean.
- A FIR finding with no Go counterpart is added with FIR's message. It goes
  through the same filters as Go findings first: `@Suppress`,
  `@SuppressWarnings`, `detekt:`/`"all"` spellings, `@file:Suppress`,
  `// krit:ignore`, and the rule's `excludes` globs. Baselines, `--diff`, and
  `--min-confidence` apply afterwards to every finding alike.

Everywhere else (gated files, files FIR did not check, rules without a
checker) Go's findings stand and FIR's are discarded. `-v` prints, per rule,
how many findings FIR confirmed, dropped, added, and enriched with Go's fix,
and how many files were gated.

**The per-rule contract.** Authority is keyed by rule ID across every checked
Kotlin file: once the jar has a checker for a rule ID, every Go finding under
that ID in a checked `.kt` file is dropped unless the checker reports it too.
A checker must therefore cover the Go rule's full Kotlin scope, every kind of
finding the Go rule reports under that ID, before it is added. A checker that
covers only part of a rule ships under a new rule ID instead. Go findings in
Java, XML, and Gradle files are unaffected, since only `.kt` files are checked.

**Known gap.** Gating is per file and driven by diagnostics located in that
file. An error in a source-dir file that is not itself checked (a broken
declaration in a dependency, for example) can leave an error type behind that
reaches a checked file without any diagnostic located there. That checked file
is not gated, and its verdict is computed against the broken type.

**Caching.** FIR findings are cached per file under `.krit/fir-cache`, keyed by
the whole compilation (every source path and content, the classpath, and the
jar) plus the enabled rules and their options, so any source edit re-runs the
checkers for every file.
