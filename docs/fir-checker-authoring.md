# Authoring a FIR checker

Checklist for porting one existing krit rule to a K2 FIR checker in
`tools/krit-fir`. One checker per change. Background on which rules qualify is in
[fir-checker-candidates.md](fir-checker-candidates.md).

## 1. Prerequisites

- The rule is registered in Go (`api.Registry`) and has an entry in
  `schemas/krit-config.schema.json`. If you just added the Go rule, run
  `make schema` to regenerate the schema. krit sends krit-fir only the IDs of
  active Go rules, so a FIR checker whose ID has no Go rule never runs in
  production (`UnsafeCastWhenNullable` is one today).
- The rule has Kotlin Go fixtures `tests/fixtures/positive/<category>/<RuleId>.kt`
  and `tests/fixtures/negative/<category>/<RuleId>.kt`. A `.java`-only fixture
  does not count, because FIR checks Kotlin. FIR parity (step 6) runs against
  these fixtures.
- K2 does not already report the Go rule's positives. Before porting, compile
  the Go positive fixture and check the compiler's own diagnostics:
  - If a K2 diagnostic covers every shape the Go rule reports, do not write a
    checker: project that diagnostic onto the Go rule ID instead.
  - If K2 rejects only some shapes (a compiler error), those files are gated and
    Go stays authoritative for them. Port the rule for the shapes that still
    compile, mark the non-compiling Go positive fixture with
    `// fir-parity: skip <reason>`, and pin the compiling positives in golden
    data. Port it only when those remaining shapes are worth a checker.
  `SynchronizedOnBoxedPrimitive` is the example of the second case: from
  language version 2.1, K2 reports `synchronized` on a primitive as the error
  `SYNCHRONIZED_BLOCK_ON_VALUE_CLASS_OR_PRIMITIVE`, so the Go positive never
  compiles cleanly and the ported checker is only authoritative for
  monitor-lock wrappers (see
  [A compiler error gates the whole file](#a-compiler-error-gates-the-whole-file)).

## 2. File layout and naming

- One file: `tools/krit-fir/src/main/kotlin/dev/jasonpearson/krit/fir/checkers/<category>/<RuleId>.kt`,
  package `dev.jasonpearson.krit.fir.checkers.<category>`. `<category>` is the
  Go rule set without hyphens (`potential-bugs` -> `potentialbugs`).
- Declare `internal object <RuleId> : Fir…Checker(MppCheckerKind.Common), FirRule`
  with `override val ruleId = "<RuleId>"`, equal to the Go ID character for
  character. Contribute the checker through `expressionCheckers`,
  `declarationCheckers`, or `typeCheckers`, for example
  `override val expressionCheckers = object : ExpressionCheckers() { override val functionCallCheckers = setOf(<RuleId>) }`.
- It must be a Kotlin `object`. `FirRuleDiscovery` finds it by scanning the jar;
  a class, or a duplicate ID, fails discovery.
- Do not edit shared files: `FirRule.kt`, `FirRuleDiscovery.kt`,
  `FirRuleMerge.kt`, `KritFirCheckers.kt`, `KritDiagnostics.kt`,
  `AnalysisSession.kt`, the compiler-test harness, or any Go code. There is no
  registry, diagnostic factory, or Go mapping to update. The one exception is
  additive stub declarations (step 6).

## 3. Reporting

- Report with `report(source, message)`. It emits `KRIT_RULE` as
  `[<RuleId>] <message>`.
- Use Go's user-facing message text for the same finding, including any
  interpolated names. Copy it from the message the Go rule passes to
  `ctx.Emit` or `ctx.EmitAt`.
- Report on the same line as Go. Anchor on the element Go reports on, such as
  the call name, the argument, or the declaration name. The column does not
  have to match, because many Go rules report column 1. Parity and merge dedup
  compare the file, the rule, and the line. Do not report on a whole function,
  class, or file unless Go does.

## 4. Matching rules

- Match symbols on `callableId` (`kotlinx/coroutines/Dispatchers.IO`) or on the
  owning `classId` plus the member name. Also match the supertype chain by
  `classId`.
- Never decide on `dispatchReceiver == null`, `isStatic`, symbol origin, or
  whether a property is synthetic. These differ between Java stubs, Kotlin stubs
  of Java-origin AndroidX types, and the real binaries (see the stubs README).
- Android platform classes are Java stubs (`stubs/java/**`). Kotlin code sees
  them as Java: statics without `Companion`, synthetic properties over
  getters/setters, and platform types.
- `Build.VERSION.SDK_INT` is not a compile-time constant. Do not rely on
  constant folding of `SDK_INT >= N`; read the comparison operands instead.
- Stop body walks at real scope boundaries: nested functions, lambdas,
  anonymous functions, local classes and objects, and shadowing declarations.
  `context.containingDeclarations` lists the enclosing symbols, and that list
  includes lambdas as `FirAnonymousFunctionSymbol`. To find the nearest named
  function, the equivalent of Go's `function_declaration`, filter for
  `FirNamedFunctionSymbol`.
- Implement the Go rule's full behavior, not a narrower reading of its name.
  Read the Go implementation for the exact callbacks, wrappers, and exemptions
  it covers.
- Require receiver or owner proof for common method names (`collect`, `launch`,
  `query`, `d`, `execute`). A local lookalike with the same name must not fire.
- Never look up a class by `classId` from a symbol that may be local or
  anonymous. `getClassLikeSymbolByClassId` (and anything that resolves a class
  id through the symbol provider) throws for a local or anonymous class, and
  the exception aborts every checker in the compilation. Get the owner from the
  containing-class lookup tag (`getContainingClassSymbol()`), which is bound to
  local classes, and compare class ids you already hold instead of resolving
  them. Cover a member of an `object { ... }` expression in the golden data.

### Skipping test files

If the Go rule skips test files (`scanner.IsTestFile(file.Path)`), skip them
with `isInTestFile()` inside `check` (it needs the `CheckerContext`), or with
`isTestFile(path)` when you already have the file path:

```kotlin
context(context: CheckerContext, reporter: DiagnosticReporter)
override fun check(declaration: FirProperty) {
    if (isInTestFile()) return
    // ...
}
```

Do not match path markers such as `/test/` or `src/*Test` in the checker, and
do not copy the Go list of test paths. The Go FIR pass classifies every
requested file with `scanner.IsTestFile`, the same call the Go rules make with
the configured `testSourcePaths` / `testSourcePathsOverride`. It sends the test
files in the check request as `testFiles`. krit-fir stores them in
`FirRuleContext`, and the helpers read them from there. Outside a check request
(oracle compiles, a bare compiler run, the golden tests) nothing counts as a
test file. To test the skip, pass `testFiles` to `KritFirProbe.compile` (see
`StateFlowMutableLeakTestFileTest`). The classification is part of the FIR
cache fingerprint, so a change to the test paths invalidates cached verdicts.

## 5. Options

- Read options with `config()`. It returns the Go rule's options keyed by the Go
  option names: integers as `Long`, lists as `List<Any?>`, and booleans and
  strings as-is.
- When an option is absent, fall back to the same default as Go. The operative
  default is the value in `config/default-krit.yml`, which must match the Go
  rule's meta and struct defaults.

## 6. Tests

All tests live under `tools/krit-fir/compiler-tests/src/test/`.

- **Golden data:** `data/diagnostic/<category>/<RuleId>*.kt`. Mark each expected
  finding inline as `<!<RuleId>!>token<!>` on the reported line. Cover
  positives, negatives, local-lookalike negatives (same names in another
  package or class), scope-boundary negatives, and Java-interop cases where the
  rule touches Java types. Java-interop cases are Kotlin code calling the Java
  stubs. File and directory names must be valid Kotlin identifiers, because the
  generator turns them into test classes and methods. A golden file runs with
  only its own rules enabled: the rule its name starts with plus any rule named
  in its markers, so name every golden file `<RuleId>….kt`. Stub smoke files
  (and any file that names no rule) run with every rule enabled and must report
  nothing, so if your checker fires in a smoke file, treat it as a false
  positive until you have shown otherwise.
- **Fixture parity** runs against the Go fixtures in two tiers, and both must
  pass:
  1. *Fast, lane-local:* `FixtureParityTest` in `compiler-tests` runs as part
     of `./gradlew test`. It compiles each Go fixture against the stubs with
     only your rule enabled. The positive fixture must produce at least one
     finding and the negative fixture none. It also fails when the rule has no
     schema entry or lacks either `.kt` fixture.
  2. *Exact:* `TestFirFixtureParity` in `tests/parity` runs with `go test` and
     needs the shadow jar and a JDK. It enumerates rules with
     `krit-fir --list-rules` and compiles every rule's fixtures through the
     jar against the same stubs. For each line, the number of FIR findings
     must equal the number of findings from the in-process Go rule, which runs
     with source inference and no oracle. `api.Registry` decides whether the
     Go rule exists.

  A fixture must compile cleanly: the failure lists the compiler errors.
  Library and platform symbols belong in the stubs. You can declare helpers
  that belong only to the fixture in the fixture itself, as long as the Go
  verdict does not change. As a last resort, add
  `// fir-parity: skip <reason>` to the fixture with a real reason. The marker
  fails as stale once the fixture compiles. If a checker disagrees with its Go
  fixture, fix the checker or report the disagreement. Do not weaken the
  fixture.
- **Stubs:** follow `data/stubs/README.md`. Use the real signatures, the Java
  layer for Java libraries, one declaration per line, never redeclare, and add
  a smoke file for each new stub or Java package.

## 7. How the verdict is used

`krit --fir` sends krit-fir the IDs and options of the active Go rules, and each
FIR checker runs on the files that compile. For a rule with a FIR checker, the
FIR result is authoritative on files that compile cleanly: it replaces the Go
rule's findings there. Go remains the fallback on files that do not compile
cleanly, and whenever FIR is disabled or unavailable.

A checker that throws does not take the compile down with it. krit-fir wraps
each rule's checkers so an exception is recorded against that rule and the file
being checked, and the compile and every other rule carry on. The check
response lists it under `ruleErrors` (rule, file, message). Go then treats only
that (file, rule) pair as non-authoritative: it keeps its own findings for the
rule on that file and drops any FIR findings the checker reported there before
throwing. Other files, and other rules on the same file, keep the FIR verdict.
`krit --fir -v` prints each rule error and counts them per rule
(`rule-errors=N`). Rule errors are cached with the file's entry, so a warm run
repeats the cold run's output until the source, the classpath, or the jar
changes. Cancellation (`ProcessCanceledException`) and JVM failures such as
`OutOfMemoryError` still propagate. Only check requests isolate checkers: the
`:compiler-tests` harness runs them unwrapped, so an exception there fails the
test. A rule error is a bug in the checker: fix
it, do not rely on the fallback. Because each rule's checkers are wrapped
separately, register a checker object in only one checker set: K2 no longer
deduplicates the same object across overlapping sets, so it would run twice.

Do not implement `@Suppress`, `excludes`, rule activation, or baselines in a
checker. Go applies all of them to FIR findings, just as it does to its own
findings.

### Parity principle

Because FIR replaces Go per rule on clean Kotlin files, a checker is held to
these rules. The pilot ports (`WeakMessageDigest`, `RsaNoPadding`,
`SynchronizedOnBoxedPrimitive`, `ErrorUsageWithThrowable`,
`StateFlowMutableLeak`, `SetJavaScriptEnabled`) were all built this way.

- **Never lose a Go true positive.** Every finding Go reports on real
  vulnerable or buggy code must also come from the checker. A lost true
  positive is a regression that only shows up when `--fir` is on.
- **Never add a false positive.** The checker must not report code that Go
  correctly leaves alone.
- **Decide artifact vs. true positive by the message, not by how Go found
  it.** A Go finding is an artifact only if the reported code lacks the
  property the finding's message asserts: the lock is not a primitive, the
  type is not a `MutableStateFlow`, the algorithm is not a weak digest. How Go
  found it (a simple-name fallback, a text match, a guessed type) is
  irrelevant. If the
  message is true of the code, the finding is a true positive and the checker
  must keep it, even when Go reached it through a fallback the checker would
  not otherwise take. For example, `SetJavaScriptEnabled` keeps Go's findings
  on a third-party `com.tencent.smtt.sdk.WebSettings`: Go finds it by the
  simple name `WebSettings`, but the code does enable JavaScript in a
  WebView.
- **Do not copy Go's mistakes.** Some Go findings are tree-sitter artifacts or
  Go false positives by the test above: a substring match on the property
  text, a type guessed from a same-named declaration elsewhere in the file, or
  a secondary constructor body parsed as a plain block, where the code does
  not have the property the message asserts. Keep the more correct FIR
  behavior. Pin it with a golden case whose comment names the Go divergence,
  for example `// Go reports this because the class declares size: Int; the
  lock is ...`. See `SynchronizedOnBoxedPrimitiveDivergence.kt` and
  `StateFlowMutableLeakPrecision.kt`.
- **Match Go by default.** When either answer is defensible (which visibility
  counts as exposed, whether a wrapper call counts), do what Go does. The
  checker is a more precise version of the same rule, not a different rule.
- **Pin deliberate improvements as golden positives.** When FIR catches a true
  positive Go misses (an inferred type, an import or type alias, a subtype
  whose name does not show it), add a golden positive with a comment saying
  Go misses it and why.
- **The reviewer decides every divergence, not the lane.** Every difference
  from Go, in either direction (a Go finding dropped, a finding Go misses
  added), must be listed in the PR description's `Divergences` table for the
  reviewer to accept:

  | Code shape | Go | FIR | Golden case |
  | --- | --- | --- | --- |
  | `view.settings.extra.javaScriptEnabled = true` (`extra` is not a `WebSettings`) | reports | no finding | `SetJavaScriptEnabledNegative.kt` |

  An unlisted divergence is a bug, and a listed one the reviewer does not
  accept goes back to matching Go.

The Go fixtures under `tests/fixtures/` must match exactly:
`TestFirFixtureParity` requires the same finding count on every line, so a Go
fixture never carries a divergence. Divergences live only in the
`compiler-tests` golden data, pinned there and listed in the PR's
`Divergences` table. A line-count difference on a Go fixture is a bug in the
checker (or a fixture that needs a `// fir-parity: skip` reason, step 6).

### A compiler error gates the whole file

A file with any compiler ERROR is not authoritative for any rule. krit-fir
reports it in `errorFiles`, and Go's findings stand for that file for every
FIR-backed rule, not just the rule whose code caused the error. K2 also stops
reporting warnings, and `KRIT_RULE` findings are warnings, once a compilation
has an error, so an error-free file is only authoritative because krit-fir
turns on `reportAllWarnings`.

This matters when the Go rule targets code that K2 itself rejects. Since
language version 2.1, K2 makes `synchronized` on a primitive lock an error
(`SYNCHRONIZED_BLOCK_ON_VALUE_CLASS_OR_PRIMITIVE`). The Go positive fixture for
`SynchronizedOnBoxedPrimitive` therefore never compiles cleanly. It carries
`// fir-parity: skip <reason>`, Go stays authoritative for it, and the FIR
positives are covered by golden data that compiles, through a monitor-lock
wrapper. If your rule's positives cannot compile, make the same split: skip
marker on the Go fixture, compiling golden coverage in `compiler-tests`.

## 8. Adversarial scope-parity review (required)

Every port gets an adversarial review against the Go rule before it merges.
The review runs after the port commit and its fixes land as a separate
`fix(krit-fir): ...` commit. Every pilot checker needed one. The reviewer reads
the Go implementation, not just its fixtures, and tries to break parity. That
includes the helpers the rule calls, not just the rule body: fallback name
lists, shared receiver and type matchers, and simple-name fallbacks (for
example `SetJavaScriptEnabled`'s helper also accepts any type named
`WebSettings`) define findings the fixtures never show.

- [ ] **Scope:** every node type, callback, wrapper, receiver shape, and
      container the Go rule visits is covered: top-level, member, companion,
      object, and interface declarations; nested and local scopes; delegated,
      getter-backed, and aliased forms.
- [ ] **Exemptions:** every Go early return (test files, visibility, override,
      suppressed owners, config options) is mirrored, and none is added.
- [ ] **Lost true positives:** for each Go positive shape, look for a way the
      checker's resolution fails to see it: unresolved receivers, platform
      types, flexible or nullable types, type aliases, import aliases, star
      imports, smart casts (including unstable ones), subtypes.
- [ ] **New false positives:** local lookalikes, same-package shadows,
      companion shadows, declarations named like library symbols, Java
      lookalikes, and third-party classes with the same simple name.
- [ ] **Message and line:** the same message text, reported on the line Go
      reports.
- [ ] **Divergences:** every intentional difference is pinned in golden data
      with a comment naming the Go behavior, and listed in the PR's
      `Divergences` table, as the parity principle requires. Each dropped Go
      finding passes the message test: the code lacks what the message
      asserts.
- [ ] **Regression tests:** each review finding gets a golden case (or a probe
      test) that fails before the fix.

## 9. Validation

```bash
cd tools/krit-fir
./gradlew --no-daemon test        # all projects, including :compiler-tests
./gradlew --no-daemon shadowJar   # last: `test` leaves a thin jar in build/libs
cd ../..
go build -o krit ./cmd/krit/ && go vet ./... && golangci-lint run ./... && go test ./... -count=1
go test ./tests/parity/ -count=1 -run TestFirFixtureParity -v   # needs the shadow jar and a JDK
make integration
```

The `TestFirFixtureParity` run must show your rule as passing, not skipped.
In CI, the `krit-fir` job runs the fast tier through `./gradlew --no-daemon test`.
The `oracle-backend-parity` job builds the shadow jar and runs the exact tier.
It fails if that test skips.
