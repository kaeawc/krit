package rules_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func benchmarkRuleByName(b *testing.B, ruleName string, code string) {
	b.Helper()
	dir := b.TempDir()
	path := filepath.Join(dir, "test.kt")
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		b.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		b.Fatalf("ParseFile(%s): %v", path, err)
	}
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == ruleName {
			rule = r
			break
		}
	}
	if rule == nil {
		b.Fatalf("rule %q not found", ruleName)
	}
	d := rules.NewDispatcher([]*api.Rule{rule}, nil)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = d.Run(file)
	}
}

// --- Deprecation ---

func TestDeprecation_Positive(t *testing.T) {
	findings := runRuleByName(t, "Deprecation", `package test

@Deprecated("Use NewClass instead")
class OldClass

fun caller() {
    val x: OldClass = OldClass()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for using deprecated class")
	}
}

func TestDeprecation_Negative(t *testing.T) {
	findings := runRuleByName(t, "Deprecation", `
package test
fun currentMethod() {}

fun caller() {
    currentMethod()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- HasPlatformType ---

func TestHasPlatformType_Positive(t *testing.T) {
	findings := runRuleByName(t, "HasPlatformType", `
package test
fun getData() = JavaClass.getData()
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for public function without explicit return type")
	}
}

func TestHasPlatformType_Negative(t *testing.T) {
	findings := runRuleByName(t, "HasPlatformType", `
package test
fun getData(): String = JavaClass.getData()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestHasPlatformType_NegativeKotlinExpression(t *testing.T) {
	// Pure Kotlin expressions should not be flagged without resolver confirmation
	cases := []string{
		`package test
fun fromCode(code: Int) = entries.first { it.code == code }`,
		`package test
fun ensureDecryptionsDrained() = withTimeoutOrNull(5000) { drain() }`,
		`package test
fun getCurrentAvatar() = getState().currentAvatar`,
		`package test
fun count() = items.size`,
		`package test
fun name() = listOf("a", "b").first()`,
	}
	for i, src := range cases {
		findings := runRuleByName(t, "HasPlatformType", src)
		if len(findings) != 0 {
			t.Errorf("case %d: expected no findings for pure Kotlin expression, got %d", i, len(findings))
		}
	}
}

func TestHasPlatformType_NegativeJavaSubstringInIdentifier(t *testing.T) {
	// "Java" appearing inside an identifier (parseJavadoc, getJavadocFor)
	// must not trigger the rule — the substring "Java" is not at a token
	// boundary, so it is not a Java-interop reference.
	cases := []string{
		`package test
fun renderDocs(content: String) = parseJavadoc(content)`,
		`package test
fun lookupDoc(id: Int) = registry.getJavadocFor(id)`,
		`package test
fun decode(raw: String) = base64.decodeRaJava(raw)`,
	}
	for i, src := range cases {
		findings := runRuleByName(t, "HasPlatformType", src)
		if len(findings) != 0 {
			t.Errorf("case %d: 'Java' inside identifier must not fire: got %d findings", i, len(findings))
		}
	}
}

func TestHasPlatformType_NegativeJavaxSubstringInIdentifier(t *testing.T) {
	// "Javax" appearing inside an identifier must not trigger.
	src := `
package test
fun render(html: String) = TemplateJavaxFooHelper.process(html)
`
	findings := runRuleByName(t, "HasPlatformType", src)
	if len(findings) != 0 {
		t.Errorf("'Javax' inside identifier must not fire: got %d findings", len(findings))
	}
}

func TestHasPlatformType_PositiveJavaIdentifierAtTokenStart(t *testing.T) {
	// "Java" at the start of an identifier still fires — this is the
	// existing positive case (JavaClass.getData()) and remains intact.
	src := `
package test
fun load() = JavaSerializer.from(input)
`
	findings := runRuleByName(t, "HasPlatformType", src)
	if len(findings) == 0 {
		t.Fatal("expected finding for 'JavaSerializer' at token start")
	}
}

func TestHasPlatformType_NegativeStringsScopeFunctionsAndKotlinConstructors(t *testing.T) {
	cases := []string{
		`package test
fun label(name: String) = "java.util.$name"`,
		`package test
fun describe(id: Int) = buildString { append("android.") ; append(id) }`,
		`package test
fun configure(target: Target) = target.apply { java.run() }`,
		`package test
class JavaLikeConfig(val name: String)
fun defaults() = JavaLikeConfig("x")`,
	}
	for i, src := range cases {
		findings := runRuleByName(t, "HasPlatformType", src)
		if len(findings) != 0 {
			t.Errorf("case %d: expected no findings, got %d", i, len(findings))
		}
	}
}

// --- IgnoredReturnValue ---

func TestIgnoredReturnValue_Positive(t *testing.T) {
	findings := runRuleByName(t, "IgnoredReturnValue", `
package test
fun main() {
    val list = listOf(1, 2, 3)
    list.map { it * 2 }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for ignored return value of map")
	}
}

func TestIgnoredReturnValue_Negative(t *testing.T) {
	findings := runRuleByName(t, "IgnoredReturnValue", `
package test
fun main() {
    val list = listOf(1, 2, 3)
    val doubled = list.map { it * 2 }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestIgnoredReturnValue_NoResolverSkipsSideEffectFunctionalNames(t *testing.T) {
	findings := runRuleByName(t, "IgnoredReturnValue", `
package test
class EffectSink {
    fun map(block: (Int) -> Unit): Unit {
        block(1)
    }

    fun filter(predicate: (Int) -> Boolean): Unit {
        predicate(1)
    }
}

fun f(sink: EffectSink) {
    sink.map { println(it) }
    sink.filter { it > 0 }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for side-effect methods, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_NoResolverKeepsFunctionalPipelineFallback(t *testing.T) {
	findings := runRuleByName(t, "IgnoredReturnValue", `
package test
fun sequence(): Sequence<Int> = sequenceOf(1, 2, 3)

fun f(items: List<Int>) {
    items.map { it + 1 }
    sequence().filter { it > 1 }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected two findings for discarded functional pipelines, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleReturnTypes(t *testing.T) {
	cases := []struct {
		name     string
		callText string
		rt       *typeinfer.ResolvedType
	}{
		{"sequence", "sequence().map { it + 1 }", &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}},
		{"flow", "flow().map { it + 1 }", &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}},
		{"stateFlow", "stateFlow().map { it + 1 }", &typeinfer.ResolvedType{Name: "StateFlow", FQN: "kotlinx.coroutines.flow.StateFlow", Kind: typeinfer.TypeClass}},
		{"stream", "stream().map { it + 1 }", &typeinfer.ResolvedType{Name: "Stream", FQN: "java.util.stream.Stream", Kind: typeinfer.TypeClass}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := fmt.Sprintf(`package test
fun f() {
    %s
}
`, tc.callText)
			findings := runIgnoredReturnValueWithOracle(t, code, tc.callText, tc.rt, nil)
			if len(findings) != 1 {
				t.Fatalf("expected one finding for %s, got %d", tc.callText, len(findings))
			}
		})
	}
}

func TestIgnoredReturnValue_OracleSkipsFunctionValueInvoke(t *testing.T) {
	fnType := &typeinfer.ResolvedType{Name: "Function1", FQN: "kotlin.Function1", Kind: typeinfer.TypeFunction}
	cases := []struct {
		name     string
		code     string
		callText string
	}{
		{"constructor property", `package test
class Presenter(private val eventSink: (Int) -> Unit) {
    fun go() {
        eventSink(1)
    }
}
`, "eventSink(1)"},
		{"parameter", `package test
fun go(eventSink: (Int) -> Unit) {
    eventSink(1)
}
`, "eventSink(1)"},
		{"member property", `package test
class Presenter {
    var handler: suspend (Int) -> Unit = {}
    suspend fun go() {
        handler(1)
    }
}
`, "handler(1)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runIgnoredReturnValueWithOracle(t, tc.code, tc.callText, fnType, nil)
			if len(findings) != 0 {
				t.Fatalf("expected no findings for invoking a function value, got %v", findings)
			}
		})
	}
}

func TestIgnoredReturnValue_OracleSkipsLambdaReturningFromEnclosingFunction(t *testing.T) {
	cases := []struct {
		name string
		code string
		rt   *typeinfer.ResolvedType
	}{
		{"let with return", `package test
fun f(x: String?): Int {
    x?.let { return it.length }
    return 0
}
`, &typeinfer.ResolvedType{Name: "Nothing?", FQN: "kotlin.Nothing?", Kind: typeinfer.TypeClass}},
		{"sequence map with return", `package test
fun f(x: List<Int>?): Int {
    x?.asSequence()?.map { return it }
    return 0
}
`, &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runIgnoredReturnValueWithOracle(t, tc.code, "x?.", tc.rt, nil)
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %v", findings)
			}
		})
	}
}

func TestIgnoredReturnValue_OracleStillFlagsLocalLabeledReturn(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
fun f(x: List<Int>) {
    x.asSequence().map { return@map it + 1 }
}
`, "x.asSequence().map", &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}, nil)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for a local labeled return, got %d", len(findings))
	}
}

func TestIgnoredReturnValue_OracleSkipsInPlaceMutators(t *testing.T) {
	cases := []struct {
		name     string
		callText string
	}{
		{"sort", "items.sort()"},
		{"shuffle", "items.shuffle()"},
		{"reverse", "items.reverse()"},
		{"fill", "items.fill(0)"},
		{"clear", "items.clear()"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := fmt.Sprintf(`package test
fun f(items: MutableList<Int>) {
    %s
}
`, tc.callText)
			findings := runIgnoredReturnValueWithOracle(t, code, tc.callText,
				&typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}, nil)
			if len(findings) != 0 {
				t.Fatalf("expected no findings for in-place mutator %s, got %d: %#v", tc.callText, len(findings), findings)
			}
		})
	}
}

func TestIgnoredReturnValue_OracleSkipsUnitNothingAndUsedExpressions(t *testing.T) {
	cases := []struct {
		name     string
		code     string
		callText string
		rt       *typeinfer.ResolvedType
	}{
		{"unit", `package test
fun log(): Unit = Unit
fun f() { log() }`, "log()", &typeinfer.ResolvedType{Name: "Unit", FQN: "kotlin.Unit", Kind: typeinfer.TypeUnit}},
		{"nothing", `package test
fun fail(): Nothing = error("x")
fun f() { fail() }`, "fail()", &typeinfer.ResolvedType{Name: "Nothing", FQN: "kotlin.Nothing", Kind: typeinfer.TypeNothing}},
		{"unitFunctionalName", `package test
class Logger { fun map(block: () -> Unit): Unit = Unit }
fun f(logger: Logger) { logger.map { } }`, "logger.map { }", &typeinfer.ResolvedType{Name: "Unit", FQN: "kotlin.Unit", Kind: typeinfer.TypeUnit}},
		{"nonMustUseFunctionalName", `package test
fun f(list: List<Int>) { list.map { it + 1 } }`, "list.map { it + 1 }", &typeinfer.ResolvedType{Name: "List", FQN: "kotlin.collections.List", Kind: typeinfer.TypeClass}},
		{"assignment", `package test
fun sequence(): Sequence<Int> = sequenceOf(1)
fun f() { val xs = sequence() }`, "sequence()", &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}},
		{"return", `package test
fun sequence(): Sequence<Int> = sequenceOf(1)
fun f(): Sequence<Int> { return sequence() }`, "sequence()", &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}},
		{"lambdaResult", `package test
fun sequence(): Sequence<Int> = sequenceOf(1)
fun f(): Sequence<Int> = run { sequence() }`, "sequence()", &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runIgnoredReturnValueWithOracle(t, tc.code, tc.callText, tc.rt, nil)
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestIgnoredReturnValue_OracleSkipsMockitoVerifyContext(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
import org.mockito.kotlin.verify

class Dao {
    fun selectOrderByKey(id: Long): Flow<String> = TODO()
}

fun f(dao: Dao) {
    verify(dao).selectOrderByKey(1)
}
`, "selectOrderByKey(1)", &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}, nil)
	if len(findings) != 0 {
		t.Fatalf("expected Mockito verify context to be ignored, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsRxEmitterSignals(t *testing.T) {
	cases := []struct {
		name     string
		callText string
	}{
		{"onNext", "emitter.onNext(value)"},
		{"onError", "emitter.onError(error)"},
		{"onComplete", "emitter.onComplete()"},
		{"sourceOnNext", "source.onNext(value)"},
		{"subjectOnError", "subject.onError(error)"},
		{"observerOnNext", "observer.onNext(value)"},
		{"observerOnError", "testObserver.onError(error)"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runIgnoredReturnValueWithOracle(t, fmt.Sprintf(`package test
class Emitter<T> {
    fun onNext(value: T): Unit = Unit
    fun onError(error: Throwable): Unit = Unit
    fun onComplete(): Unit = Unit
}

fun f(emitter: Emitter<String>, source: Emitter<String>, subject: Emitter<String>, observer: Emitter<String>, testObserver: Emitter<String>, value: String, error: Throwable) {
    %s
}
`, tc.callText), tc.callText, &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}, nil)
			if len(findings) != 0 {
				t.Fatalf("expected Rx emitter signal %s to be ignored, got %d: %#v", tc.callText, len(findings), findings)
			}
		})
	}
}

func TestIgnoredReturnValue_OracleSkipsDisposeTeardown(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
class Disposable {
    fun dispose(): Unit = Unit
}

fun f(disposable: Disposable) {
    disposable.dispose()
}
`, "disposable.dispose()", &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}, nil)
	if len(findings) != 0 {
		t.Fatalf("expected dispose teardown call to be ignored, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsRxJavaSubscribeWithHandlers(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
class Observable<T> {
    fun subscribe(onNext: (T) -> Unit, onError: (Throwable) -> Unit): Disposable = Disposable()
}
class Disposable

fun f(observable: Observable<String>) {
    observable.subscribe({ value -> println(value) }) { error -> println(error) }
}
`, "observable.subscribe",
		&typeinfer.ResolvedType{Name: "Disposable", FQN: "io.reactivex.rxjava3.disposables.Disposable", Kind: typeinfer.TypeClass}, []string{"CheckReturnValue"})
	if len(findings) != 0 {
		t.Fatalf("expected RxJava subscribe handlers to be ignored, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleFlagsRxJavaSubscribeWithoutHandlers(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
class Observable<T> {
    fun subscribe(): Disposable = Disposable()
}
class Disposable

fun f(observable: Observable<String>) {
    observable.subscribe()
}
`, "observable.subscribe()",
		&typeinfer.ResolvedType{Name: "Disposable", FQN: "io.reactivex.rxjava3.disposables.Disposable", Kind: typeinfer.TypeClass}, []string{"CheckReturnValue"})
	if len(findings) == 0 {
		t.Fatal("expected RxJava subscribe without handlers to remain reportable")
	}
}

func TestIgnoredReturnValue_OracleSkipsSideEffectResultFold(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
class Logger {
    fun error(error: Throwable): Unit = Unit
}

fun f(result: Result<String>, logger: Logger) {
    var rendered = ""
    result.fold(
        onSuccess = { value -> rendered = value },
        onFailure = { error -> logger.error(error) },
    )
}
`, "result.fold(\n        onSuccess = { value -> rendered = value },\n        onFailure = { error -> logger.error(error) },\n    )",
		&typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}, nil)
	if len(findings) != 0 {
		t.Fatalf("expected side-effect Result.fold dispatch to be ignored, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsSideEffectResultOnSuccess(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
sealed class SyncOutcome {
    object Merged : SyncOutcome()
    object Fresh : SyncOutcome()
}

fun recordOutcome(outcome: SyncOutcome): Unit = Unit
fun refreshRow(outcome: SyncOutcome): Unit = Unit

fun f(result: Result<SyncOutcome>) {
    result.onSuccess { outcome ->
        when (outcome) {
            is SyncOutcome.Merged -> recordOutcome(outcome)
            is SyncOutcome.Fresh -> refreshRow(outcome)
        }
    }
}
`, "result.onSuccess",
		&typeinfer.ResolvedType{Name: "Result", FQN: "kotlin.Result", Kind: typeinfer.TypeClass}, []string{"CheckReturnValue"})
	if len(findings) != 0 {
		t.Fatalf("expected side-effect Result.onSuccess dispatch to be ignored, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsSideEffectResultOnFailure(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
class Logger {
    fun error(error: Throwable): Unit = Unit
}

fun f(result: Result<String>, logger: Logger) {
    result.onFailure { error ->
        logger.error(error)
    }
}
`, "result.onFailure",
		&typeinfer.ResolvedType{Name: "Result", FQN: "kotlin.Result", Kind: typeinfer.TypeClass}, []string{"CheckReturnValue"})
	if len(findings) != 0 {
		t.Fatalf("expected side-effect Result.onFailure dispatch to be ignored, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsFlowBuilderResultHandlers(t *testing.T) {
	cases := []struct {
		name     string
		callText string
		code     string
	}{
		{
			name:     "onSuccess",
			callText: "result.onSuccess",
			code: `package test
class Flow<T>
class FlowCollector<T> {
    fun emit(value: T): Unit = Unit
}
fun <T> flow(block: FlowCollector<T>.() -> Unit): Flow<T> = Flow()
class Result<T>

fun f(result: Result<String>): Flow<String> {
    return flow {
        result.onSuccess { value ->
            emit(value)
        }
    }
}
`,
		},
		{
			name:     "onFailure",
			callText: "result.onFailure",
			code: `package test
class Flow<T>
class FlowCollector<T> {
    fun emit(value: T): Unit = Unit
}
fun <T> channelFlow(block: FlowCollector<T>.() -> Unit): Flow<T> = Flow()
class Result<T>

fun f(result: Result<String>): Flow<String> {
    return channelFlow {
        result.onFailure { error ->
            emit(error.message.orEmpty())
        }
    }
}
`,
		},
		{
			name:     "onError",
			callText: "result.onError",
			code: `package test
class Flow<T>
class FlowCollector<T> {
    fun emit(value: T): Unit = Unit
}
fun <T> callbackFlow(block: FlowCollector<T>.() -> Unit): Flow<T> = Flow()
class Result<T>

fun f(result: Result<String>): Flow<String> {
    return callbackFlow {
        result.onError { error ->
            emit(error.message.orEmpty())
        }
    }
}
`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runIgnoredReturnValueWithOracle(t, tc.code, tc.callText,
				&typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}, nil)
			if len(findings) != 0 {
				t.Fatalf("expected Flow-builder Result handler to be ignored, got %d: %#v", len(findings), findings)
			}
		})
	}
}

func TestIgnoredReturnValue_OracleFlagsFlowReturningResultHandlerOutsideFlowBuilder(t *testing.T) {
	findings := runIgnoredReturnValueWithOracleEvidence(t, `package test
class Result<T>

fun f(result: Result<String>) {
    result.onSuccess { value ->
        println(value)
    }
}
`, oracleCallEvidence{CallText: "result.onSuccess", CallTarget: "kotlin.Result.onSuccess", ReturnType: &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}})
	if len(findings) == 0 {
		t.Fatal("expected Flow-returning Result handler outside a Flow builder to remain flagged")
	}
}

func TestIgnoredReturnValue_OracleFlagsPureFlowBuilderResultHandler(t *testing.T) {
	findings := runIgnoredReturnValueWithOracleEvidence(t, `package test
class Flow<T>
class FlowCollector<T> {
    fun emit(value: T): Unit = Unit
}
fun <T> flow(block: FlowCollector<T>.() -> Unit): Flow<T> = Flow()
class Result<T>

fun f(result: Result<String>): Flow<String> {
    return flow {
        result.onSuccess { value ->
            value.length
        }
        emit("done")
    }
}
`, oracleCallEvidence{CallText: "result.onSuccess", CallTarget: "kotlin.Result.onSuccess", ReturnType: &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}})
	if len(findings) == 0 {
		t.Fatal("expected pure Flow-builder Result handler to remain flagged")
	}
}

func TestIgnoredReturnValue_OracleSkipsResultAccessorsUsedInWhenCondition(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
class Logger {
    fun log(message: String): Unit = Unit
}

fun f(result: Result<String>, logger: Logger) {
    when {
        result.isSuccess -> logger.log(result.getOrDefault(""))
        result.exceptionOrNull()?.message == "rate_limited" -> logger.log("rate_limited")
        else -> logger.log("other")
    }
}
`, "result.exceptionOrNull()",
		&typeinfer.ResolvedType{Name: "Throwable", FQN: "kotlin.Throwable", Kind: typeinfer.TypeClass}, []string{"CheckReturnValue"})
	if len(findings) != 0 {
		t.Fatalf("expected Result accessors used in when conditions to be ignored, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsSystemUnderTestSetupCallWithAssertion(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
annotation class Test
class InvoiceManager {
    fun render(customer: String): Draft = Draft()
}
class Draft
fun assertThat(value: Any): Subject = Subject()
class Subject {
    fun isNotNull(): Unit = Unit
}
class InvoiceManagerTest {
    private val invoiceManager = InvoiceManager()

    @Test
    fun renderUpdatesTotals() {
        invoiceManager.render("acme")
        assertThat(invoiceManager).isNotNull()
    }
}
`, `invoiceManager.render("acme")`,
		&typeinfer.ResolvedType{Name: "Draft", FQN: "test.Draft", Kind: typeinfer.TypeClass}, []string{"CheckReturnValue"})
	if len(findings) != 0 {
		t.Fatalf("expected setup call on system under test to be ignored when the test asserts elsewhere, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsHelperSetupCallWithAssertion(t *testing.T) {
	t.Run("factory", func(t *testing.T) {
		findings := runIgnoredReturnValueWithOracle(t, `package test
annotation class Test
class ThemeSettingsFactory {
    fun create(): ThemeSettings = ThemeSettings()
}
class ThemeSettings
fun assertThat(value: Any): Subject = Subject()
class Subject {
    fun isNotNull(): Unit = Unit
}
class ThemeSettingsTest {
    private val themeSettingsFactory = ThemeSettingsFactory()

    @Test
    fun recreatesAfterDelete() {
        themeSettingsFactory.create()
        assertThat(themeSettingsFactory).isNotNull()
    }
}
`, `themeSettingsFactory.create()`,
			&typeinfer.ResolvedType{Name: "ThemeSettings", FQN: "test.ThemeSettings", Kind: typeinfer.TypeClass}, []string{"CheckReturnValue"})
		if len(findings) != 0 {
			t.Fatalf("expected factory setup call with later assertion to be ignored, got %d: %#v", len(findings), findings)
		}
	})
	t.Run("helper", func(t *testing.T) {
		findings := runIgnoredReturnValueWithOracle(t, `package test
annotation class Test
class LedgerHelper {
    fun load(): Draft = Draft()
}
class Draft
fun assertThat(value: Any): Subject = Subject()
class Subject {
    fun isNotNull(): Unit = Unit
}
class LedgerHelperTest {
    private val ledgerHelper = LedgerHelper()

    @Test
    fun loadsBeforeCheck() {
        ledgerHelper.load()
        assertThat(ledgerHelper).isNotNull()
    }
}
`, `ledgerHelper.load()`,
			&typeinfer.ResolvedType{Name: "Draft", FQN: "test.Draft", Kind: typeinfer.TypeClass}, []string{"CheckReturnValue"})
		if len(findings) != 0 {
			t.Fatalf("expected helper setup call with later assertion to be ignored, got %d: %#v", len(findings), findings)
		}
	})
}

func TestIgnoredReturnValue_OracleFlagsSystemUnderTestSetupCallWithoutAssertion(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
annotation class Test
class InvoiceManager {
    fun render(customer: String): Draft = Draft()
}
class Draft
class InvoiceManagerTest {
    private val invoiceManager = InvoiceManager()

    @Test
    fun renderUpdatesTotals() {
        invoiceManager.render("acme")
    }
}
`, `invoiceManager.render("acme")`,
		&typeinfer.ResolvedType{Name: "Draft", FQN: "test.Draft", Kind: typeinfer.TypeClass}, []string{"CheckReturnValue"})
	if len(findings) == 0 {
		t.Fatal("expected setup-looking call without any assertion to remain flagged")
	}
}

func TestIgnoredReturnValue_OracleFlagsFunctionalCallInTestEvenWithAssertion(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
annotation class Test
fun assertThat(value: Any): Subject = Subject()
class Subject {
    fun isNotNull(): Unit = Unit
}
class InvoiceManagerTest {
    @Test
    fun mapsButDropsResult() {
        val values = listOf(1, 2, 3)
        values.map { it * 2 }
        assertThat(values).isNotNull()
    }
}
`, `values.map { it * 2 }`,
		&typeinfer.ResolvedType{Name: "List", FQN: "kotlin.collections.List", Kind: typeinfer.TypeClass}, []string{"CheckReturnValue"})
	if len(findings) == 0 {
		t.Fatal("expected ordinary functional calls in tests to remain flagged")
	}
}

func TestIgnoredReturnValue_OracleFlagsPureResultOnSuccess(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
fun f(result: Result<String>) {
    result.onSuccess { value ->
        value.length
    }
}
`, "result.onSuccess",
		&typeinfer.ResolvedType{Name: "Result", FQN: "kotlin.Result", Kind: typeinfer.TypeClass}, []string{"CheckReturnValue"})
	if len(findings) == 0 {
		t.Fatal("expected discarded pure Result.onSuccess handler to be flagged")
	}
}

func TestIgnoredReturnValue_OracleSkipsBuilderCallbackLambdaResult(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
class Flow<T>
class Result {
    fun events(): Flow<String> = Flow()
}
class ImageRequestBuilder {
    fun onSuccess(block: (Result) -> Unit): ImageRequestBuilder = this
}

fun f(builder: ImageRequestBuilder) {
    builder.onSuccess { result ->
        result.events()
    }
}
`, "result.events()", &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}, nil)
	if len(findings) != 0 {
		t.Fatalf("expected builder callback lambda result to be ignored, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsBuilderConstructorCallbackResult(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
class ImageResult
class ImageRequestBuilder {
    fun listener(onSuccess: (ImageResult) -> Unit): ImageRequestBuilder = this
}

fun f() {
    ImageRequestBuilder().listener(onSuccess = { result ->
        println(result)
    })
}
`, "ImageRequestBuilder().listener", &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}, nil)
	if len(findings) != 0 {
		t.Fatalf("expected builder constructor callback result to be ignored, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleFlagsNonBuilderCallbackLookalike(t *testing.T) {
	findings := runIgnoredReturnValueWithOracleEvidence(t, `package test
class Result {
    fun onSuccess(block: () -> Unit): Result = this
}

fun f(result: Result) {
    result.onSuccess {
        println("done")
    }
}
`, oracleCallEvidence{CallText: "result.onSuccess", CallTarget: "kotlin.Result.onSuccess", ReturnType: &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}})
	if len(findings) == 0 {
		t.Fatal("expected non-builder onSuccess lookalike to remain flagged")
	}
}

func TestIgnoredReturnValue_OracleFlagsPureResultFold(t *testing.T) {
	callText := "result.fold(\n        onSuccess = { value -> value },\n        onFailure = { error -> error.message.orEmpty() },\n    )"
	findings := runIgnoredReturnValueWithOracle(t, `package test
fun f(result: Result<String>) {
    result.fold(
        onSuccess = { value -> value },
        onFailure = { error -> error.message.orEmpty() },
    )
}
`, callText, &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}, nil)
	if len(findings) == 0 {
		t.Fatal("expected discarded pure Result.fold result to be flagged")
	}
}

func TestIgnoredReturnValue_OracleSkipsStateFlowValueAssignmentReceiver(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
import kotlinx.coroutines.flow.MutableStateFlow

fun f() {
    MutableStateFlow(false).value = true
}
`, "MutableStateFlow(false)", &typeinfer.ResolvedType{Name: "MutableStateFlow", FQN: "kotlinx.coroutines.flow.MutableStateFlow", Kind: typeinfer.TypeClass}, nil)
	if len(findings) != 0 {
		t.Fatalf("expected MutableStateFlow value assignment receiver to be ignored, got %d: %#v", len(findings), findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsFlowFactoryWhenBranchReturns(t *testing.T) {
	cases := []struct {
		name     string
		callText string
	}{
		{"emptyFlow", "emptyFlow()"},
		{"flowOf", "flowOf(\"ready\")"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runIgnoredReturnValueWithOracle(t, `package test
class Flow<T>
fun <T> emptyFlow(): Flow<T> = Flow()
fun <T> flowOf(value: T): Flow<T> = Flow()

fun f(enabled: Boolean): Flow<String> {
    return when (enabled) {
        true -> flowOf("ready")
        false -> emptyFlow()
    }
}
`, tc.callText, &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}, nil)
			if len(findings) != 0 {
				t.Fatalf("expected Flow factory used as when-branch return to be ignored, got %d: %#v", len(findings), findings)
			}
		})
	}
}

func TestIgnoredReturnValue_OracleFlagsDroppedFlowFactory(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
class Flow<T>
fun <T> emptyFlow(): Flow<T> = Flow()

fun f() {
    emptyFlow<String>()
}
`, "emptyFlow<String>()", &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}, nil)
	if len(findings) == 0 {
		t.Fatal("expected dropped Flow factory call to remain flagged")
	}
}

func TestIgnoredReturnValue_OracleAnnotations(t *testing.T) {
	checkReturn := []string{"com.google.errorprone.annotations.CheckReturnValue"}
	checkResult := []string{"androidx.annotation.CheckResult"}
	canIgnore := []string{"com.google.errorprone.annotations.CheckReturnValue", "com.google.errorprone.annotations.CanIgnoreReturnValue"}

	if findings := runIgnoredReturnValueWithOracle(t, `package test
fun buildToken(): String = "token"
fun f() { buildToken() }`, "buildToken()", &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypePrimitive}, checkReturn); len(findings) != 1 {
		t.Fatalf("expected @CheckReturnValue finding, got %d", len(findings))
	}
	if findings := runIgnoredReturnValueWithOracle(t, `package test
fun buildToken(): String = "token"
fun f() { buildToken() }`, "buildToken()", &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypePrimitive}, checkResult); len(findings) != 1 {
		t.Fatalf("expected @CheckResult finding, got %d", len(findings))
	}
	if findings := runIgnoredReturnValueWithOracle(t, `package test
fun maybeBuild(): String = "token"
fun f() { maybeBuild() }`, "maybeBuild()", &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypePrimitive}, canIgnore); len(findings) != 0 {
		t.Fatalf("expected @CanIgnoreReturnValue suppression, got %d", len(findings))
	}

	// Note: @CheckReturnValue / @CanIgnoreReturnValue on the *containing*
	// declaration is intentionally not consulted. That would require class-level
	// annotations (NeedsOracleClassAnnotations), which this rule does not
	// request, so the lookup returned nothing in production. See
	// registry_potentialbugs_misc.go.
}

type oracleCallEvidence struct {
	CallText    string
	CallTarget  string
	ReturnType  *typeinfer.ResolvedType
	Annotations []string
}

func runIgnoredReturnValueWithOracle(t *testing.T, code, callText string, rt *typeinfer.ResolvedType, annotations []string) []scanner.Finding {
	return runIgnoredReturnValueWithOracleEvidence(t, code, oracleCallEvidence{
		CallText:    callText,
		ReturnType:  rt,
		Annotations: annotations,
	})
}

func runIgnoredReturnValueWithOracleEvidence(t *testing.T, code string, ev oracleCallEvidence) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{}
	fake.CallTargets[file.Path] = map[string]string{}
	fake.CallTargetAnnotations[file.Path] = map[string][]string{}
	var matched bool
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		text := strings.TrimSpace(file.FlatNodeText(idx))
		if !strings.Contains(text, ev.CallText) {
			return
		}
		key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
		if ev.ReturnType != nil {
			fake.Expressions[file.Path][key] = ev.ReturnType
		}
		if ev.CallTarget != "" {
			fake.CallTargets[file.Path][key] = ev.CallTarget
		}
		if len(ev.Annotations) > 0 {
			fake.CallTargetAnnotations[file.Path][key] = ev.Annotations
		}
		matched = true
	})
	if !matched {
		t.Fatalf("call %q not found", ev.CallText)
	}
	resolver := oracle.NewCompositeResolver(fake, typeinfer.NewFakeResolver())
	for _, r := range api.Registry {
		if r.ID == "IgnoredReturnValue" {
			d := rules.NewDispatcher([]*api.Rule{r}, resolver)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("IgnoredReturnValue rule not found")
	return nil
}

// --- ImplicitDefaultLocale ---

func TestImplicitDefaultLocale_Positive(t *testing.T) {
	findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
fun main() {
    val s = "Hello"
    s.toLowerCase()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for toLowerCase() without Locale")
	}
}

func TestImplicitDefaultLocale_Negative(t *testing.T) {
	findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
import java.util.Locale
fun main() {
    val s = "Hello"
    s.toLowerCase(Locale.ROOT)
    // Kotlin 1.5+ lowercase() is locale-invariant (Locale.ROOT) by design.
    s.lowercase()
    s.uppercase()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_FormatSpecifierPolicy(t *testing.T) {
	tests := []struct {
		name         string
		call         string
		wantFindings int
	}{
		{name: "integer digits", call: `String.format("%d", count)`, wantFindings: 1},
		{name: "grouped integer", call: `String.format("%,d", count)`, wantFindings: 1},
		{name: "floating point", call: `String.format("%.2f", value)`, wantFindings: 1},
		{name: "date time", call: `String.format("%tF", now)`, wantFindings: 1},
		{name: "string", call: `String.format("%s", name)`, wantFindings: 0},
		{name: "escaped percent", call: `String.format("progress %% done")`, wantFindings: 0},
		{name: "escaped percent before d", call: `String.format("%%d")`, wantFindings: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
fun main(now: Long, value: Double, count: Int, name: String) {
    `+tt.call+`
}
`)
			if len(findings) != tt.wantFindings {
				t.Fatalf("expected %d findings for %s, got %d", tt.wantFindings, tt.call, len(findings))
			}
		})
	}
}

func TestImplicitDefaultLocale_FormatConstSpecifierPolicy(t *testing.T) {
	findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
private const val NAME_TEMPLATE = "Name: %s"
private const val COUNT_TEMPLATE = "Count: %d"
fun main(count: Int, name: String) {
    String.format(NAME_TEMPLATE, name)
    String.format(COUNT_TEMPLATE, count)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for locale-sensitive const format specifier, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_FormatNonLiteralFormatString(t *testing.T) {
	findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
fun show(count: Int, template: String) {
    String.format(getString(R.string.items_count), count)
    String.format(stringResource(R.string.items_count), count)
    String.format(template, count)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for resource or parameter format strings, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_FormatPositiveSpecifiers(t *testing.T) {
	findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
fun main(now: Long, value: Double, count: Int) {
    String.format("Audio: %d Kbit/s", count)
    String.format("%,d", count)
    "%.2f".format(value)
    "%tF".format(now)
    "Timestamp: %d".format(now)
}
`)
	if len(findings) != 5 {
		t.Fatalf("expected 5 findings for locale-sensitive format specifiers, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_FormatNegativeSpecifiers(t *testing.T) {
	findings := runRuleByName(t, "ImplicitDefaultLocale", `
package test
import java.util.Locale
fun main(value: Double, count: Int, name: String) {
    String.format(Locale.US, "Audio: %d Kbit/s", count)
    "%.2f".format(Locale.US, value)
    String.format("%s", name)
    "progress %% done".format()
    "%%d".format()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for explicit Locale or locale-insensitive specifiers, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_OracleConfirmsStringFormatTargets(t *testing.T) {
	findings := runRuleByNameWithCallTarget(t, "ImplicitDefaultLocale", `
package test
fun main(count: Int) {
    String.format("Audio: %d Kbit/s", count)
}
`, `String.format("Audio: %d Kbit/s", count)`, "java.lang.String.format")
	if len(findings) != 1 {
		t.Fatalf("expected finding for resolved java.lang.String.format, got %d", len(findings))
	}

	findings = runRuleByNameWithCallTarget(t, "ImplicitDefaultLocale", `
package test
fun main(value: Double) {
    "%.2f".format(value)
}
`, `"%.2f".format(value)`, "kotlin.text.format")
	if len(findings) != 1 {
		t.Fatalf("expected finding for resolved kotlin.text.format, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_EmptyOracleFallsBackToAst(t *testing.T) {
	file := parseInline(t, `
package test
fun main(count: Int) {
    "%d".format(count)
}
`)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	composite := oracle.NewCompositeResolver(oracle.NewFakeOracle(), resolver)

	for _, r := range api.Registry {
		if r.ID == "ImplicitDefaultLocale" {
			dispatcher := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := dispatcher.Run(file)
			findings := cols.Findings()
			if len(findings) != 1 {
				t.Fatalf("expected AST fallback finding when oracle has no call target, got %d", len(findings))
			}
			return
		}
	}
	t.Fatal("rule \"ImplicitDefaultLocale\" not found in registry")
}

func TestImplicitDefaultLocale_LexicalOracleTargetFallsBackToAst(t *testing.T) {
	findings := runRuleByNameWithCallTarget(t, "ImplicitDefaultLocale", `
package test
fun main(count: Int) {
    "%d".format(count)
}
`, `"%d".format(count)`, "format")
	if len(findings) != 1 {
		t.Fatalf("expected AST fallback finding when oracle target is lexical fallback, got %d", len(findings))
	}
}

func TestImplicitDefaultLocale_OracleSuppressesNonStringFormatTargets(t *testing.T) {
	findings := runRuleByNameWithCallTarget(t, "ImplicitDefaultLocale", `
package test
fun kotlin.String.format(value: Double): kotlin.String = this
fun main(value: Double) {
    "%.2f".format(value)
}
`, `"%.2f".format(value)`, "test.format")
	if len(findings) != 0 {
		t.Fatalf("expected no findings for resolved user-defined format extension, got %d", len(findings))
	}
}

// --- LocaleDefaultForCurrency ---

func TestLocaleDefaultForCurrency_Positive(t *testing.T) {
	findings := runRuleByName(t, "LocaleDefaultForCurrency", `
package test

import java.util.Currency
import java.util.Locale

class PriceFormatter {
    private val currency = Currency.getInstance(Locale.getDefault())
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for currency derived from Locale.getDefault() in a price-related class")
	}
}

func TestLocaleDefaultForCurrency_Negative(t *testing.T) {
	findings := runRuleByName(t, "LocaleDefaultForCurrency", `
package test

import java.util.Currency
import java.util.Locale

class DisplayFormatter {
    private val currency = Currency.getInstance(Locale.getDefault())
}

class PriceFormatter(private val currencyCode: String) {
    private val currency = Currency.getInstance(currencyCode)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func BenchmarkIgnoredReturnValue_DiscardedMap(b *testing.B) {
	benchmarkRuleByName(b, "IgnoredReturnValue", `
package test
fun main() {
    val list = listOf(1, 2, 3)
    list.map { it * 2 }
}
`)
}

func BenchmarkImplicitDefaultLocale_FormatWithExplicitLocale(b *testing.B) {
	benchmarkRuleByName(b, "ImplicitDefaultLocale", `
package test
import java.util.Locale
fun main() {
    val s = "Hello"
    String.format(Locale.ROOT, "%s", s)
}
`)
}

// --- HardcodedDateFormat ---

func TestHardcodedDateFormat_FlagsLocaleSensitivePatterns(t *testing.T) {
	for _, pattern := range []string{"yyyy-MM-dd'T'HH:mm z", "w", "YYYY-'W'ww", "e"} {
		t.Run(pattern, func(t *testing.T) {
			findings := runRuleByName(t, "HardcodedDateFormat", `
package test
import java.time.format.DateTimeFormatter
val formatter = DateTimeFormatter.ofPattern("`+pattern+`")
`)
			if len(findings) != 1 {
				t.Fatalf("expected 1 finding for %q, got %d", pattern, len(findings))
			}
		})
	}
}

func TestHardcodedDateFormat_SuggestsDefaultLocaleForUI(t *testing.T) {
	ui := runRuleByName(t, "HardcodedDateFormat", `
package test
import java.time.format.DateTimeFormatter
class Row(private val label: TextView) {
    private val formatter = DateTimeFormatter.ofPattern("'Week' w")
    fun bind(d: LocalDate) { label.setText(formatter.format(d)) }
}
`)
	if len(ui) != 1 || !strings.Contains(ui[0].Message, "Locale.getDefault()") || strings.Contains(ui[0].Message, "Locale.ROOT") {
		t.Fatalf("expected a UI-oriented message, got %v", ui)
	}
	machine := runRuleByName(t, "HardcodedDateFormat", `
package test
import java.time.format.DateTimeFormatter
class Row {
    private val fileNameFormatter = DateTimeFormatter.ofPattern("EEE-MMM-d")
}
`)
	if len(machine) != 1 || !strings.Contains(machine[0].Message, "Locale.ROOT") {
		t.Fatalf("expected the machine-readable message, got %v", machine)
	}
}

func TestHardcodedDateFormat_WeekYearWithoutWeek(t *testing.T) {
	findings := runRuleByName(t, "HardcodedDateFormat", `
package test
import java.time.format.DateTimeFormatter
val a = DateTimeFormatter.ofPattern("YYYY-MM-dd")
val b = DateTimeFormatter.ofPattern("YYYY-'W'ww")
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "week-based year") {
		t.Errorf("first finding should mention the week-year problem: %s", findings[0].Message)
	}
	if strings.Contains(findings[1].Message, "week-based year") {
		t.Errorf("YYYY with ww is a valid week-year pattern: %s", findings[1].Message)
	}
}

func TestHardcodedDateFormat_IgnoresNumericPatterns(t *testing.T) {
	findings := runRuleByName(t, "HardcodedDateFormat", `
package test
import java.time.format.DateTimeFormatter
val a = DateTimeFormatter.ofPattern("yyyy-MM-dd")
val b = DateTimeFormatter.ofPattern("HH:mm:ss.SSS")
val c = DateTimeFormatter.ofPattern("yyyy-MM-dd'T'HH:mm:ssX")
val d = DateTimeFormatter.ofPattern("d/M/yy")
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for numeric patterns, got %d", len(findings))
	}
}

func TestHardcodedDateFormat_IgnoresExplicitLocaleChain(t *testing.T) {
	findings := runRuleByName(t, "HardcodedDateFormat", `
package test
import java.time.format.DateTimeFormatter
import java.util.Locale
val a = DateTimeFormatter.ofPattern("MMM d").withLocale(Locale.US)
val b = DateTimeFormatter.ofPattern("EEEE").localizedBy(Locale.US)
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when a locale is chained, got %d", len(findings))
	}
}

func TestHardcodedNumberFormat_SkipsDisplayFormatting(t *testing.T) {
	findings := runRuleByName(t, "HardcodedNumberFormat", `
package test

import java.text.DecimalFormat
import java.text.NumberFormat

class Labels {
    fun priceLabel(value: Double): String = NumberFormat.getInstance().format(value)

    fun percentLabel(value: Double): String = DecimalFormat("#.#").format(value)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for display formatting, got %d", len(findings))
	}
}

func TestHardcodedNumberFormat_FlagsFormatterFeedingCacheKey(t *testing.T) {
	findings := runRuleByName(t, "HardcodedNumberFormat", `
package test

import java.text.NumberFormat

class Cache {
    fun cacheKey(value: Double): String = "price-" + NumberFormat.getInstance().format(value)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestHardcodedNumberFormat_FollowsFormattedValueToSinks(t *testing.T) {
	cases := map[string]string{
		"local alias used as url argument": `
    fun build(value: Double): URL {
        val text = NumberFormat.getInstance().format(value)
        return URL("https://example.test/items/" + text)
    }`,
		"formatter variable result parsed back": `
    fun roundTrip(value: Double): Double {
        val nf = NumberFormat.getInstance()
        return nf.format(value).toDouble()
    }`,
		"assigned to a file name variable": `
    fun target(value: Double): File {
        val reportFileName = "report-${NumberFormat.getInstance().format(value)}.txt"
        return File(reportFileName)
    }`,
		"string passed to a query parameter": `
    fun uri(value: Double, builder: Uri.Builder) {
        builder.appendQueryParameter("amount", NumberFormat.getInstance().format(value))
    }`,
	}
	for name, body := range cases {
		findings := runRuleByName(t, "HardcodedNumberFormat", "package test\n\nimport java.text.NumberFormat\n\nclass Subject {"+body+"\n}\n")
		if len(findings) != 1 {
			t.Errorf("%s: expected 1 finding, got %d", name, len(findings))
		}
	}
}

func TestHardcodedNumberFormat_SilentWhenResultOnlyDisplayed(t *testing.T) {
	cases := map[string]string{
		"assigned to a label then shown": `
    fun show(value: Double, view: TextView) {
        val label = NumberFormat.getInstance().format(value)
        view.setText(label)
    }`,
		"function name mentions a key but value is not returned": `
    fun keyLabel(value: Double, view: TextView) {
        view.setText(NumberFormat.getInstance().format(value))
    }`,
		"result is discarded": `
    fun warm(value: Double) {
        NumberFormat.getInstance().format(value)
    }`,
		"formatter never formats": `
    fun create(): NumberFormat = NumberFormat.getInstance()`,
	}
	for name, body := range cases {
		findings := runRuleByName(t, "HardcodedNumberFormat", "package test\n\nimport java.text.NumberFormat\n\nclass Subject {"+body+"\n}\n")
		if len(findings) != 0 {
			t.Errorf("%s: expected no findings, got %d", name, len(findings))
		}
	}
}

func TestIgnoredReturnValue_OracleSkipsDetachedReturnOperand(t *testing.T) {
	flowType := &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}
	findings := runIgnoredReturnValueWithOracle(t, `package test
fun f(src: Source) = src.switchMap { ev ->
    if (ev == null) return@switchMap emptyFlow()
    known(ev)
}
`, "emptyFlow()", flowType, nil)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a returned operand, got %v", findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsFunctionValueInvokeWithReceiver(t *testing.T) {
	fnType := &typeinfer.ResolvedType{Name: "Function1", FQN: "kotlin.Function1", Kind: typeinfer.TypeFunction}
	for name, call := range map[string]string{
		"safe invoke":   "callback?.invoke(1)",
		"state handler": "state.eventSink(1)",
	} {
		t.Run(name, func(t *testing.T) {
			findings := runIgnoredReturnValueWithOracle(t, "package test\nfun f() {\n    "+call+"\n}\n", call, fnType, nil)
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %v", findings)
			}
		})
	}
}

func TestIgnoredReturnValue_OracleSkipsHelperWhoseResultIsUsedElsewhere(t *testing.T) {
	flowType := &typeinfer.ResolvedType{Name: "MutableStateFlow", FQN: "kotlinx.coroutines.flow.MutableStateFlow", Kind: typeinfer.TypeClass}
	findings := runIgnoredReturnValueWithOracle(t, `package test
private fun setup(id: Int): MutableStateFlow<Int> = MutableStateFlow(id)
fun a() {
    setup(1)
}
fun b() {
    val flow = setup(2)
}
`, "setup(1)", flowType, nil)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsSubscribeWith(t *testing.T) {
	findings := runIgnoredReturnValueWithOracle(t, `package test
fun f(obs: Observable<Int>, observer: Observer) {
    obs.subscribeWith(observer)
}
`, "subscribeWith(observer)", nil, []string{"io.reactivex.annotations.CheckReturnValue"})
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsBranchTailFollowedByComment(t *testing.T) {
	seq := &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}
	findings := runIgnoredReturnValueWithOracle(t, `package test
fun pick(items: List<Int>?): Sequence<Int> {
  return if (items == null) {
    emptySequence()
  } else {
    items.asSequence().flatMap { sequenceOf(it) }
    // stop once we have one
  }
}`, "items.asSequence()", seq, nil)
	if len(findings) != 0 {
		t.Fatalf("a branch tail followed by a comment is used as the result, got %d", len(findings))
	}
}

func TestIgnoredReturnValue_OracleStillFlagsDiscardedSequenceBeforeComment(t *testing.T) {
	seq := &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}
	findings := runIgnoredReturnValueWithOracle(t, `package test
fun pick(items: List<Int>) {
  items.asSequence().flatMap { sequenceOf(it) }
  // dropped on purpose?
  println("done")
}`, "items.asSequence()", seq, nil)
	if len(findings) == 0 {
		t.Fatalf("expected a discarded sequence statement to still be reported")
	}
}

func TestIgnoredReturnValue_OracleSkipsScopeFunctionWithSideEffectTail(t *testing.T) {
	flow := &typeinfer.ResolvedType{Name: "MutableStateFlow", FQN: "kotlinx.coroutines.flow.MutableStateFlow", Kind: typeinfer.TypeClass}
	for _, fn := range []string{"let", "run", "with"} {
		code := `package test
class FakeRepo {
  private val items = mutableMapOf<Int, MutableStateFlow<Int>>()
  fun add(id: Int, item: Int) {
    items.` + fn + ` { entry ->
      entry.compute(id) { _, value -> value?.also { it.value = item } ?: MutableStateFlow(item) }
    }
  }
}`
		findings := runIgnoredReturnValueWithOracle(t, code, "items."+fn, flow, nil)
		if len(findings) != 0 {
			t.Fatalf("%s: a scope function used for side effects is not an ignored result, got %d", fn, len(findings))
		}
	}
}

func TestIgnoredReturnValue_OracleSkipsTestAssertionLibraries(t *testing.T) {
	fnType := &typeinfer.ResolvedType{Name: "Function0", FQN: "kotlin.Function0", Kind: typeinfer.TypeFunction}
	flowType := &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}
	cases := []struct {
		name   string
		call   string
		target string
		rt     *typeinfer.ResolvedType
	}{
		{"kotlin.test generic result is a function", "assertNotNull(callback)", "kotlin.test.assertNotNull", fnType},
		{"kotlin.test generic result is a flow", "assertNotNull(spans)", "kotlin.test.assertNotNull", flowType},
		{"junit", "assertNotNull(spans)", "org.junit.Assert.assertNotNull", flowType},
		{"truth", "assertThat(spans)", "com.google.common.truth.Truth.assertThat", flowType},
		{"unresolved target", "assertNotNull(spans)", "", flowType},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code := fmt.Sprintf("package test\nfun f() {\n    %s\n}\n", tc.call)
			findings := runIgnoredReturnValueWithOracleEvidence(t, code, oracleCallEvidence{CallText: tc.call, ReturnType: tc.rt, CallTarget: tc.target})
			if len(findings) != 0 {
				t.Fatalf("assertion call must not be reported, got %v", findings)
			}
		})
	}
}

func TestIgnoredReturnValue_OracleSkipsInvokeAfterAssertOnNextLine(t *testing.T) {
	code := `package test
fun go(state: Any) {
    assert(state is String)
    (state as Holder)
        .eventSink
        .invoke(Event.Clicked)
}
`
	flow := &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}
	findings := runIgnoredReturnValueWithOracleEvidence(t, code, oracleCallEvidence{
		CallText: "invoke", ReturnType: flow, CallTarget: "kotlin.Function1.invoke",
	})
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %v", findings)
	}
}

func TestIgnoredReturnValue_OracleSkipsCallThatIsAssignmentQualifier(t *testing.T) {
	flowType := &typeinfer.ResolvedType{Name: "MutableStateFlow", FQN: "kotlinx.coroutines.flow.MutableStateFlow", Kind: typeinfer.TypeClass}
	cases := []struct {
		name string
		code string
	}{
		{"valueInLambda", `package test
fun f(holders: Map<String, Holder>) {
    run {
        holders.getValue("a").current = null
        println()
    }
}
`},
		{"valueInFunction", `package test
fun f(holders: Map<String, Holder>) {
    holders.getValue("a").current = null
}
`},
		{"indexedAssignment", `package test
fun f(holders: Map<String, Holder>) {
    holders.getValue("a")[0] = null
}
`},
		{"augmentedAssignment", `package test
fun f(holders: Map<String, Holder>) {
    holders.getValue("a").count += 1
}
`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runIgnoredReturnValueWithOracleEvidence(t, tc.code, oracleCallEvidence{CallText: `getValue("a")`, ReturnType: flowType, CallTarget: "kotlin.collections.getValue"})
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d: %s", len(findings), findings[0].Message)
			}
		})
	}
}

func TestIgnoredReturnValue_OracleStillFlagsStatementCall(t *testing.T) {
	flowType := &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}
	findings := runIgnoredReturnValueWithOracleEvidence(t, `package test
fun f(holders: Map<String, Holder>) {
    holders.getValue("a")
}
`, oracleCallEvidence{CallText: `getValue("a")`, ReturnType: flowType, CallTarget: "kotlin.collections.getValue"})
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(findings))
	}
}

func TestIgnoredReturnValue_HonoursCheckResultSuppression(t *testing.T) {
	flowType := &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass}
	cases := map[string]struct {
		code string
		want int
	}{
		"unsuppressed":          {"package test\nfun f(s: Source) {\n    s.items()\n}\n", 1},
		"unrelatedSuppress":     {"package test\n@Suppress(\"Other\")\nfun f(s: Source) {\n    s.items()\n}\n", 1},
		"suppressLintFunction":  {"package test\n@SuppressLint(\"CheckResult\")\nfun f(s: Source) {\n    s.items()\n}\n", 0},
		"suppressWarningsClass": {"package test\n@SuppressWarnings(\"CheckResult\")\nclass C {\n    fun f(s: Source) {\n        s.items()\n    }\n}\n", 0},
		"suppressFunction":      {"package test\n@Suppress(\"CheckResult\")\nfun f(s: Source) {\n    s.items()\n}\n", 0},
		"suppressFile":          {"@file:Suppress(\"CheckResult\")\npackage test\nfun f(s: Source) {\n    s.items()\n}\n", 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			file := parseInline(t, tc.code)
			fake := oracle.NewFakeOracle()
			fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{}
			fake.CallTargets[file.Path] = map[string]string{}
			file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
				key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
				fake.Expressions[file.Path][key] = flowType
				fake.CallTargets[file.Path][key] = "kotlin.sequences.map"
			})
			resolver := oracle.NewCompositeResolver(fake, typeinfer.NewFakeResolver())
			got := 0
			for _, r := range api.Registry {
				if r.ID != "IgnoredReturnValue" {
					continue
				}
				filter := scanner.BuildSuppressionFilter(file, nil, nil, "").WithRuleAliases(rules.AllSuppressionAliases())
				cols := rules.NewDispatcher([]*api.Rule{r}, resolver).Run(file)
				for _, f := range cols.Findings() {
					if !filter.IsSuppressed(f.Rule, f.RuleSet, f.Line) {
						got++
					}
				}
			}
			if got != tc.want {
				t.Fatalf("got %d findings, want %d", got, tc.want)
			}
		})
	}
}

func TestIgnoredReturnValue_SubscribeOverloadFollowsResolvedSymbol(t *testing.T) {
	code := `package test
fun f(source: Observable<String>, sink: Sink<String>) {
    source.subscribe(sink)
}
`
	// A Consumer-typed argument selects the Disposable overload, which carries the must-use annotation.
	consumer := runIgnoredReturnValueWithOracleEvidence(t, code, oracleCallEvidence{
		CallText:    "source.subscribe(sink)",
		ReturnType:  &typeinfer.ResolvedType{Name: "Disposable", FQN: "io.reactivex.rxjava3.disposables.Disposable", Kind: typeinfer.TypeClass},
		CallTarget:  "io.reactivex.rxjava3.core.Observable.subscribe",
		Annotations: []string{"io.reactivex.rxjava3.annotations.CheckReturnValue"},
	})
	if len(consumer) != 1 {
		t.Fatalf("expected the Consumer overload to be reported, got %d", len(consumer))
	}
	// An Observer-typed argument selects the void overload, which has no result to drop.
	observer := runIgnoredReturnValueWithOracleEvidence(t, code, oracleCallEvidence{
		CallText:   "source.subscribe(sink)",
		ReturnType: &typeinfer.ResolvedType{Name: "Unit", FQN: "kotlin.Unit", Kind: typeinfer.TypeClass},
		CallTarget: "io.reactivex.rxjava3.core.Observable.subscribe",
	})
	if len(observer) != 0 {
		t.Fatalf("expected the Observer overload to be skipped, got %d", len(observer))
	}
}

func TestIgnoredReturnValue_OracleSkipsFunctionPropertyInvokeOnOtherObject(t *testing.T) {
	code := `package test
class FakeRepo { var latest: (String) -> kotlinx.coroutines.flow.Flow<Boolean> = { error("unset") } }
fun f(repo: FakeRepo) {
    repo.latest("a")
}
`
	findings := runIgnoredReturnValueWithOracleEvidence(t, code, oracleCallEvidence{
		CallText:   `repo.latest("a")`,
		ReturnType: &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass},
		CallTarget: "kotlin.Function1.invoke",
	})
	if len(findings) != 0 {
		t.Fatalf("invoking a function-typed property must not be reported as a pure stdlib call, got %v", findings)
	}
}
