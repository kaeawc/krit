package rules_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

// --- CollectInOnCreateWithoutLifecycle ---

func TestCollectInOnCreateWithoutLifecycle_PositiveOnCreate(t *testing.T) {
	findings := runRuleByName(t, "CollectInOnCreateWithoutLifecycle", `
package test
import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope

class ExampleActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        lifecycleScope.launch {
            vm.state.collect { render(it) }
        }
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestCollectInOnCreateWithoutLifecycle_PositiveOnViewCreated(t *testing.T) {
	findings := runRuleByName(t, "CollectInOnCreateWithoutLifecycle", `
package test
import android.os.Bundle
import android.view.View
import androidx.fragment.app.Fragment
import androidx.lifecycle.lifecycleScope

class ExampleFragment : Fragment() {
    override fun onViewCreated(view: View, savedInstanceState: Bundle?) {
        super.onViewCreated(view, savedInstanceState)
        lifecycleScope.launch {
            vm.state.collect { render(it) }
        }
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestCollectInOnCreateWithoutLifecycle_NegativeWithRepeatOnLifecycle(t *testing.T) {
	findings := runRuleByName(t, "CollectInOnCreateWithoutLifecycle", `
package test
import android.os.Bundle
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle

class ExampleActivity : AppCompatActivity() {
    override fun onStart() {
        super.onStart()
        lifecycleScope.launch {
            repeatOnLifecycle(Lifecycle.State.STARTED) {
                vm.state.collect { render(it) }
            }
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestCollectInOnCreateWithoutLifecycle_NegativeOutsideLifecycleCallback(t *testing.T) {
	findings := runRuleByName(t, "CollectInOnCreateWithoutLifecycle", `
package test
import androidx.lifecycle.lifecycleScope

class ExampleActivity {
    fun observe() {
        lifecycleScope.launch {
            vm.state.collect { render(it) }
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// --- GlobalCoroutineUsage ---

func TestGlobalCoroutineUsage_Positive(t *testing.T) {
	findings := runRuleByName(t, "GlobalCoroutineUsage", `
package test
import kotlinx.coroutines.GlobalScope
fun start() {
    GlobalScope.launch {
        doWork()
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected GlobalCoroutineUsage to flag GlobalScope.launch")
	}
}

func TestGlobalCoroutineUsage_Negative(t *testing.T) {
	findings := runRuleByName(t, "GlobalCoroutineUsage", `
package test
import kotlinx.coroutines.CoroutineScope
class MyClass(private val scope: CoroutineScope) {
    fun start() {
        scope.launch {
            doWork()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// --- RedundantSuspendModifier ---

func TestRedundantSuspendModifier_Positive(t *testing.T) {
	findings := runRuleByName(t, "RedundantSuspendModifier", `
package test
suspend fun doNothing(): Int {
    return 42
}
`)
	if len(findings) == 0 {
		t.Error("expected RedundantSuspendModifier to flag suspend fun with no suspend calls")
	}
}

func TestRedundantSuspendModifier_Negative(t *testing.T) {
	findings := runRuleByName(t, "RedundantSuspendModifier", `
package test
suspend fun doWork() {
    delay(1000)
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestRedundantSuspendModifier_ProjectNonSuspendCallWithOracle(t *testing.T) {
	findings := runRedundantSuspendModifierWithCallTargetSuspend(t, `
package test
fun helper() = Unit

suspend fun redundant() {
    helper()
}
`, map[string]bool{"helper()": false})
	if len(findings) == 0 {
		t.Fatal("expected redundant suspend finding when project helper resolves non-suspend")
	}
}

func TestRedundantSuspendModifier_ProjectSuspendCallWithOracle(t *testing.T) {
	findings := runRedundantSuspendModifierWithCallTargetSuspend(t, `
package test
open class Service {
    open suspend fun projectSuspend() = Unit
}

suspend fun needed(service: Service) {
    service.projectSuspend()
}
`, map[string]bool{"service.projectSuspend()": true})
	if len(findings) != 0 {
		t.Fatalf("expected project suspend call to suppress finding, got %d: %v", len(findings), findings)
	}
}

func TestRedundantSuspendModifier_UnresolvedProjectCallStaysConservative(t *testing.T) {
	findings := runRedundantSuspendModifierWithCallTargetSuspend(t, `
package test
fun helper() = Unit

suspend fun maybeNeeded() {
    helper()
}
`, nil)
	if len(findings) != 0 {
		t.Fatalf("expected unresolved project call to suppress finding, got %d: %v", len(findings), findings)
	}
}

func TestRedundantSuspendModifier_OpenOverrideAbstractStaySuppressed(t *testing.T) {
	findings := runRuleByName(t, "RedundantSuspendModifier", `
package test

open suspend fun overridable() {}

abstract class Base {
    abstract suspend fun required()
    open suspend fun inherited() {}
}

class Child : Base() {
    override suspend fun inherited() {}
    override suspend fun required() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected open/override/abstract suspend functions to stay suppressed, got %d: %v", len(findings), findings)
	}
}

func runRedundantSuspendModifierWithCallTargetSuspend(t *testing.T, code string, suspendByCallText map[string]bool) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	if len(suspendByCallText) > 0 {
		fake.CallTargets[file.Path] = map[string]string{}
		fake.CallTargetSuspend[file.Path] = map[string]bool{}
		file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
			callText := strings.TrimSpace(file.FlatNodeText(idx))
			isSuspend, ok := suspendByCallText[callText]
			if !ok {
				return
			}
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = "test." + strings.TrimSuffix(callText, "()")
			fake.CallTargetSuspend[file.Path][key] = isSuspend
		})
	}
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range v2rules.Registry {
		if r.ID == "RedundantSuspendModifier" {
			d := rules.NewDispatcherV2([]*v2rules.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", "RedundantSuspendModifier")
	return nil
}

// --- SleepInsteadOfDelay ---

func TestSleepInsteadOfDelay_Positive(t *testing.T) {
	findings := runRuleByName(t, "SleepInsteadOfDelay", `
package test
suspend fun doWork() {
    Thread.sleep(1000)
}
`)
	if len(findings) == 0 {
		t.Error("expected SleepInsteadOfDelay to flag Thread.sleep in suspend fun")
	}
}

func TestSleepInsteadOfDelay_Negative(t *testing.T) {
	findings := runRuleByName(t, "SleepInsteadOfDelay", `
package test
suspend fun doWork() {
    delay(1000)
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// --- SuspendFunWithFlowReturnType ---

func TestSuspendFunWithFlowReturnType_Positive(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunWithFlowReturnType", `
package test
import kotlinx.coroutines.flow.Flow
suspend fun getItems(): Flow<Int> {
    return flow { emit(1) }
}
`)
	if len(findings) == 0 {
		t.Error("expected SuspendFunWithFlowReturnType to flag suspend fun returning Flow")
	}
}

func TestSuspendFunWithFlowReturnType_Negative(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunWithFlowReturnType", `
package test
import kotlinx.coroutines.flow.Flow
fun getItems(): Flow<Int> {
    return flow { emit(1) }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// --- InjectDispatcher ---

func TestInjectDispatcher_Positive(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
suspend fun loadData() {
    withContext(Dispatchers.IO) {
        fetchFromNetwork()
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected InjectDispatcher to flag hardcoded Dispatchers.IO")
	}
}

func TestInjectDispatcher_PositiveBareLaunch(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
suspend fun loadData() {
    launch(Dispatchers.Default) {
        fetchFromNetwork()
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected InjectDispatcher to flag hardcoded Dispatchers.Default in launch")
	}
}

func TestInjectDispatcher_Negative(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.CoroutineDispatcher
suspend fun loadData(dispatcher: CoroutineDispatcher) {
    withContext(dispatcher) {
        fetchFromNetwork()
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestInjectDispatcher_NegativeInjectedPatterns(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class Repository(
    private val ioDispatcher: CoroutineDispatcher,
    provider: DispatcherProvider,
) {
    private val defaultDispatcher: CoroutineDispatcher = ioDispatcher
    private val providerDispatcher: CoroutineDispatcher = provider.io

    suspend fun fromFunctionParam(dispatcher: CoroutineDispatcher = Dispatchers.IO) {
        withContext(dispatcher) { fetchFromNetwork() }
    }

    suspend fun fromConstructorParam() {
        withContext(ioDispatcher) { fetchFromNetwork() }
    }

    suspend fun fromClassProperty() {
        withContext(defaultDispatcher) { fetchFromNetwork() }
    }

    suspend fun fromProvider() {
        withContext(providerDispatcher) { fetchFromNetwork() }
    }
}

interface DispatcherProvider {
    val io: CoroutineDispatcher
}
`)
	if len(findings) != 0 {
		t.Errorf("expected injected dispatcher patterns to be clean, got %d: %v", len(findings), findings)
	}
}

func TestInjectDispatcher_TypeInfoConfirmsCoroutineDispatcher(t *testing.T) {
	findings := runInjectDispatcherWithExpressionType(t, `
package test
import kotlinx.coroutines.Dispatchers
suspend fun loadData() {
    withContext(Dispatchers.IO) { fetchFromNetwork() }
}
`, "Dispatchers.IO", &typeinfer.ResolvedType{Name: "CoroutineDispatcher", FQN: "kotlinx.coroutines.CoroutineDispatcher", Kind: typeinfer.TypeClass})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when oracle confirms CoroutineDispatcher, got %d", len(findings))
	}
}

func TestInjectDispatcher_TypeInfoSuppressesNonDispatcher(t *testing.T) {
	findings := runInjectDispatcherWithExpressionType(t, `
package test
object Dispatchers {
    val IO: String = "io"
}
suspend fun loadData() {
    withContext(Dispatchers.IO) { fetchFromNetwork() }
}
`, "Dispatchers.IO", &typeinfer.ResolvedType{Name: "String", FQN: "kotlin.String", Kind: typeinfer.TypePrimitive})
	if len(findings) != 0 {
		t.Fatalf("expected type-info mismatch to suppress finding, got %d: %v", len(findings), findings)
	}
}

func TestInjectDispatcher_WildcardImportLocalDispatchersLookalike(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.*

object Dispatchers {
    val IO: String = "io"
}

suspend fun loadData() {
    withContext(Dispatchers.IO) { fetchFromNetwork() }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected local Dispatchers lookalike to suppress finding, got %d: %v", len(findings), findings)
	}
}

func TestInjectDispatcher_DoesNotRequireTypeContext(t *testing.T) {
	rule := buildRuleIndex()["InjectDispatcher"]
	if rule == nil {
		t.Fatal("InjectDispatcher rule not found")
	}
	if rule.Needs.Has(v2rules.NeedsResolver) || rule.Needs.Has(v2rules.NeedsOracle) ||
		rule.Needs.Has(v2rules.NeedsParsedFiles) || rule.Needs.Has(v2rules.NeedsCrossFile) {
		t.Fatalf("InjectDispatcher should stay AST/import-only; got Needs=%b", rule.Needs)
	}
	if rule.TypeInfo != (v2rules.TypeInfoHint{}) {
		t.Fatalf("InjectDispatcher TypeInfo=%+v, want zero value", rule.TypeInfo)
	}
}

func TestInjectDispatcher_PositiveMain(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
suspend fun loadData() {
    withContext(Dispatchers.Main) { renderUi() }
}
`)
	if len(findings) == 0 {
		t.Error("expected InjectDispatcher to flag hardcoded Dispatchers.Main")
	}
}

func TestInjectDispatcher_ChainedCallNoDuplicates(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

suspend fun foo() {
    withContext(Dispatchers.IO) { }
    withContext(Dispatchers.Default) { }
}
`)
	if len(findings) != 2 {
		t.Errorf("expected exactly 2 findings, got %d", len(findings))
		for _, f := range findings {
			t.Logf("  L%d:%d %s", f.Line, f.Col, f.Message)
		}
	}
	lines := map[int]bool{}
	for _, f := range findings {
		lines[f.Line] = true
	}
	if len(lines) != 2 {
		t.Errorf("expected findings on 2 distinct lines, got %d distinct lines", len(lines))
	}
}

func runInjectDispatcherWithExpressionType(t *testing.T, code string, exprText string, typ *typeinfer.ResolvedType) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.Expressions[file.Path] = map[string]*typeinfer.ResolvedType{}
	file.FlatWalkNodes(0, "navigation_expression", func(idx uint32) {
		if strings.TrimSpace(file.FlatNodeText(idx)) == exprText {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.Expressions[file.Path][key] = typ
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range v2rules.Registry {
		if r.ID == "InjectDispatcher" {
			d := rules.NewDispatcherV2([]*v2rules.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", "InjectDispatcher")
	return nil
}

func TestInjectDispatcher_LinePointsToDispatcher(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
suspend fun loadData() {
    withContext(Dispatchers.Default) {
        fetchFromNetwork()
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	// Line 5 is "    withContext(Dispatchers.Default) {" — the Dispatchers.Default
	// should be reported on the line where it actually appears.
	if findings[0].Line != 5 {
		t.Errorf("expected finding on line 5, got line %d", findings[0].Line)
	}
}

func BenchmarkInjectDispatcher_ManyCalls(b *testing.B) {
	benchmarkRuleByName(b, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

suspend fun loadData() {
    withContext(Dispatchers.IO) { fetchA() }
    withContext(Dispatchers.Default) { fetchB() }
    flow.flowOn(Dispatchers.IO)
    viewModelScope.launch(Dispatchers.Default) { fetchC() }
    lifecycleScope.launch(Dispatchers.IO) { fetchD() }
    repeat(100) {
        withContext(Dispatchers.IO) { fetchA() }
        withContext(Dispatchers.Default) { fetchB() }
        flow.flowOn(Dispatchers.IO)
        viewModelScope.launch(Dispatchers.Default) { fetchC() }
        lifecycleScope.launch(Dispatchers.IO) { fetchD() }
    }
}
`)
}

// --- CoroutineLaunchedInTestWithoutRunTest ---

func TestCoroutineLaunchedInTestWithoutRunTest_Positive(t *testing.T) {
	findings := runRuleByName(t, "CoroutineLaunchedInTestWithoutRunTest", `
package test
import org.junit.Test
class MyTest {
    @Test
    fun myTest() {
        launch {
            doWork()
        }
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for launch in @Test without runTest")
	}
}

func TestCoroutineLaunchedInTestWithoutRunTest_Negative(t *testing.T) {
	findings := runRuleByName(t, "CoroutineLaunchedInTestWithoutRunTest", `
package test
import org.junit.Test
class MyTest {
    @Test
    fun myTest() = runTest {
        launch {
            doWork()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for launch inside runTest, got %d: %v", len(findings), findings)
	}
}

// --- SuspendFunInFinallySection ---

func TestSuspendFunInFinallySection_Positive(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunInFinallySection", `
package test
suspend fun riskyCleanup() {
    try {
        doWork()
    } finally {
        delay(100)
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for suspend call delay() in finally block")
	}
}

func TestSuspendFunInFinallySection_Negative(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunInFinallySection", `
package test
suspend fun safeCleanup() {
    try {
        doWork()
    } finally {
        println("done")
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-suspend call in finally, got %d: %v", len(findings), findings)
	}
}

// --- SuspendFunSwallowedCancellation ---

func TestSuspendFunSwallowedCancellation_Positive(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunSwallowedCancellation", `
package test
import kotlinx.coroutines.CancellationException
suspend fun doWork() {
    try {
        delay(1000)
    } catch (e: CancellationException) {
        println("cancelled")
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for catching CancellationException without rethrowing")
	}
}

func TestSuspendFunSwallowedCancellation_PositiveSuperclassCatch(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunSwallowedCancellation", `
package test
suspend fun doWork() {
    try {
        delay(1000)
    } catch (e: Exception) {
        println("cancelled")
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for catching Exception around suspend call without rethrowing")
	}
}

func TestSuspendFunSwallowedCancellation_Negative(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunSwallowedCancellation", `
package test
import kotlinx.coroutines.CancellationException
suspend fun doWork() {
    try {
        delay(1000)
    } catch (e: CancellationException) {
        println("cancelled")
        throw e
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings when CancellationException is rethrown, got %d: %v", len(findings), findings)
	}
}

func TestSuspendFunSwallowedCancellation_NegativeNonSuspendContext(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunSwallowedCancellation", `
package test
fun getProcessedEmoji(emoji: CharSequence): CharSequence = try {
    process(emoji)
} catch (e: IllegalStateException) {
    emoji
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings outside a suspend function, got %d: %v", len(findings), findings)
	}
}

func TestSuspendFunSwallowedCancellation_NegativeNoSuspendCall(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunSwallowedCancellation", `
package test
suspend fun doWork() {
    try {
        println("done")
    } catch (e: Exception) {
        println("ignored")
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings when try block has no suspend calls, got %d: %v", len(findings), findings)
	}
}

// --- SuspendFunWithCoroutineScopeReceiver ---

func TestSuspendFunWithCoroutineScopeReceiver_Positive(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunWithCoroutineScopeReceiver", `
package test
import kotlinx.coroutines.CoroutineScope
suspend fun CoroutineScope.doWork() {
    launch { }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for suspend fun with CoroutineScope receiver")
	}
}

func TestSuspendFunWithCoroutineScopeReceiver_Negative(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunWithCoroutineScopeReceiver", `
package test
import kotlinx.coroutines.CoroutineScope
fun CoroutineScope.doWork() {
    launch { }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for non-suspend CoroutineScope extension, got %d: %v", len(findings), findings)
	}
}

// --- ChannelReceiveWithoutClose ---

func TestChannelReceiveWithoutClose_Positive(t *testing.T) {
	findings := runRuleByName(t, "ChannelReceiveWithoutClose", `
package test
import kotlinx.coroutines.channels.Channel
class Worker {
    private val events = Channel<String>()
    fun send(e: String) { events.trySend(e) }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for Channel without close")
	}
}

func TestChannelReceiveWithoutClose_Negative(t *testing.T) {
	findings := runRuleByName(t, "ChannelReceiveWithoutClose", `
package test
import kotlinx.coroutines.channels.Channel
import java.io.Closeable
class Worker : Closeable {
    private val events = Channel<String>()
    fun send(e: String) { events.trySend(e) }
    override fun close() { events.close() }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- CollectionsSynchronizedListIteration ---

func TestCollectionsSynchronizedListIteration_Positive(t *testing.T) {
	findings := runRuleByName(t, "CollectionsSynchronizedListIteration", `
package test
import java.util.Collections
fun iterate() {
    for (item in Collections.synchronizedList(mutableListOf(1, 2))) {
        println(item)
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for unsynchronized iteration")
	}
}

func TestCollectionsSynchronizedListIteration_Negative(t *testing.T) {
	findings := runRuleByName(t, "CollectionsSynchronizedListIteration", `
package test
import java.util.Collections
fun iterate() {
    val list = Collections.synchronizedList(mutableListOf(1, 2))
    synchronized(list) {
        for (item in list) { println(item) }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- ConcurrentModificationIteration ---

func TestConcurrentModificationIteration_Positive(t *testing.T) {
	findings := runRuleByName(t, "ConcurrentModificationIteration", `
package test
fun removeStale(items: MutableList<String>) {
    for (item in items) {
        if (item == "old") items.remove(item)
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for concurrent modification")
	}
}

func TestConcurrentModificationIteration_Negative(t *testing.T) {
	findings := runRuleByName(t, "ConcurrentModificationIteration", `
package test
fun removeStale(items: MutableList<String>) {
    items.removeAll { it == "old" }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- CoroutineScopeCreatedButNeverCancelled ---

func TestCoroutineScopeCreatedButNeverCancelled_Positive(t *testing.T) {
	findings := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", `
package test
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.Dispatchers
class Cache {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for uncancelled scope")
	}
}

func TestCoroutineScopeCreatedButNeverCancelled_Negative(t *testing.T) {
	findings := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", `
package test
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.cancel
import java.io.Closeable
class Cache : Closeable {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    override fun close() { scope.cancel() }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- DeferredAwaitInFinally ---

func TestDeferredAwaitInFinally_Positive(t *testing.T) {
	findings := runRuleByName(t, "DeferredAwaitInFinally", `
package test
import kotlinx.coroutines.Deferred
suspend fun doWork(cleanup: Deferred<Unit>) {
    try { println("working") } finally { cleanup.await() }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for await in finally")
	}
}

func TestDeferredAwaitInFinally_Negative(t *testing.T) {
	findings := runRuleByName(t, "DeferredAwaitInFinally", `
package test
import kotlinx.coroutines.Deferred
suspend fun doWork(cleanup: Deferred<Unit>) {
    try { println("working") } finally { runCatching { cleanup.await() } }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- FlowWithoutFlowOn ---

func TestFlowWithoutFlowOn_Positive(t *testing.T) {
	findings := runRuleByName(t, "FlowWithoutFlowOn", `
package test
import kotlinx.coroutines.flow.flow
fun collectRows() {
    flow { emit(db.query()) }.collect { render(it) }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for flow without flowOn")
	}
}

func TestFlowWithoutFlowOn_Negative(t *testing.T) {
	findings := runRuleByName(t, "FlowWithoutFlowOn", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOn
fun collectRows() {
    flow { emit(db.query()) }.flowOn(Dispatchers.IO).collect { render(it) }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- SynchronizedOnString ---

func TestSynchronizedOnString_Positive(t *testing.T) {
	findings := runRuleByName(t, "SynchronizedOnString", `
package test
class Cache {
    fun mutate() {
        synchronized("global") { println("work") }
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for synchronized on string")
	}
}

func TestSynchronizedOnString_Negative(t *testing.T) {
	findings := runRuleByName(t, "SynchronizedOnString", `
package test
class Cache {
    private val lock = Any()
    fun mutate() {
        synchronized(lock) { println("work") }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- SynchronizedOnBoxedPrimitive ---

func TestSynchronizedOnBoxedPrimitive_Positive(t *testing.T) {
	findings := runRuleByName(t, "SynchronizedOnBoxedPrimitive", `
package test
class Counter {
    val count: Int = 1
    fun work() {
        synchronized(count) { println("work") }
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for synchronized on boxed primitive")
	}
}

func TestSynchronizedOnBoxedPrimitive_Negative(t *testing.T) {
	findings := runRuleByName(t, "SynchronizedOnBoxedPrimitive", `
package test
class Counter {
    private val lock = Any()
    fun work() {
        synchronized(lock) { println("work") }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- SynchronizedOnNonFinal ---

func TestSynchronizedOnNonFinal_Positive(t *testing.T) {
	findings := runRuleByName(t, "SynchronizedOnNonFinal", `
package test
class Worker {
    private var lock = Any()
    fun op() {
        synchronized(lock) { println("work") }
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for synchronized on var")
	}
}

func TestSynchronizedOnNonFinal_Negative(t *testing.T) {
	findings := runRuleByName(t, "SynchronizedOnNonFinal", `
package test
class Worker {
    private val lock = Any()
    fun op() {
        synchronized(lock) { println("work") }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- VolatileMissingOnDcl ---

func TestVolatileMissingOnDcl_Positive(t *testing.T) {
	findings := runRuleByName(t, "VolatileMissingOnDcl", `
package test
class Singleton {
    private var instance: Singleton? = null
    fun getInstance(): Singleton {
        if (instance == null) {
            synchronized(this) {
                if (instance == null) { instance = Singleton() }
            }
        }
        return instance!!
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for DCL without @Volatile")
	}
}

func TestVolatileMissingOnDcl_Negative(t *testing.T) {
	findings := runRuleByName(t, "VolatileMissingOnDcl", `
package test
class Singleton {
    @Volatile private var instance: Singleton? = null
    fun getInstance(): Singleton {
        if (instance == null) {
            synchronized(this) {
                if (instance == null) { instance = Singleton() }
            }
        }
        return instance!!
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- MutableStateInObject ---

func TestMutableStateInObject_Positive(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
object Counter {
    var total = 0
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for mutable state in object")
	}
}

func TestMutableStateInObject_Negative(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
import java.util.concurrent.atomic.AtomicInteger
object Counter {
    private val total = AtomicInteger(0)
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestMutableStateInObject_IgnoresTestSharedSourceSet(t *testing.T) {
	findings := runRuleByNameOnPath(t, "MutableStateInObject", "src/testShared/Fake.kt", `
package test
object FakeState {
    var total = 0
}
`)
	if len(findings) != 0 {
		t.Errorf("expected testShared source to be clean, got %d", len(findings))
	}
}

func TestMutableStateInObject_IgnoresPrivateSynchronizedState(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
object Counter {
    private var total = 0
    fun next(): Int {
        synchronized(this) {
            total++
            return total
        }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected synchronized private state to be clean, got %d", len(findings))
	}
}

func TestMutableStateInObject_FlagsPrivateUnsynchronizedState(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
object Counter {
    private var total = 0
    fun next(): Int {
        total++
        return total
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for unsynchronized private mutable state")
	}
}

func TestMutableStateInObject_NoTypeOracleCapability(t *testing.T) {
	for _, rule := range v2rules.Registry {
		if rule.ID != "MutableStateInObject" {
			continue
		}
		if rule.Needs.Has(v2rules.NeedsResolver) || rule.Needs.Has(v2rules.NeedsTypeInfo) || rule.Needs.Has(v2rules.NeedsOracle) ||
			rule.Oracle != nil || rule.OracleCallTargets != nil || rule.OracleDeclarationNeeds != nil {
			t.Fatalf("MutableStateInObject should remain AST-only, got Needs=%b Oracle=%+v CallTargets=%+v DeclarationNeeds=%+v",
				rule.Needs, rule.Oracle, rule.OracleCallTargets, rule.OracleDeclarationNeeds)
		}
		return
	}
	t.Fatal("MutableStateInObject rule not found")
}

// --- StateFlowMutableLeak ---

func TestStateFlowMutableLeak_Positive(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test
import kotlinx.coroutines.flow.MutableStateFlow
class VM {
    val state = MutableStateFlow(0)
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for public MutableStateFlow")
	}
}

func TestStateFlowMutableLeak_Negative(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
class VM {
    private val _state = MutableStateFlow(0)
    val state: StateFlow<Int> = _state
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestStateFlowMutableLeak_NegativeLocalLookalike(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test
class MutableStateFlow<T>(value: T)
class VM {
    val state = MutableStateFlow(0)
}
`)
	if len(findings) != 0 {
		t.Errorf("expected local MutableStateFlow lookalike to be ignored, got %d", len(findings))
	}
}

func TestStateFlowMutableLeak_NegativeOverrideReportedAtContract(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test
import kotlinx.coroutines.flow.MutableStateFlow

interface VM {
    val state: MutableStateFlow<Int>
}

class RealVM : VM {
    override val state = MutableStateFlow(0)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected only the mutable interface contract to be reported, got %d", len(findings))
	}
}

// --- SharedFlowWithoutReplay ---

func TestSharedFlowWithoutReplay_Positive(t *testing.T) {
	findings := runRuleByName(t, "SharedFlowWithoutReplay", `
package test
import kotlinx.coroutines.flow.MutableSharedFlow
class EventBus {
    private val events = MutableSharedFlow<String>()
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for SharedFlow without replay")
	}
}

func TestSharedFlowWithoutReplay_Negative(t *testing.T) {
	findings := runRuleByName(t, "SharedFlowWithoutReplay", `
package test
import kotlinx.coroutines.flow.MutableSharedFlow
class EventBus {
    private val events = MutableSharedFlow<String>(extraBufferCapacity = 1)
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- StateFlowCompareByReference ---

func TestStateFlowCompareByReference_Positive(t *testing.T) {
	findings := runRuleByName(t, "StateFlowCompareByReference", `
package test
import kotlinx.coroutines.flow.StateFlow
fun observe(state: StateFlow<UiState>) {
    state.map { it.count }.distinctUntilChanged().collect { render(it) }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for redundant distinctUntilChanged")
	}
}

func TestStateFlowCompareByReference_Negative(t *testing.T) {
	findings := runRuleByName(t, "StateFlowCompareByReference", `
package test
import kotlinx.coroutines.flow.StateFlow
fun observe(state: StateFlow<UiState>) {
    state.map { it.count }.collect { render(it) }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- GlobalScopeLaunchInViewModel ---

func TestGlobalScopeLaunchInViewModel_Positive(t *testing.T) {
	findings := runRuleByName(t, "GlobalScopeLaunchInViewModel", `
package test
import kotlinx.coroutines.GlobalScope
class UserViewModel {
    fun load() {
        GlobalScope.launch { fetchData() }
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for GlobalScope in ViewModel")
	}
}

func TestGlobalScopeLaunchInViewModel_Negative(t *testing.T) {
	findings := runRuleByName(t, "GlobalScopeLaunchInViewModel", `
package test
class UserViewModel {
    fun load() {
        viewModelScope.launch { fetchData() }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- SupervisorScopeInEventHandler ---

func TestSupervisorScopeInEventHandler_Positive(t *testing.T) {
	findings := runRuleByName(t, "SupervisorScopeInEventHandler", `
package test
import kotlinx.coroutines.supervisorScope
suspend fun handle() = supervisorScope {
    fetch()
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for single-child supervisorScope")
	}
}

func TestSupervisorScopeInEventHandler_Negative(t *testing.T) {
	findings := runRuleByName(t, "SupervisorScopeInEventHandler", `
package test
import kotlinx.coroutines.async
import kotlinx.coroutines.supervisorScope
suspend fun handle() = supervisorScope {
    val a = async { fetchA() }
    val b = async { fetchB() }
    a.await() to b.await()
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- WithContextInSuspendFunctionNoop ---

func TestWithContextInSuspendFunctionNoop_Positive(t *testing.T) {
	findings := runRuleByName(t, "WithContextInSuspendFunctionNoop", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
suspend fun loadData() {
    withContext(Dispatchers.IO) {
        withContext(Dispatchers.IO) {
            fetch()
        }
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for redundant nested withContext")
	}
}

func TestWithContextInSuspendFunctionNoop_Negative(t *testing.T) {
	findings := runRuleByName(t, "WithContextInSuspendFunctionNoop", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
suspend fun loadData() {
    withContext(Dispatchers.IO) {
        fetch()
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// --- LaunchWithoutCoroutineExceptionHandler ---

func TestLaunchWithoutCoroutineExceptionHandler_Positive(t *testing.T) {
	findings := runRuleByName(t, "LaunchWithoutCoroutineExceptionHandler", `
package test
import kotlinx.coroutines.GlobalScope
fun start() {
    GlobalScope.launch {
        throw RuntimeException("boom")
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding for launch without handler")
	}
}

func TestLaunchWithoutCoroutineExceptionHandler_Negative(t *testing.T) {
	findings := runRuleByName(t, "LaunchWithoutCoroutineExceptionHandler", `
package test
import kotlinx.coroutines.CoroutineExceptionHandler
import kotlinx.coroutines.GlobalScope
fun start() {
    val handler = CoroutineExceptionHandler { _, t -> println(t) }
    GlobalScope.launch(handler) {
        throw RuntimeException("boom")
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}
