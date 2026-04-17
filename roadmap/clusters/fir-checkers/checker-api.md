# B.3 — Internal checker API

**Cluster:** [fir-checkers](README.md) · **Status:** planned · **Track:** B · **Severity:** n/a (tool mode)

## Catches

Once the JVM runner can execute a `FirAdditionalCheckersExtension`
(B.1) and the Go side can wire it up (B.2), we need an **internal
Kotlin API** for writing checker rules: a base class hierarchy, a
diagnostic declaration file, a registration mechanism, and a test
harness. This is where rule authors actually live. Getting the
shape right early is cheaper than porting 50 rules and then
discovering the base classes don't compose.

## The shape (copied from Metro, adapted for linting)

### 1. Diagnostic declarations — `KritDiagnostics.kt`

A single Kotlin file declaring every FIR-backed rule's diagnostic
factory. Metro's `MetroDiagnostics.kt` is the template:

```kotlin
object KritDiagnostics : KtDiagnosticsContainer() {
  val FLOW_COLLECT_IN_ON_CREATE: KtDiagnosticFactory0 by warning0()
  val COMPOSE_REMEMBER_WITHOUT_KEY: KtDiagnosticFactory1<String> by warning1()
  val UNSAFE_CAST_WHEN_NULLABLE: KtDiagnosticFactory0 by error0()
  // ...
}
```

Each diagnostic has a rendered message (via a separate
`KritDiagnosticsRendering.kt`) and a severity (warning / error /
info). Severity maps to krit's existing `scanner.Finding.Severity`.

Diagnostics are looked up by the collector (`FindingCollector`
from B.1) and translated to `scanner.Finding` by name. The
diagnostic factory's short name becomes the `rule` field; the
rendered message becomes `message`; source range becomes `line` /
`col` / byte offsets.

### 2. Checker base classes — extend kotlinc's, don't wrap

Checker classes extend the same base classes Metro uses, directly
from `org.jetbrains.kotlin.fir.analysis.checkers.*`:

| Need | Extend | Examples |
|---|---|---|
| Class-level check | `FirClassChecker(MppCheckerKind.Common)` | ComposeStableAnnotationMisuse, DependencyGraphChecker equivalent |
| Callable check | `FirCallableDeclarationChecker(MppCheckerKind.Common)` | SuspendFunctionWithoutContext, PublicApiMissingKdoc |
| Function-call check | `FirFunctionCallChecker(MppCheckerKind.Common)` | FlowCollectInOnCreate, CoroutineScopeInComposition |
| Annotation check | `FirAnnotationChecker(MppCheckerKind.Common)` | SerializableWithoutExplicitOrder |
| Type-ref check | `FirTypeRefChecker(MppCheckerKind.Common)` | PlatformTypeInPublicApi |

Each checker is a stateless `object`. Example template:

```kotlin
internal object FlowCollectInOnCreate : FirFunctionCallChecker(MppCheckerKind.Common) {
  context(context: CheckerContext, reporter: DiagnosticReporter)
  override fun check(expression: FirFunctionCall) {
    val source = expression.source ?: return
    val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
    if (callee.callableId.asSingleFqName().asString() != "kotlinx.coroutines.flow.Flow.collect") return
    val container = context.containingDeclarations
      .filterIsInstance<FirSimpleFunctionSymbol>()
      .firstOrNull() ?: return
    if (container.name.asString() != "onCreate") return
    if (context.isInsideLifecycleScopeCall()) return
    reporter.reportOn(source, KritDiagnostics.FLOW_COLLECT_IN_ON_CREATE)
  }
}
```

**Why not a krit-specific wrapper?** Option A in
[jvm-runner.md](jvm-runner.md) locks in kotlinc's base classes.
A wrapper layer would hide the things rule authors actually need
(containing declaration stack, smart-cast state, symbol resolution)
and make Track C impossible without a rewrite. The cost of
exposing kotlinc types in checker code is real but it's the only
shape that dual-packages cleanly.

### 3. Registration — stateless `Set<Checker>` on the extension

Exactly Metro's pattern. `KritFirCheckers.kt`:

```kotlin
internal class KritFirCheckers(session: FirSession) : FirAdditionalCheckersExtension(session) {
  override val declarationCheckers = object : DeclarationCheckers() {
    override val classCheckers = setOf(ComposeStableAnnotationMisuse, /* ... */)
    override val callableDeclarationCheckers = setOf(SuspendFunctionWithoutContext)
  }
  override val expressionCheckers = object : ExpressionCheckers() {
    override val functionCallCheckers = setOf(FlowCollectInOnCreate)
  }
}
```

Registered once in `KritFirExtensionRegistrar` via `+::KritFirCheckers`.

### 4. ServiceLoader extension point (deferred to Track C)

Metro uses `ServiceLoader<MetroFirDeclarationGenerationExtension>`
to let downstream projects register their own extensions. The
equivalent for us would be `ServiceLoader<KritFirChecker>` so
third parties can add their own checker `object`s without forking
krit. **Deferred** — not needed for Track B because krit ships
its own checker set. Revisit after Track C.1 when the plugin JAR
is public and third-party rule development becomes a real use
case.

## Test harness

Adopt **JetBrains' compiler-test framework directly**, not a
custom fixture format. Metro uses it; their
[`compiler-tests/`](../../../../../github/metro/compiler-tests/)
directory is the template.

### Shape

```
tools/krit-fir/compiler-tests/
  src/test/data/diagnostic/
    flow/
      FlowCollectInOnCreate.kt              ← golden-value test file
      FlowCollectInLifecycleScope.kt        ← negative test
    compose/
      ComposeRememberWithoutKey.kt
  src/test/kotlin/dev/krit/fir/tests/
    AbstractDiagnosticTest.kt               ← base test class
    GenerateTests.kt                        ← regenerates test methods from data files
```

Each `.kt` under `src/test/data/diagnostic/` is both the input
and the expected output. Diagnostics are annotated inline using
the compiler-test DSL:

```kotlin
// RENDER_DIAGNOSTICS_FULL_TEXT
package test

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow

class MyFragment : Fragment() {
  override fun onCreate(savedInstanceState: Bundle?) {
    super.onCreate(savedInstanceState)
    val state: Flow<Int> = MutableStateFlow(0)
    state.<!FLOW_COLLECT_IN_ON_CREATE!>collect<!> { println(it) }
  }
}
```

Running `./gradlew :tools:krit-fir:compiler-tests:generateTests`
regenerates JUnit test methods from the data files. Running the
generated tests runs the FIR checker against each file and
verifies the inline annotations match the emitted diagnostics.

**Why not reuse krit's existing `tests/fixtures/` directory?**
The existing fixtures are `.kt` files plus expected `.findings`
sidecars, driven by Go test harness code. That format doesn't
cleanly express "diagnostic fired at this exact token range with
these type arguments" — which is exactly what FIR checkers need
to assert. JetBrains' framework does. The two test systems can
coexist: Go rules keep their format, FIR rules use the compiler
framework.

Install the
[Compiler DevKit IntelliJ plugin](https://github.com/JetBrains/kotlin-compiler-devkit)
for running individual test data files directly from the IDE
(Metro's `compiler-tests/README.md` recommends this — quoted
verbatim from their docs).

## Definition of done

- `KritDiagnostics.kt` exists with at least three registered
  diagnostics (one per pilot rule in Track B.4)
- `KritFirCheckers.kt` registers those three as a
  `FirAdditionalCheckersExtension`
- `compiler-tests/` module builds and `./gradlew generateTests`
  produces test methods from data files
- At least one positive + one negative golden-value test per
  pilot rule
- Tests run under `./gradlew :tools:krit-fir:compiler-tests:test`
  in under 60 s

## Non-goals (for this concept)

- Which specific rules to port — see
  [pilot-rules.md](pilot-rules.md)
- Parity oracle against existing Go implementations — see
  [pilot-rules.md](pilot-rules.md)
- Fix emission format — fixes follow the same `scanner.Fix`
  struct shape the Go rules use; the collector reads
  `reporter.reportOn(source, factory)` and synthesizes a `Fix`
  from the checker's optional `quickFix(source)` method when
  present. Detailed design deferred until the first pilot rule
  wants to emit a fix.
