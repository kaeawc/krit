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
  generator turns them into test classes and methods. Golden files and stub
  smoke files run with every rule enabled. If your checker fires in someone
  else's data, treat it as a false positive until you have shown otherwise.
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
cleanly, and whenever FIR is disabled or unavailable. A checker therefore must
not report findings Go would not, and must not miss findings Go reports, unless
the difference is a deliberate precision fix that you test.

Do not implement `@Suppress`, `excludes`, rule activation, or baselines in a
checker. Go applies all of them to FIR findings, just as it does to its own
findings.

## 8. Validation

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
