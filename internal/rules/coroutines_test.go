package rules_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
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

// Oracle resolves `collect` to a non-Flow callable → suppressed.
func TestCollectInOnCreateWithoutLifecycle_OracleSuppressesNonFlowCollect(t *testing.T) {
	findings := runCollectInOnCreateWithCallTarget(t, `
package test
class ExampleActivity {
    fun onCreate() {
        items.collect { render(it) }
    }
}
`, "items.collect", "com.acme.local.Collector.collect")
	if len(findings) != 0 {
		t.Fatalf("expected oracle to suppress non-Flow collect, got %d: %v", len(findings), findings)
	}
}

// Oracle confirms Flow.collect → fires.
func TestCollectInOnCreateWithoutLifecycle_OracleConfirmsFlowCollect(t *testing.T) {
	findings := runCollectInOnCreateWithCallTarget(t, `
package test
class ExampleActivity {
    fun onCreate() {
        vm.state.collect { render(it) }
    }
}
`, "vm.state.collect", "kotlinx.coroutines.flow.FlowKt.collect")
	if len(findings) != 1 {
		t.Fatalf("expected oracle-confirmed Flow.collect to fire, got %d: %v", len(findings), findings)
	}
}

// The call-target spellings the oracle backends produce for a Flow-family
// `collect`: krit-fir names the package-level extension without a facade
// class and a member by its declaring type, krit-types adds the FlowKt
// facade. All of them are the collect the rule reports, as the krit-fir
// checker (any callable or receiver in kotlinx.coroutines.flow) does.
func TestCollectInOnCreateWithoutLifecycle_OracleConfirmsFlowPackageTargets(t *testing.T) {
	for _, target := range []string{
		"kotlinx.coroutines.flow.collect",
		"kotlinx.coroutines.flow.FlowKt.collect",
		"kotlinx.coroutines.flow.Flow.collect",
		"kotlinx.coroutines.flow.MutableStateFlow.collect",
		"kotlinx.coroutines.flow.ChannelFlow.collect",
	} {
		findings := runCollectInOnCreateWithCallTarget(t, `
package test
class ExampleActivity {
    fun onCreate() {
        vm.state.collect { render(it) }
    }
}
`, "vm.state.collect", target)
		if len(findings) != 1 {
			t.Errorf("target %s: expected 1 finding, got %d: %v", target, len(findings), findings)
		}
	}
}

// A package that merely starts with the same letters is not the Flow package.
func TestCollectInOnCreateWithoutLifecycle_OracleSuppressesLookalikePackage(t *testing.T) {
	findings := runCollectInOnCreateWithCallTarget(t, `
package test
class ExampleActivity {
    fun onCreate() {
        vm.state.collect { render(it) }
    }
}
`, "vm.state.collect", "kotlinx.coroutines.flowlike.Stream.collect")
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// Pins NeedsOracleCallTargets on the rule.
func TestCollectInOnCreateWithoutLifecycle_DeclaresOracleCallTargets(t *testing.T) {
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "CollectInOnCreateWithoutLifecycle" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("rule not registered")
	}
	if !rule.Needs.Has(api.NeedsOracleCallTargets) {
		t.Errorf("missing NeedsOracleCallTargets: Needs=%b", rule.Needs)
	}
}

func runCollectInOnCreateWithCallTarget(t *testing.T, code string, callText string, target string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		if target != "" && strings.HasPrefix(strings.TrimSpace(file.FlatNodeText(idx)), callText) {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = target
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == "CollectInOnCreateWithoutLifecycle" {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule not found in registry")
	return nil
}

func TestCollectInOnCreateWithoutLifecycle_OracleUnresolvedStaysSilent(t *testing.T) {
	findings := runCollectInOnCreateWithCallTarget(t, `
package test
class ExampleActivity {
    fun onCreate() {
        vm.state.collect { render(it) }
    }
}
`, "vm.state.collect", "")
	if len(findings) != 0 {
		t.Fatalf("expected unresolved oracle to suppress collect, got %d: %v", len(findings), findings)
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

func TestRedundantSuspendModifier_ForLoopOverChannelSuspends(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RedundantSuspendModifier", `
package test
import kotlinx.coroutines.channels.ReceiveChannel
class A(private val ch: ReceiveChannel<Int>) {
    suspend fun drain() {
        for (x in ch) { println(x) }
    }
    fun consume(x: Int) {}
}
`)
	if len(findings) != 0 {
		t.Errorf("for loop over a channel suspends in hasNext, got %v", findings)
	}
}

func TestRedundantSuspendModifier_ForLoopOverListStillFlagged(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "RedundantSuspendModifier", `
package test
class A {
    suspend fun walk(items: List<Int>) {
        for (x in items) { println(x) }
    }
    fun consume(x: Int) {}
}
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %v", findings)
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

func TestRedundantSuspendModifier_UnknownCallShapesStayConservative(t *testing.T) {
	tests := map[string]string{
		"invoke on a value":        "suspend fun f(useCase: UseCase, p: Params) = useCase(p)",
		"invoke on this":           "class A { suspend fun f(p: Params) = this(p) }",
		"invoke on indexed value":  "suspend fun f(hs: List<Handler>, p: Params) = hs[0](p)",
		"cross-module member call": "suspend fun f(repo: Repo) = repo.load()",
		"chained first":            "suspend fun f(repo: Repo) = repo.items().first()",
		"chained awaitFirst":       "suspend fun f(repo: Repo) = repo.stream().awaitFirst()",
		"member named like helper": "suspend fun f(repo: Repo) = repo.check()",
		"call on parenthesized":    "suspend fun f(p: Params) = (lookup())(p)",
	}
	for name, decl := range tests {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByName(t, "RedundantSuspendModifier", "package test\n"+decl+"\n")
			if len(findings) != 0 {
				t.Fatalf("expected unknown call to suppress finding, got %d: %v", len(findings), findings)
			}
		})
	}
}

func TestRedundantSuspendModifier_KnownNonSuspendCallsStillFlag(t *testing.T) {
	findings := runRuleByName(t, "RedundantSuspendModifier", `
package test
suspend fun f(a: Int): List<Int> {
    require(a > 0)
    return listOf(a)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

// A chain's calls share their start; krit-fir keys the second one it records "line:col@endByte".
func TestRedundantSuspendModifier_ChainedSuspendCallSharingAStart(t *testing.T) {
	file := parseInline(t, `
package test
interface Query {
    suspend fun awaitAll(): List<Int>
}
interface Source {
    fun query(): Query
}

suspend fun needed(source: Source): List<Int> {
    return source.query().awaitAll()
}
`)
	var inner, outer uint32
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		switch strings.TrimSpace(file.FlatNodeText(idx)) {
		case "source.query()":
			inner = idx
		case "source.query().awaitAll()":
			outer = idx
		}
	})
	if inner == 0 || outer == 0 || file.FlatStartByte(inner) != file.FlatStartByte(outer) {
		t.Fatalf("expected two calls sharing a start, got inner=%d outer=%d", inner, outer)
	}
	key := fmt.Sprintf("%d:%d", file.FlatRow(inner)+1, file.FlatCol(inner)+1)
	fact := func(idx uint32, target string, suspend bool) *oracle.ExpressionType {
		return &oracle.ExpressionType{
			StartByte: int(file.FlatStartByte(idx)), EndByte: int(file.FlatEndByte(idx)),
			CallTarget: target, CallTargetResolved: true, CallTargetSuspend: suspend,
		}
	}
	o, err := oracle.LoadFromData(&oracle.Data{
		Version: 1, KotlinVersion: "2.1.0",
		Files: map[string]*oracle.File{file.Path: {Expressions: map[string]*oracle.ExpressionType{
			key: fact(inner, "test.Source.query", false),
			fmt.Sprintf("%s@%d", key, file.FlatEndByte(outer)): fact(outer, "test.Query.awaitAll", true),
		}}},
		Dependencies: map[string]*oracle.Class{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if suspend, ok := o.LookupCallTargetSuspendFlat(file, outer); !ok || !suspend {
		t.Fatalf("outer call: suspend=%v ok=%v, want the chained call's own suspend fact", suspend, ok)
	}
	if suspend, ok := o.LookupCallTargetSuspendFlat(file, inner); !ok || suspend {
		t.Fatalf("inner call: suspend=%v ok=%v, want its own non-suspend fact", suspend, ok)
	}
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	composite := oracle.NewCompositeResolver(o, resolver)
	for _, r := range api.Registry {
		if r.ID == "RedundantSuspendModifier" {
			cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
			findings := cols.Findings()
			if len(findings) != 0 {
				t.Fatalf("the chained suspend call must keep the modifier, got %v", findings)
			}
			return
		}
	}
	t.Fatal("RedundantSuspendModifier not registered")
}

func runRedundantSuspendModifierWithCallTargetSuspend(t *testing.T, code string, suspendByCallText map[string]bool) []scanner.Finding {
	t.Helper()
	return runRedundantSuspendModifierWithProjectFiles(t, code, nil, suspendByCallText)
}

func runRedundantSuspendModifierWithProjectFiles(t *testing.T, code string, otherCode []string, suspendByCallText map[string]bool) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	indexed := []*scanner.File{file}
	for _, other := range otherCode {
		indexed = append(indexed, parseInline(t, other))
	}
	resolver.IndexFilesParallel(indexed, 1)
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
	for _, r := range api.Registry {
		if r.ID == "RedundantSuspendModifier" {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
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

func TestSuspendFunWithFlowReturnType_NegativeOpenHooks(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunWithFlowReturnType", `
package test
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flowOf

open class UseCase {
    protected open suspend fun convert(input: Int): Flow<Int> = flowOf(input)
    open suspend fun publicHook(input: Int): Flow<Int> = flowOf(input)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for open hooks, got %d: %v", len(findings), findings)
	}
}

func TestSuspendFunWithFlowReturnType_PositiveFinalSuspend(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunWithFlowReturnType", `
package test
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.flowOf

class UseCase {
    suspend fun convert(input: Int): Flow<Int> = flowOf(input)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
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

func TestSuspendFunWithFlowReturnType_NegativeSignatureDictatedElsewhere(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunWithFlowReturnType", `
package test
import kotlinx.coroutines.flow.Flow
interface Source {
    suspend fun items(): Flow<Int>
    suspend fun withBody(): Flow<Int> = flow { emit(1) }
}
abstract class Base {
    abstract suspend fun more(): Flow<Int>
}
class Impl : Base(), Source {
    override suspend fun items(): Flow<Int> = flow { emit(1) }
    override suspend fun more(): Flow<Int> = flow { emit(2) }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestSuspendFunWithFlowReturnType_NegativeSuspendCallBeforeFlow(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunWithFlowReturnType", `
package test
import kotlinx.coroutines.flow.Flow
suspend fun warmUp() {}
suspend fun viaKnown(): Flow<Int> {
    delay(10)
    return flow { emit(1) }
}
suspend fun viaLocal(): Flow<Int> {
    warmUp()
    return flow { emit(1) }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestSuspendFunWithFlowReturnType_PositiveSuspendCallOnlyInsideFlowLambda(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunWithFlowReturnType", `
package test
import kotlinx.coroutines.flow.Flow
suspend fun items(): Flow<Int> {
    return flow {
        delay(10)
        emit(1)
    }
}
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

// --- InjectDispatcher ---

// TestInjectDispatcher_RegistryDefaultMatchesMeta guards the
// registry-instantiated DispatcherNames against silently drifting from
// the metadata Default that the docs/schema/config emitter advertise.
// Both must list the runtime defaults — IO, Default, Unconfined, Main —
// so a user copying the documented default into YAML gets the same
// behavior as omitting the option.
func TestInjectDispatcher_RegistryDefaultMatchesMeta(t *testing.T) {
	var rule *rules.InjectDispatcherRule
	for _, candidate := range api.Registry {
		if candidate.ID == "InjectDispatcher" {
			var ok bool
			rule, ok = candidate.Implementation.(*rules.InjectDispatcherRule)
			if !ok {
				t.Fatalf("expected InjectDispatcherRule, got %T", candidate.Implementation)
			}
			break
		}
	}
	if rule == nil {
		t.Fatal("InjectDispatcher rule not registered")
	}
	want := []string{"IO", "Default", "Unconfined"}
	if !equalStringSlices(rule.DispatcherNames, want) {
		t.Fatalf("registry DispatcherNames drift: got %v, want %v", rule.DispatcherNames, want)
	}

	// Cross-check metadata Default.
	desc := rule.Meta()
	for _, opt := range desc.Options {
		if opt.Name == "dispatcherNames" {
			gotMeta, ok := opt.Default.([]string)
			if !ok {
				t.Fatalf("metadata dispatcherNames Default has unexpected type %T", opt.Default)
			}
			if !equalStringSlices(gotMeta, want) {
				t.Fatalf("metadata dispatcherNames Default drift: got %v, want %v", gotMeta, want)
			}
			return
		}
	}
	t.Fatal("dispatcherNames option missing from metadata")
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestInjectDispatcher_Positive(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
class Repo {
    suspend fun loadData() {
        withContext(Dispatchers.IO) {
            fetchFromNetwork()
        }
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected InjectDispatcher to flag hardcoded Dispatchers.IO")
	}
}

// Unconfined is a deliberate choice, so it is not reported.
func TestInjectDispatcher_NegativeUnconfined(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
class Repo(private val scope: CoroutineScope) {
    fun start() {
        scope.launch(Dispatchers.Unconfined, start = CoroutineStart.UNDISPATCHED) { work() }
    }
    suspend fun run() = withContext(Dispatchers.Unconfined) { work() }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for Dispatchers.Unconfined, got %v", findings)
	}
}

// Test source sets inject dispatchers by hand, so the rule skips them.
func TestInjectDispatcher_NegativeTestSourceSet(t *testing.T) {
	findings := runRuleByNameOnPath(t, "InjectDispatcher", "src/test/kotlin/RepoTest.kt", `
package test
import kotlinx.coroutines.Dispatchers
class RepoTest {
    fun build() = Repo(Dispatchers.IO)
    suspend fun load() = withContext(Dispatchers.IO) { fetch() }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings in test source set, got %v", findings)
	}
}

// A dispatcher combined with other context elements through `+` is as
// hardcoded as a lone argument, on either side of the operator.
func TestInjectDispatcher_PositiveContextPlusChain(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.CoroutineName
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
class Repo(private val handler: CoroutineExceptionHandler) {
    suspend fun right() = withContext(CoroutineName("load") + Dispatchers.IO) { fetch() }
    suspend fun left() = withContext(Dispatchers.Default + CoroutineName("load")) { fetch() }
    suspend fun nested() = withContext((handler + Dispatchers.Default) + CoroutineName("load")) { fetch() }
}
`)
	want := map[int]string{
		7: "Hardcoded Dispatchers.IO. Inject dispatchers for better testability.",
		8: "Hardcoded Dispatchers.Default. Inject dispatchers for better testability.",
		9: "Hardcoded Dispatchers.Default. Inject dispatchers for better testability.",
	}
	if len(findings) != len(want) {
		t.Fatalf("expected %d findings, got %d: %v", len(want), len(findings), findings)
	}
	for _, f := range findings {
		if want[f.Line] != f.Message {
			t.Errorf("line %d: got %q, want %q", f.Line, f.Message, want[f.Line])
		}
	}
}

// `Main + IO` leaves IO active (CoroutineContext.plus keeps the right-hand
// element for the same key), so a leading Main must not hide the injectable
// dispatcher, and every hardcoded operand is reported, as the FIR checker does.
func TestInjectDispatcher_PositiveContextPlusChainEveryNonMainOperand(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
class Repo {
    suspend fun mainThenIO() = withContext(Dispatchers.Main + Dispatchers.IO) { fetch() }
    suspend fun both() = withContext(Dispatchers.IO + Dispatchers.Default) { fetch() }
}
`)
	type key struct{ line, col int }
	want := map[key]string{
		{6, 63}: "Hardcoded Dispatchers.IO. Inject dispatchers for better testability.",
		{7, 38}: "Hardcoded Dispatchers.IO. Inject dispatchers for better testability.",
		{7, 55}: "Hardcoded Dispatchers.Default. Inject dispatchers for better testability.",
	}
	if len(findings) != len(want) {
		t.Fatalf("expected %d findings, got %d: %v", len(want), len(findings), findings)
	}
	for _, f := range findings {
		if want[key{f.Line, f.Col}] != f.Message {
			t.Errorf("%d:%d: got %q, want one of %v", f.Line, f.Col, f.Message, want)
		}
	}
}

// The idiomatic-host and top-level exemptions hold for a combined context
// too, a `+` chain that is not passed to a call is out of the rule's scope,
// and a chain with another operator is not a context combination.
func TestInjectDispatcher_NegativeContextPlusChain(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.CoroutineName
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
class ImageCache : ViewModel() {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val named = CoroutineScope(Dispatchers.Default + CoroutineName("cache"))
    private val context = SupervisorJob() + Dispatchers.IO
    fun load() {
        viewModelScope.launch(Dispatchers.IO + CoroutineName("load")) { fetch() }
    }
    suspend fun main() = withContext(Dispatchers.Main + CoroutineName("ui")) { render() }
    suspend fun minus() = withContext(base - Dispatchers.IO) { fetch() }
}
suspend fun topLevel() = withContext(CoroutineName("load") + Dispatchers.IO) { fetch() }
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

// A dispatcher that leads a `+` chain starts at the same position as the
// chain, so the oracle's position lookup answers with the chain's target
// (CoroutineContext.plus). That is not evidence of a lookalike Dispatchers
// object and must not suppress the finding.
func TestInjectDispatcher_OracleKeepsDispatcherLeadingPlusChain(t *testing.T) {
	file := parseInline(t, `
package test
import kotlinx.coroutines.CoroutineName
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
class Repo {
    suspend fun left() = withContext(Dispatchers.Default + CoroutineName("load")) { fetch() }
    suspend fun lookalike() = withContext(Dispatchers.IO) { fetch() }
}
`)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "navigation_expression", func(idx uint32) {
		key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
		switch file.FlatNodeText(idx) {
		case "Dispatchers.Default":
			fake.CallTargets[file.Path][key] = "kotlin.coroutines.CoroutineContext.plus"
		case "Dispatchers.IO":
			fake.CallTargets[file.Path][key] = "com.acme.local.Dispatchers.IO"
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID != "InjectDispatcher" {
			continue
		}
		cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
		findings := cols.Findings()
		if len(findings) != 1 || findings[0].Line != 7 {
			t.Fatalf("expected 1 finding on line 7 (and the lookalike on line 8 suppressed), got %v", findings)
		}
		return
	}
	t.Fatal("rule not found in registry")
}

func TestInjectDispatcher_PositiveBareLaunch(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
class Repo {
    suspend fun loadData() {
        launch(Dispatchers.Default) {
            fetchFromNetwork()
        }
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

// A top-level function has no enclosing class/object to inject a
// dispatcher into, so a hardcoded Dispatchers.* there is not actionable
// and must not be flagged. Mirrors the FIR checker's top-level
// exemption. Detection is structural: no class_declaration /
// object_declaration / companion_object ancestor.
func TestInjectDispatcher_NegativeTopLevelFunction(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

fun loadTopLevel() {
    withContext(Dispatchers.IO) { fetchFromNetwork() }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for top-level function, got %d: %v", len(findings), findings)
	}
}

// An extension function's receiver is fixed by the call site and it has
// no enclosing type to inject into, so a hardcoded Dispatchers.* must
// not be flagged. Detection is structural: the function_declaration has
// a `.` token direct child (receiver-type separator), not a name match.
func TestInjectDispatcher_NegativeExtensionFunction(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

fun String.loadExt() {
    withContext(Dispatchers.Default) { fetchFromNetwork() }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for extension function, got %d: %v", len(findings), findings)
	}
}

// A generic extension function (receiver type carries type parameters)
// is still an extension and must not be flagged. Confirms the `.`-token
// detection is robust to `fun <T> T.foo()` shapes.
func TestInjectDispatcher_NegativeGenericExtensionFunction(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

suspend fun <T> T.loadExt() {
    withContext(Dispatchers.IO) { fetchFromNetwork() }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for generic extension function, got %d: %v", len(findings), findings)
	}
}

// An extension function declared *inside* a class still has an
// injectable host (the class), so it must continue to flag. Guards
// against over-suppressing every extension regardless of enclosing
// type.
func TestInjectDispatcher_PositiveMemberExtensionFunction(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class Repo {
    suspend fun String.loadExt() {
        withContext(Dispatchers.IO) { fetchFromNetwork() }
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected member extension function to flag hardcoded Dispatchers.IO")
	}
}

func TestInjectDispatcher_TypeInfoConfirmsCoroutineDispatcher(t *testing.T) {
	findings := runInjectDispatcherWithExpressionType(t, `
package test
import kotlinx.coroutines.Dispatchers
class Repo {
    suspend fun loadData() {
        withContext(Dispatchers.IO) { fetchFromNetwork() }
    }
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

// Pins the post-migration capability shape: TypeInfo + NeedsOracleCallTargets, single-file only.
func TestInjectDispatcher_DeclaresOracleCallTargets(t *testing.T) {
	rule := buildRuleIndex()["InjectDispatcher"]
	if rule == nil {
		t.Fatal("InjectDispatcher rule not found")
	}
	if !rule.Needs.Has(api.NeedsTypeInfo) {
		t.Errorf("InjectDispatcher missing NeedsTypeInfo: Needs=%b", rule.Needs)
	}
	if !rule.Needs.Has(api.NeedsOracleCallTargets) {
		t.Errorf("InjectDispatcher missing NeedsOracleCallTargets: Needs=%b", rule.Needs)
	}
	if rule.Needs.Has(api.NeedsParsedFiles) || rule.Needs.Has(api.NeedsCrossFile) ||
		rule.Needs.Has(api.NeedsModuleIndex) {
		t.Errorf("InjectDispatcher should be single-file only; got Needs=%b", rule.Needs)
	}
}

// Dispatchers.Main is intentionally exempted: kotlinx-coroutines-test
// provides Dispatchers.setMain(testDispatcher) for swapping it in
// tests, so injection isn't the only path to testability. Mirrors the
// equivalent exemption in the krit-fir InjectDispatcher checker.
func TestInjectDispatcher_NegativeMainIsExempt(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
suspend fun loadData() {
    withContext(Dispatchers.Main) { renderUi() }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for Dispatchers.Main (test-swappable via setMain), got %d: %v", len(findings), findings)
	}
}

func TestInjectDispatcher_ChainedCallNoDuplicates(t *testing.T) {
	findings := runRuleByName(t, "InjectDispatcher", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

class Repo {
    suspend fun foo() {
        withContext(Dispatchers.IO) { }
        withContext(Dispatchers.Default) { }
    }
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

// runInjectDispatcherWithCallTarget sets a synthetic oracle
// `callTarget` for every navigation_expression matching exprText
// and runs the InjectDispatcher rule against the resulting
// composite resolver. Pairs with the post-migration logic that
// keys on the resolved call-target FQN.
func runInjectDispatcherWithCallTarget(t *testing.T, code string, exprText string, target string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	file.FlatWalkNodes(0, "navigation_expression", func(idx uint32) {
		if strings.TrimSpace(file.FlatNodeText(idx)) == exprText {
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			fake.CallTargets[file.Path][key] = target
		}
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == "InjectDispatcher" {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			return cols.Findings()
		}
	}
	t.Fatalf("rule %q not found in registry", "InjectDispatcher")
	return nil
}

func TestInjectDispatcher_OracleCallTargetSuppressesLookalike(t *testing.T) {
	// A project-local `Dispatchers` lookalike whose oracle-resolved
	// callable FQN is NOT `kotlinx.coroutines.Dispatchers.IO`. The
	// AST token match alone would have produced a false positive
	// here; the oracle gate suppresses it.
	findings := runInjectDispatcherWithCallTarget(t, `
package test
suspend fun loadData() {
    withContext(Dispatchers.IO) { fetchFromNetwork() }
}
`, "Dispatchers.IO", "com.acme.local.Dispatchers.IO")
	if len(findings) != 0 {
		t.Fatalf("expected oracle to suppress lookalike, got %d: %v", len(findings), findings)
	}
}

func TestInjectDispatcher_OracleCallTargetConfirmsRealDispatchers(t *testing.T) {
	// Oracle resolves the same syntactic shape to the real
	// kotlinx.coroutines FQN — the rule fires.
	findings := runInjectDispatcherWithCallTarget(t, `
package test
import kotlinx.coroutines.Dispatchers
class Repo {
    suspend fun loadData() {
        withContext(Dispatchers.IO) { fetchFromNetwork() }
    }
}
`, "Dispatchers.IO", "kotlinx.coroutines.Dispatchers.IO")
	if len(findings) != 1 {
		t.Fatalf("expected oracle-confirmed dispatcher to fire, got %d: %v", len(findings), findings)
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
	for _, r := range api.Registry {
		if r.ID == "InjectDispatcher" {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
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
class Repo {
    suspend fun loadData() {
        withContext(Dispatchers.Default) {
            fetchFromNetwork()
        }
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	// Line 6 is "        withContext(Dispatchers.Default) {" — the Dispatchers.Default
	// should be reported on the line where it actually appears.
	if findings[0].Line != 6 {
		t.Errorf("expected finding on line 6, got line %d", findings[0].Line)
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

func TestSuspendFunInFinallySection_NonCancellableAndBuildersNotFlagged(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunInFinallySection", `
package test
suspend fun Cleanup(scope: kotlinx.coroutines.CoroutineScope) {
    try {
        work()
    } finally {
        withContext(NonCancellable) { delay(1) }
        scope.launch { }
        launch { }
        async { }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %v", findings)
	}
}

func TestSuspendFunInFinallySection_ReportsOncePerCall(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunInFinallySection", `
package test
suspend fun Cleanup() {
    try {
        work()
    } finally {
        withContext(Dispatchers.IO) { release() }
        delay(1)
    }
}
`)
	if len(findings) != 2 {
		t.Errorf("expected withContext and delay once each, got %v", findings)
	}
}

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

func TestChannelReceiveWithoutClose_DomainChannelClassIgnored(t *testing.T) {
	findings := runRuleByName(t, "ChannelReceiveWithoutClose", `
package test
import com.example.model.Channel
class Holder {
    private val channel = Channel("C1", "general")
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for a non-coroutines Channel, got %d", len(findings))
	}
}

func TestChannelReceiveWithoutClose_ConsumeAsFlowIgnored(t *testing.T) {
	findings := runRuleByName(t, "ChannelReceiveWithoutClose", `
package test
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.consumeAsFlow
class Worker {
    private val events = Channel<String>()
    fun stream() = events.consumeAsFlow()
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings when consumeAsFlow cancels the channel, got %d", len(findings))
	}
}

func TestChannelReceiveWithoutClose_ScopeBoundReceivers(t *testing.T) {
	cases := map[string]string{
		"for loop in helper launched on scope": `
class Worker(private val scope: CoroutineScope) {
    private val events = Channel<String>(Channel.UNLIMITED)
    init { scope.launch { drain() } }
    private suspend fun drain() { for (e in events) println(e) }
}`,
		"receive in launch": `
class Worker(private val scope: CoroutineScope) {
    private val events = Channel<String>()
    fun start() { scope.launch { val e = events.receive(); println(e) } }
}`,
		"receiveAsFlow collected by caller": `
class Worker {
    private val events = Channel<String>()
    val stream = events.receiveAsFlow()
}`,
	}
	for name, body := range cases {
		findings := runRuleByName(t, "ChannelReceiveWithoutClose", "package test\nimport kotlinx.coroutines.channels.Channel\n"+body)
		if len(findings) != 0 {
			t.Errorf("%s: expected no findings, got %d", name, len(findings))
		}
	}
}

func TestChannelReceiveWithoutClose_GlobalScopeReceiverStillFlagged(t *testing.T) {
	findings := runRuleByName(t, "ChannelReceiveWithoutClose", `
package test
import kotlinx.coroutines.channels.Channel
class Worker {
    private val events = Channel<String>()
    init { GlobalScope.launch { for (e in events) println(e) } }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding when the receiver runs in GlobalScope")
	}
}

func TestChannelReceiveWithoutClose_ExtensionConsumerInCancellableScope(t *testing.T) {
	cases := map[string]string{
		"extension helper in injected scope": `
class Worker(scope: CoroutineScope) {
    private val events = Channel<String>(Channel.UNLIMITED)
    init { scope.launch { events.drainInChunks(10) { println(it) } } }
}`,
		"extension helper in pooled scope": `
class Worker(groups: JobGroups) {
    private val events = Channel<String>()
    init { groups.openScope().launch { events.drainAll { println(it) } } }
}`,
		"extension helper called outside launch": `
class Worker {
    private val events = Channel<String>()
    suspend fun run() { events.drainAll { println(it) } }
}`,
	}
	for name, body := range cases {
		findings := runRuleByName(t, "ChannelReceiveWithoutClose", "package test\nimport kotlinx.coroutines.channels.Channel\n"+body)
		if len(findings) != 0 {
			t.Errorf("%s: expected no findings, got %d", name, len(findings))
		}
	}
}

func TestChannelReceiveWithoutClose_ExtensionConsumerInUnboundScopeStillFlagged(t *testing.T) {
	findings := runRuleByName(t, "ChannelReceiveWithoutClose", `
package test
import kotlinx.coroutines.channels.Channel
class Worker {
    private val events = Channel<String>()
    init { GlobalScope.launch { events.drainAll { println(it) } } }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if strings.Contains(findings[0].Message, "leaks") {
		t.Errorf("message should not claim a leak: %s", findings[0].Message)
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

func TestConcurrentModificationIteration_NegativeIteratorRemove(t *testing.T) {
	findings := runRuleByName(t, "ConcurrentModificationIteration", `
package test
fun prune(items: MutableList<String>) {
    val iterator = items.iterator()
    for (item in iterator) {
        if (item == "old") iterator.remove()
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for Iterator.remove, got %d", len(findings))
	}
}

func TestConcurrentModificationIteration_NegativeConcurrentCollection(t *testing.T) {
	findings := runRuleByName(t, "ConcurrentModificationIteration", `
package test
import java.util.concurrent.CopyOnWriteArrayList
fun prune(items: CopyOnWriteArrayList<String>) {
    for (item in items) {
        if (item == "old") items.remove(item)
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for concurrent collection, got %d", len(findings))
	}
}

func TestConcurrentModificationIteration_NegativeIterationOverCopy(t *testing.T) {
	findings := runRuleByName(t, "ConcurrentModificationIteration", `
package test
fun prune(items: MutableList<String>) {
    for (item in items.toList()) {
        if (item == "old") items.remove(item)
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for iteration over a copy, got %d", len(findings))
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

func TestCoroutineScopeCreatedButNeverCancelled_NegativeParentJobScopes(t *testing.T) {
	findings := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", `
package test
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.Dispatchers
class Cache(private val parent: CoroutineScope) {
    private val child = CoroutineScope(parent.coroutineContext + SupervisorJob(parent.coroutineContext[Job]))
    private val linked = CoroutineScope(SupervisorJob(parentJob) + Dispatchers.IO)
}
class TestHarness(backgroundScope: CoroutineScope) {
    private val scope = CoroutineScope(backgroundScope.coroutineContext)
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for scopes with a parent job, got %d", len(findings))
	}
}

func TestCoroutineScopeCreatedButNeverCancelled_NegativeCancelledElsewhereInFile(t *testing.T) {
	findings := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", `
package test
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.Dispatchers
class Cache {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}
class Owner(private val cache: Cache) {
    fun shutdown() { cache.scope.cancel() }
}
class Pool {
    private val workers = CoroutineScope(SupervisorJob() + Dispatchers.Default)
    private val all = mutableListOf<CoroutineScope>()
    init { all.add(workers) }
    fun stop() { all.forEach { it.cancel() } }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for scopes cancelled elsewhere, got %d", len(findings))
	}
}

const coroutineScopeTestHeader = `
package test
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.cancel
import kotlinx.coroutines.cancelChildren
import javax.inject.Singleton
`

func TestCoroutineScopeCreatedButNeverCancelled_CancelForms(t *testing.T) {
	cancels := map[string]string{
		"scope cancel":             "scope.cancel()",
		"context cancel":           "scope.coroutineContext.cancel()",
		"context cancelChildren":   "scope.coroutineContext.cancelChildren()",
		"job safe call cancel":     "scope.coroutineContext[Job]?.cancel()",
		"job property cancel":      "scope.coroutineContext.job.cancel()",
		"job property cancelChild": "scope.coroutineContext.job.cancelChildren()",
		"scope cancelChildren":     "scope.coroutineContext.cancelChildren()",
	}
	for name, call := range cancels {
		for _, hook := range []string{"close", "release", "onCleared", "onForgotten", "onAbandoned", "onDestroy"} {
			code := coroutineScopeTestHeader + `
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun ` + hook + `() { ` + call + ` }
}
`
			if f := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", code); len(f) != 0 {
				t.Errorf("%s in %s: expected no findings, got %d", name, hook, len(f))
			}
		}
	}
}

func TestCoroutineScopeCreatedButNeverCancelled_CancelThroughPrivateHelper(t *testing.T) {
	f := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", coroutineScopeTestHeader+`
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun release() { teardown() }
    private fun teardown() { scope.coroutineContext.job.cancel() }
}
`)
	if len(f) != 0 {
		t.Errorf("expected no findings, got %d", len(f))
	}
}

func TestCoroutineScopeCreatedButNeverCancelled_ProcessLifetimeSingletonSkipped(t *testing.T) {
	cases := map[string]string{
		"javax singleton": `
@Singleton
class Registry {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}`,
		"app scope singleton": `
@SingleIn(AppScope::class)
class Registry {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}`,
		"app scope singleton with hook": `
@SingleIn(AppScope::class)
class Registry {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun close() {}
}`,
		"narrower singleton without hook": `
@SingleIn(SessionScope::class)
class Registry {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}`,
	}
	for name, body := range cases {
		if f := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", coroutineScopeTestHeader+body); len(f) != 0 {
			t.Errorf("%s: expected no findings, got %d", name, len(f))
		}
	}
}

func TestCoroutineScopeCreatedButNeverCancelled_TeardownResetExtensionAndContributions(t *testing.T) {
	skipped := map[string]string{
		"reset in detach": `
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun detach() { scope.reset() }
}`,
		"cancelAll in onCleared": `
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun onCleared() { scope.cancelAll() }
}`,
		"context clear in close": `
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun close() { scope.coroutineContext.clear() }
}`,
		"contributes binding without hook": `
@ContributesBinding(SessionScope::class)
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}`,
		"contributes to without hook": `
@ContributesTo(SessionScope::class)
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}`,
	}
	for name, body := range skipped {
		if f := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", coroutineScopeTestHeader+body); len(f) != 0 {
			t.Errorf("%s: expected no findings, got %d", name, len(f))
		}
	}
	reported := map[string]string{
		"reset outside teardown": `
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun refresh() { scope.reset() }
}`,
		"reset on another receiver": `
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    private val other = Cache()
    fun close() { other.reset() }
}`,
		"contributes binding with hook that never cancels": `
@ContributesBinding(SessionScope::class)
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun close() {}
}`,
	}
	for name, body := range reported {
		if f := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", coroutineScopeTestHeader+body); len(f) != 1 {
			t.Errorf("%s: expected 1 finding, got %d", name, len(f))
		}
	}
}

func TestCoroutineScopeCreatedButNeverCancelled_StillReported(t *testing.T) {
	cases := map[string]string{
		"plain class": `
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
}`,
		"lifecycle hook that never cancels": `
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun close() { println("closed") }
}`,
		"narrower singleton with hook that never cancels": `
@SingleIn(SessionScope::class)
class Holder {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    fun onCleared() {}
}`,
		"scope handed to stateIn only": `
class Holder(private val source: Flow<Int>) {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    val state = source.stateIn(scope, SharingStarted.Eagerly, 0)
}`,
	}
	for name, body := range cases {
		f := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", coroutineScopeTestHeader+body)
		if len(f) != 1 {
			t.Errorf("%s: expected 1 finding, got %d", name, len(f))
		}
		for _, x := range f {
			if strings.Contains(x.Message, "leaks") {
				t.Errorf("%s: message should not claim a leak: %s", name, x.Message)
			}
		}
	}
}

func TestCoroutineScopeCreatedButNeverCancelled_HandedToLongLivedConsumer(t *testing.T) {
	f := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", coroutineScopeTestHeader+`
class Prefs(private val context: Context) {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    val store = PreferenceDataStoreFactory.create(scope = scope) { context.file() }
}
class Other(private val context: Context) {
    private val scope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
    val store = Helper.create(scope) 
}
`)
	if len(f) != 0 {
		t.Errorf("expected no findings, got %d", len(f))
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

func TestSynchronizedOnNonFinal_ShadowedByParameterLocalAndLoopVariables(t *testing.T) {
	findings := runRuleByName(t, "SynchronizedOnNonFinal", `
package test
class Shadow {
    private var lock = Any()
    fun param(lock: Any) { synchronized(lock) { } }
    fun local() { val lock = Any(); synchronized(lock) { } }
    fun loop() { for (lock in listOf(Any())) { synchronized(lock) { } } }
    fun lambda() { listOf(Any()).forEach { lock -> synchronized(lock) { } } }
}
`)
	if len(findings) != 0 {
		t.Errorf("shadowed names are not the var property, got %v", findings)
	}
}

func TestSynchronizedOnNonFinal_ConstructorVarAndTopLevelVar(t *testing.T) {
	ctor := runRuleByName(t, "SynchronizedOnNonFinal", `
package test
class Ctor(private var lock: Any) {
    fun f() { synchronized(lock) { } }
}
`)
	if len(ctor) != 1 {
		t.Errorf("expected 1 finding for constructor var, got %v", ctor)
	}
	top := runRuleByName(t, "SynchronizedOnNonFinal", `
package test
private var lock = Any()
fun f() { synchronized(lock) { } }
`)
	if len(top) != 1 {
		t.Errorf("expected 1 finding for top-level var, got %v", top)
	}
}

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

func TestVolatileMissingOnDcl_NegativeOnlyCheckedUnderLock(t *testing.T) {
	findings := runRuleByName(t, "VolatileMissingOnDcl", `
package test
class Registry {
    private var cache: Cache? = null
    private val lock = Any()

    fun get(): Cache = synchronized(lock) {
        if (cache == null) { cache = Cache() }
        cache!!
    }

    @Synchronized
    fun reset() {
        if (cache == null) return
        cache = null
    }

    private fun clearLocked() {
        if (cache == null) return
        cache = null
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings when every check is under a lock, got %d", len(findings))
	}
}

func TestVolatileMissingOnDcl_NegativeLocalShadowing(t *testing.T) {
	// Property `instance` is a non-null val; the `instance == null` checks
	// inside the methods refer to *local* vals, not the class property. The
	// substring-based heuristic incorrectly flagged this; the AST-aware
	// counter must not.
	findings := runRuleByName(t, "VolatileMissingOnDcl", `
package test
class Holder {
    private var instance: String = ""
    fun first(): String? {
        val instance: String? = compute()
        if (instance == null) return null
        synchronized(this) {
            val instance: String? = compute()
            if (instance == null) return null
            return instance
        }
    }
    private fun compute(): String? = null
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings when null checks reference local shadows, got %d", len(findings))
	}
}

func TestVolatileMissingOnDcl_NegativeUnrelatedNullChecks(t *testing.T) {
	// Two `instance == null` checks referencing local function parameters
	// in unrelated methods must not trigger DCL detection.
	findings := runRuleByName(t, "VolatileMissingOnDcl", `
package test
class Holder {
    private var instance: String? = null
    fun a(instance: String?) {
        synchronized(this) {
            if (instance == null) println("a")
        }
    }
    fun b(instance: String?) {
        if (instance == null) println("b")
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings when null checks bind to parameters, got %d", len(findings))
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

func TestMutableStateInObject_AccessorOnlyPropertyHasNoState(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
interface Config { var name: String? }
object NoOpConfig : Config {
    override var name: String?
        get() = null
        set(_) = Unit
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for a property without a backing field, got %d", len(findings))
	}
	findings = runRuleByName(t, "MutableStateInObject", `
package test
object Holder {
    var name: String? = null
        get() = field
        set(value) { field = value }
    var other: Int
        get() = 1
        set(value) { field = value }
}
`)
	if len(findings) != 2 {
		t.Errorf("expected findings for properties that use field, got %d", len(findings))
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

func TestMutableStateInObject_IgnoresPrivateStateInSynchronizedFunction(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
object Counter {
    private var total = 0

    @Synchronized
    fun next(): Int {
        total++
        return total
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected @Synchronized private state to be clean, got %d", len(findings))
	}
}

func TestMutableStateInObject_IgnoresPrivateHelperOnlyCalledFromSynchronizedFunction(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
object Counter {
    private var total = 0

    @Synchronized
    fun next(): Int {
        return increment()
    }

    private fun increment(): Int {
        total++
        return total
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected private helper reached through @Synchronized function to be clean, got %d", len(findings))
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

func TestMutableStateInObject_FlagsPrivateHelperCalledFromUnsynchronizedFunction(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
object Counter {
    private var total = 0

    fun next(): Int {
        return increment()
    }

    private fun increment(): Int {
        total++
        return total
    }
}
`)
	if len(findings) == 0 {
		t.Error("expected finding when private helper is reached from an unsynchronized function")
	}
}

func TestMutableStateInObject_NoTypeOracleCapability(t *testing.T) {
	for _, rule := range api.Registry {
		if rule.ID != "MutableStateInObject" {
			continue
		}
		if rule.Needs.Has(api.NeedsResolver) || rule.Needs.Has(api.NeedsTypeInfo) || rule.Needs.Has(api.NeedsOracle) ||
			rule.Oracle != nil {
			t.Fatalf("MutableStateInObject should remain AST-only, got Needs=%b Oracle=%+v",
				rule.Needs, rule.Oracle)
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

func TestStateFlowMutableLeak_NegativePrivateContainers(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test
import kotlinx.coroutines.flow.MutableStateFlow

class Replayer {
    private inner class Run {
        val state = MutableStateFlow(0)
    }
}

private class Hidden {
    val state = MutableStateFlow(1)
}

internal object Registry {
    val state = MutableStateFlow(2)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for members of non-public containers, got %d: %v", len(findings), findings)
	}
}

func TestStateFlowMutableLeak_PositiveInnerClassOfPublicContainer(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test
import kotlinx.coroutines.flow.MutableStateFlow

class Replayer {
    inner class Run {
        val state = MutableStateFlow(0)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
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

func TestStateFlowMutableLeak_NegativeTestSupport(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test
import androidx.annotation.VisibleForTesting
import kotlinx.coroutines.flow.MutableStateFlow
class FakeRepo {
    val state = MutableStateFlow(0)
}
class TestHarness {
    val state = MutableStateFlow(0)
}
@VisibleForTesting
class Exposed {
    val state = MutableStateFlow(0)
}
class Holder {
    @VisibleForTesting
    val state = MutableStateFlow(0)
}
`)
	if len(findings) != 0 {
		t.Errorf("expected test support classes to be ignored, got %d", len(findings))
	}
}

const stateFlowCustomSupportSource = `
package test
import kotlinx.coroutines.flow.MutableStateFlow
class WidgetDriver {
    val state = MutableStateFlow(0)
}
`

func TestStateFlowMutableLeak_PositiveCustomSupportNameNotConfiguredByDefault(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", stateFlowCustomSupportSource)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding without configured names, got %d", len(findings))
	}
}

func TestStateFlowMutableLeak_NegativeConfiguredTestSupportClassName(t *testing.T) {
	var impl *rules.StateFlowMutableLeakRule
	for _, r := range api.Registry {
		if r.ID == "StateFlowMutableLeak" {
			impl = r.Implementation.(*rules.StateFlowMutableLeakRule)
		}
	}
	if impl == nil {
		t.Fatal("StateFlowMutableLeak not registered")
	}
	saved := impl.TestSupportClassNames
	t.Cleanup(func() { impl.TestSupportClassNames = saved })
	impl.TestSupportClassNames = []string{"Driver"}
	if findings := runRuleByName(t, "StateFlowMutableLeak", stateFlowCustomSupportSource); len(findings) != 0 {
		t.Errorf("expected a configured name to mark test support, got %d", len(findings))
	}
}

func TestStateFlowMutableLeak_NegativeTestFixturesSourceSet(t *testing.T) {
	findings := runRuleByNameOnPath(t, "StateFlowMutableLeak", "feature/src/testFixtures/kotlin/Repo.kt", `
package test
import kotlinx.coroutines.flow.MutableStateFlow
class Repo {
    val state = MutableStateFlow(0)
}
`)
	if len(findings) != 0 {
		t.Errorf("expected testFixtures source set to be ignored, got %d", len(findings))
	}
}

func TestStateFlowMutableLeak_NegativeFunctionTypedProperty(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test
import kotlinx.coroutines.flow.MutableStateFlow
class VM {
    val factory: () -> Any = { MutableStateFlow(0) }
    val inferred = { MutableStateFlow(0) }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected function-typed properties to be ignored, got %d", len(findings))
	}
}

func TestStateFlowMutableLeak_PositiveDeclaredType(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test
import kotlinx.coroutines.flow.MutableStateFlow
class VM {
    val state: MutableStateFlow<Int> = MutableStateFlow(0)
}
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding for declared MutableStateFlow type, got %d", len(findings))
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
    state.distinctUntilChanged().collect { render(it) }
}
`)
	if len(findings) != 1 {
		t.Errorf("expected finding for distinctUntilChanged on a StateFlow, got %v", findings)
	}
}

func TestStateFlowCompareByReference_ConstructorAndAsStateFlow(t *testing.T) {
	findings := runRuleByName(t, "StateFlowCompareByReference", `
package test
class Holder {
    private val backing = MutableStateFlow(0)
    val exposed = backing.asStateFlow()
    fun a() = exposed.distinctUntilChanged()
    fun b() = backing.asStateFlow().distinctUntilChanged()
}
`)
	if len(findings) == 0 {
		t.Errorf("expected findings for StateFlow receivers")
	}
}

func TestStateFlowCompareByReference_MapBeforeDistinctIsNeeded(t *testing.T) {
	findings := runRuleByName(t, "StateFlowCompareByReference", `
package test
import kotlinx.coroutines.flow.StateFlow
fun observe(state: StateFlow<UiState>, uiState: Flow<Int>) {
    state.map { it.count }.distinctUntilChanged().collect { render(it) }
    uiState.distinctUntilChanged().collect { render(it) }
}
`)
	if len(findings) != 0 {
		t.Errorf("map on a StateFlow returns a plain Flow, so distinctUntilChanged is needed; got %v", findings)
	}
}

// --- GlobalScopeLaunchInViewModel ---

func TestGlobalScopeLaunchInViewModel_NegativeOptInDelicateApi(t *testing.T) {
	findings := runRuleByName(t, "GlobalScopeLaunchInViewModel", `
package test
import kotlinx.coroutines.DelicateCoroutinesApi
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch

class SyncViewModel {
    @OptIn(DelicateCoroutinesApi::class)
    fun flush() {
        GlobalScope.launch { upload() }
    }
}

@OptIn(DelicateCoroutinesApi::class)
class ExportViewModel {
    fun export() {
        GlobalScope.launch { upload() }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings with @OptIn(DelicateCoroutinesApi::class), got %d", len(findings))
	}
}

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

// --- WithContextInSuspendFunctionNoop regression tests ---

// Different dispatchers — inner switch is meaningful.
func TestWithContextInSuspendFunctionNoop_NegativeDifferentDispatchers(t *testing.T) {
	findings := runRuleByName(t, "WithContextInSuspendFunctionNoop", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
suspend fun loadData() {
    withContext(Dispatchers.IO) {
        withContext(Dispatchers.Main) {
            updateUi()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// Injected dispatcher variable — cannot prove symbolic equality, must stay quiet.
func TestWithContextInSuspendFunctionNoop_NegativeInjectedDispatcher(t *testing.T) {
	findings := runRuleByName(t, "WithContextInSuspendFunctionNoop", `
package test
import kotlinx.coroutines.CoroutineDispatcher
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
class Repo(private val io: CoroutineDispatcher) {
    suspend fun loadData() {
        withContext(io) {
            withContext(Dispatchers.IO) {
                fetch()
            }
        }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// Local function named withContext shadows kotlinx.coroutines.withContext.
func TestWithContextInSuspendFunctionNoop_NegativeLocalShadow(t *testing.T) {
	findings := runRuleByName(t, "WithContextInSuspendFunctionNoop", `
package test
import kotlinx.coroutines.Dispatchers
suspend fun withContext(d: Any, block: suspend () -> Unit) { block() }
suspend fun loadData() {
    withContext(Dispatchers.IO) {
        withContext(Dispatchers.IO) {
            fetch()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// Missing kotlinx.coroutines.withContext import — bare call could be anything.
func TestWithContextInSuspendFunctionNoop_NegativeMissingImport(t *testing.T) {
	findings := runRuleByName(t, "WithContextInSuspendFunctionNoop", `
package test
import kotlinx.coroutines.Dispatchers
suspend fun loadData() {
    withContext(Dispatchers.IO) {
        withContext(Dispatchers.IO) {
            fetch()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// Intervening launch — different coroutine context, no redundancy.
func TestWithContextInSuspendFunctionNoop_NegativeInterveningLaunch(t *testing.T) {
	findings := runRuleByName(t, "WithContextInSuspendFunctionNoop", `
package test
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
suspend fun loadData(scope: CoroutineScope) {
    withContext(Dispatchers.IO) {
        scope.launch {
            withContext(Dispatchers.IO) {
                fetch()
            }
        }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// Intervening coroutineScope { ... } — child coroutine builder, not a withContext;
// don't bleed comparison through it.
func TestWithContextInSuspendFunctionNoop_NegativeInterveningCoroutineScope(t *testing.T) {
	findings := runRuleByName(t, "WithContextInSuspendFunctionNoop", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.withContext
suspend fun loadData() {
    withContext(Dispatchers.IO) {
        coroutineScope {
            withContext(Dispatchers.IO) {
                fetch()
            }
        }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// Nested local function declaration boundary — inner withContext is in a
// separate function body, do not compare against outer suspend function.
func TestWithContextInSuspendFunctionNoop_NegativeNestedFunction(t *testing.T) {
	findings := runRuleByName(t, "WithContextInSuspendFunctionNoop", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
suspend fun loadData() {
    withContext(Dispatchers.IO) {
        suspend fun helper() {
            withContext(Dispatchers.IO) {
                fetch()
            }
        }
        helper()
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// Comment containing the words "withContext" must not match anything; AST-based
// dispatch already enforces this but lock the regression.
func TestWithContextInSuspendFunctionNoop_NegativeCommentLookalike(t *testing.T) {
	findings := runRuleByName(t, "WithContextInSuspendFunctionNoop", `
package test
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext
suspend fun loadData() {
    // withContext(Dispatchers.IO) { withContext(Dispatchers.IO) { } }
    val msg = "withContext(Dispatchers.IO) inside withContext(Dispatchers.IO)"
    withContext(Dispatchers.IO) {
        println(msg)
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

// Capability declaration must stay narrow: this rule does not need oracle or
// type info.
func TestWithContextInSuspendFunctionNoop_StaysASTOnly(t *testing.T) {
	var rule *api.Rule
	for _, r := range api.Registry {
		if r.ID == "WithContextInSuspendFunctionNoop" {
			rule = r
			break
		}
	}
	if rule == nil {
		t.Fatal("WithContextInSuspendFunctionNoop not registered")
	}
	if rule.Needs != 0 {
		t.Fatalf("WithContextInSuspendFunctionNoop Needs=%v, want 0 (AST + imports only)", rule.Needs)
	}
	if rule.Oracle != nil {
		t.Fatalf("WithContextInSuspendFunctionNoop unexpectedly carries oracle metadata: %+v",
			rule.Oracle)
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

// --- SuspendFunSwallowedCancellation ---

func TestSuspendFunSwallowedCancellation_RealRethrow(t *testing.T) {
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
		t.Fatalf("expected 0 findings when caught exception is actually rethrown, got %d:\n%v", len(findings), findings)
	}
}

func TestSuspendFunSwallowedCancellation_CommentedRethrow(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunSwallowedCancellation", `
package test
import kotlinx.coroutines.CancellationException
suspend fun doWork() {
    try {
        delay(1000)
    } catch (e: CancellationException) {
        println("cancelled")
        // throw e — disabled while debugging
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when rethrow only appears in a comment, got %d:\n%v", len(findings), findings)
	}
}

func TestSuspendFunSwallowedCancellation_RethrowInStringLiteral(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunSwallowedCancellation", `
package test
import kotlinx.coroutines.CancellationException
suspend fun doWork() {
    try {
        delay(1000)
    } catch (e: CancellationException) {
        println("note: caller can throw e to surface this")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding when 'throw e' only appears in a string literal, got %d:\n%v", len(findings), findings)
	}
}

func TestMutableStateInObject_IgnoresPrivateVarWrittenOnlyByTestHook(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
object Config {
    private var testing = false

    @VisibleForTesting
    fun setIsTesting(value: Boolean) { testing = value }

    fun isTesting(): Boolean = testing
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestMutableStateInObject_IgnoresVarResetInAfter(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
object Flags {
    var enabled = false

    class Resetter {
        @After fun tearDown() { Flags.enabled = false }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestMutableStateInObject_IgnoresSamplePath(t *testing.T) {
	findings := runRuleByNameOnPath(t, "MutableStateInObject", "app/sample/src/main/Counter.kt", `
package test
object Counter {
    var total = 0
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestMutableStateInObject_FlagsReadModifyWriteInProduction(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
object Counter {
    private var total = 0

    fun bump() { total++ }
}
`)
	if len(findings) != 1 {
		t.Errorf("expected 1 finding, got %d", len(findings))
	}
}

func TestRedundantSuspendModifier_ProjectSuspendDeclaredInOtherFileSuppresses(t *testing.T) {
	code := `
package test
suspend fun f() {
    loadAll()
}
`
	other := []string{"package test\nsuspend fun loadAll() {}\n"}
	suspendByCall := map[string]bool{"loadAll()": false}
	if findings := runRedundantSuspendModifierWithProjectFiles(t, code, other, suspendByCall); len(findings) != 0 {
		t.Fatalf("expected no finding when another file declares suspend loadAll, got %d: %v", len(findings), findings)
	}
	if findings := runRedundantSuspendModifierWithProjectFiles(t, code, nil, suspendByCall); len(findings) != 1 {
		t.Fatalf("expected a finding when no file declares suspend loadAll, got %d: %v", len(findings), findings)
	}
}

func TestRedundantSuspendModifier_ChainedCallsDoNotTrustNonSuspendVerdict(t *testing.T) {
	cases := map[string]string{
		"first":     "flow.first()",
		"awaitItem": "turbine.awaitItem()",
		"property":  "repo.load()",
	}
	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			code := "package test\nsuspend fun f(flow: Flow<Int>, turbine: Turbine<Int>) {\n    " + call + "\n}\n"
			findings := runRedundantSuspendModifierWithProjectFiles(t, code, nil, map[string]bool{call: false})
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
			}
		})
	}
}

func TestRedundantSuspendModifier_LocalReceiverNonSuspendVerdictStillFlags(t *testing.T) {
	code := `
package test
suspend fun f(): Int {
    val items = listOf(1, 2)
    return items.size()
}
`
	findings := runRedundantSuspendModifierWithProjectFiles(t, code, nil, map[string]bool{"items.size()": false})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func runSuspendFunWithFlowReturnTypeWithProjectFiles(t *testing.T, code string, otherCode []string) []scanner.Finding {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	indexed := []*scanner.File{file}
	for _, other := range otherCode {
		indexed = append(indexed, parseInline(t, other))
	}
	resolver.IndexFilesParallel(indexed, 1)
	for _, r := range api.Registry {
		if r.ID == "SuspendFunWithFlowReturnType" {
			cols := rules.NewDispatcher([]*api.Rule{r}, resolver).Run(file)
			return cols.Findings()
		}
	}
	t.Fatal("SuspendFunWithFlowReturnType not found in registry")
	return nil
}

func TestSuspendFunWithFlowReturnType_SuspendCalleeDeclaredInOtherFileKeepsModifier(t *testing.T) {
	code := `
package test
suspend fun events(): Flow<Int> {
    val config = loadConfig()
    return flowOf(config)
}
`
	other := []string{"package test\nsuspend fun loadConfig(): Int = 1\n"}
	if findings := runSuspendFunWithFlowReturnTypeWithProjectFiles(t, code, other); len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
	if findings := runSuspendFunWithFlowReturnTypeWithProjectFiles(t, code, nil); len(findings) != 1 {
		t.Fatalf("expected 1 finding without a project suspend declaration, got %d: %v", len(findings), findings)
	}
}

func TestSuspendFunWithFlowReturnType_ChainedFirstCallKeepsModifier(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunWithFlowReturnType", `
package test
suspend fun events(source: Flow<Int>): Flow<Int> {
    val head = source.first()
    return flowOf(head)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestStateFlowMutableLeak_DocumentedMutableInterfaceAndMutableNamedTypesIgnored(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test
import kotlinx.coroutines.flow.MutableStateFlow
/** Mutable by design: callers push state through it. */
interface Pusher {
    val state: MutableStateFlow<Int>
}
interface Other {
    /** Mutable on purpose. */
    val state: MutableStateFlow<Int>
}
class MutableBox {
    val state = MutableStateFlow(0)
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %d", len(findings))
	}
}

func TestGlobalScopeLaunchInViewModel_FileLevelOptInIgnored(t *testing.T) {
	findings := runRuleByName(t, "GlobalScopeLaunchInViewModel", `
@file:OptIn(DelicateCoroutinesApi::class)
package test
import kotlinx.coroutines.DelicateCoroutinesApi
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch
class FooViewModel { fun a() { GlobalScope.launch { } } }
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings with a file-level OptIn, got %d", len(findings))
	}
}

func TestConcurrentModificationIteration_ClassLevelConcurrentCollectionIgnored(t *testing.T) {
	findings := runRuleByName(t, "ConcurrentModificationIteration", `
package test
import java.util.concurrent.CopyOnWriteArrayList
class Holder {
    private val list = CopyOnWriteArrayList<Int>()
    fun f() {
        for (x in list) {
            list.remove(x)
        }
    }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings for a class-level CopyOnWriteArrayList, got %d", len(findings))
	}
}

func TestMutableStateInObject_TestDoubleObjectIgnored(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
object FakeRegistry { var counter = 0 }
object TestHelpers { var counter = 0 }
object Registry { var counter = 0 }
`)
	if len(findings) != 1 {
		t.Errorf("expected only Registry to be flagged, got %d", len(findings))
	}
}

func TestCoroutineLaunchedInTest_OnlyBareBuilders(t *testing.T) {
	findings := runRuleByName(t, "CoroutineLaunchedInTestWithoutRunTest", `
package test
class T {
    @Test fun builder() { launch { } }
    @Test fun prefixed() { launchAnimation(); asyncWork() }
    @Test fun wrapped() = runBlocking { launch { } }
}
`)
	if len(findings) != 1 {
		t.Errorf("expected only the bare launch to be flagged, got %v", findings)
	}
}

func TestSuspendFunWithCoroutineScopeReceiver_GenericNullableAndQualifiedReceivers(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunWithCoroutineScopeReceiver", `
package test
suspend fun <T> CoroutineScope.generic(x: T) {}
suspend fun CoroutineScope?.nullable() {}
suspend fun kotlinx.coroutines.CoroutineScope.qualified() {}
suspend fun Other.notScope() {}
suspend fun plain(scope: CoroutineScope) {}
fun CoroutineScope.notSuspend() {}
`)
	if len(findings) != 3 {
		t.Errorf("expected 3 findings, got %v", findings)
	}
}

func TestSuspendFunSwallowedCancellation_EarlierCancellationCatchAndEnsureActive(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunSwallowedCancellation", `
package test
suspend fun a() {
    try { delay(1) } catch (e: CancellationException) { throw e } catch (e: Exception) { log(e) }
}
suspend fun b() {
    try { delay(1) } catch (e: Exception) { currentCoroutineContext().ensureActive(); log(e) }
}
`)
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %v", findings)
	}
}

func TestSuspendFunSwallowedCancellation_BroadCatchHasNoRethrowFix(t *testing.T) {
	findings := runRuleByName(t, "SuspendFunSwallowedCancellation", `
package test
suspend fun a() {
    try { delay(1) } catch (e: Exception) { log(e) }
}
suspend fun b() {
    try { delay(1) } catch (e: CancellationException) { log(e) }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %v", findings)
	}
	if findings[0].Fix != nil {
		t.Errorf("broad catch must not get a blanket rethrow fix")
	}
	if findings[1].Fix == nil {
		t.Errorf("catching CancellationException itself keeps the rethrow fix")
	}
}

func TestConcurrentModificationIteration_MutationFollowedByExitIsSafe(t *testing.T) {
	findings := runRuleByName(t, "ConcurrentModificationIteration", `
package test
class Iter {
    private val items = mutableListOf(1)
    fun safe() { for (i in items) { if (i == 1) { items.remove(i); break } } }
    fun safeReturn() { for (i in items) { items.remove(i); return } }
    fun unsafe() { for (i in items) { items.remove(i) } }
}
`)
	if len(findings) != 1 {
		t.Errorf("expected only the unsafe loop, got %v", findings)
	}
}

func TestSleepInsteadOfDelay_MatchesDetektShapes(t *testing.T) {
	flagged := map[string]string{
		"qualified call":     "suspend fun a() { java.lang.Thread.sleep(10) }",
		"callable reference": "suspend fun a() { val f = Thread::sleep }",
		"flow builder":       "fun a() = flow<Int> { Thread.sleep(10) }",
		"callbackFlow":       "fun a() = callbackFlow<Int> { Thread.sleep(10) }",
		"runTest":            "fun a() = runTest { Thread.sleep(10) }",
		"split across lines": "suspend fun a() { Thread\n        .sleep(10) }",
	}
	for name, src := range flagged {
		if got := runRuleByName(t, "SleepInsteadOfDelay", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		} else if got[0].Message != "This use of Thread.sleep() inside a suspend function should be replaced by delay()." {
			t.Errorf("%s: message %q is not detekt's", name, got[0].Message)
		}
	}
	clean := map[string]string{
		"plain function":     "fun a() { Thread.sleep(10) }",
		"other receiver":     "suspend fun a(t: Foo) { t.sleep(10) }",
		"plain lambda":       "fun a() = listOf(1).map { Thread.sleep(10) }",
		"other class":        "suspend fun a() { MyThread.sleep(10) }",
		"unqualified sleep":  "suspend fun a() { sleep(10) }",
		"non-sleep callable": "suspend fun a() { val f = Thread::yield }",
	}
	for name, src := range clean {
		if got := runRuleByName(t, "SleepInsteadOfDelay", "package test\n"+src+"\n"); len(got) != 0 {
			t.Errorf("%s: want no findings, got %d", name, len(got))
		}
	}
}

func TestSleepInsteadOfDelay_FixOnlyWithImportAndSingleArgument(t *testing.T) {
	withImport := runRuleByName(t, "SleepInsteadOfDelay", `
package test
import kotlinx.coroutines.delay
suspend fun a() { Thread.sleep(10) }
suspend fun b() { Thread.sleep(10, 5) }
`)
	if len(withImport) != 2 || withImport[0].Fix == nil || withImport[1].Fix != nil {
		t.Errorf("expected a fix only for the one-argument call, got %v", withImport)
	}
	noImport := runRuleByName(t, "SleepInsteadOfDelay", `
package test
suspend fun a() { Thread.sleep(10) }
`)
	if len(noImport) != 1 || noImport[0].Fix != nil {
		t.Errorf("expected a finding without a fix when delay is not imported, got %v", noImport)
	}
}

func TestCoroutineLaunchedInTest_AnnotationMatchIsExact(t *testing.T) {
	findings := runRuleByName(t, "CoroutineLaunchedInTestWithoutRunTest", `
package test
class T {
    @TestOnly fun helper() { launch { } }
    @TestFactory fun factory() { launch { } }
    @org.junit.Test fun real() { launch { } }
}
`)
	if len(findings) != 1 {
		t.Errorf("only the real @Test function should be checked, got %v", findings)
	}
}

func TestStateFlowMutableLeak_InterfaceMemberMessageSurvivesAnnotatedSupertype(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test
import kotlinx.coroutines.flow.MutableStateFlow

class Contract {
  @SuppressLint("x")
  interface Presenter :
    @Suppress("DEPRECATION") ListPresenter<NoteView>, NoteEditor {
    val state: MutableStateFlow<Int>
  }

  class Holder {
    val other = MutableStateFlow(0)
  }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected two findings, got %d", len(findings))
	}
	for _, f := range findings {
		isInterfaceMember := strings.Contains(f.Message, "'state'")
		if isInterfaceMember && strings.Contains(f.Message, "Keep it private") {
			t.Errorf("interface member must not be told to keep it private: %s", f.Message)
		}
		if !isInterfaceMember && !strings.Contains(f.Message, "Keep it private") {
			t.Errorf("class member should keep the private remedy: %s", f.Message)
		}
	}
}

func TestStateFlowMutableLeak_InterfaceMemberMessageSurvivesQualifiedAnnotatedSupertype(t *testing.T) {
	findings := runRuleByName(t, "StateFlowMutableLeak", `
package test

import kotlinx.coroutines.flow.MutableStateFlow

/** Contract. */
class Contract {

  @SuppressLint("x")
  interface Presenter :
    @Suppress("DEPRECATION") some.pkg.ListPresenter<NoteView>, NoteEditor {

    /** The channel. */
    val noteId: MutableStateFlow<String?>

    val state: MutableStateFlow<Int>

    fun focused(): Boolean
  }

  interface View : BaseView<Presenter> {
    fun exit()
  }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected two findings, got %d", len(findings))
	}
	for _, f := range findings {
		if strings.Contains(f.Message, "Keep it private") {
			t.Errorf("interface member must not be told to keep it private: %s", f.Message)
		}
	}
}

func TestRedundantSuspendModifier_OracleVerdictBeatsCoroutinesPackagePrefix(t *testing.T) {
	code := `
package test
suspend fun f() = flowOf(1)
`
	// The compiler resolved flowOf to a kotlinx.coroutines function that is not suspend.
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.CallTargets[file.Path] = map[string]string{}
	fake.CallTargetSuspend[file.Path] = map[string]bool{}
	file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
		key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
		fake.CallTargets[file.Path][key] = "kotlinx.coroutines.flow.flowOf"
		fake.CallTargetSuspend[file.Path][key] = false
	})
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == "RedundantSuspendModifier" {
			d := rules.NewDispatcher([]*api.Rule{r}, composite)
			cols := d.Run(file)
			if got := cols.Findings(); len(got) != 1 {
				t.Fatalf("want 1 finding for a non-suspend kotlinx.coroutines call, got %v", got)
			}
			return
		}
	}
	t.Fatal("RedundantSuspendModifier not registered")
}

func TestSuspendFunSwallowedCancellation_RunCatchingWithSuspendCall(t *testing.T) {
	run := func(code string, suspendCalls map[string]bool) []scanner.Finding {
		file := parseInline(t, code)
		resolver := typeinfer.NewResolver()
		resolver.IndexFilesParallel([]*scanner.File{file}, 1)
		fake := oracle.NewFakeOracle()
		fake.CallTargets[file.Path] = map[string]string{}
		fake.CallTargetSuspend[file.Path] = map[string]bool{}
		file.FlatWalkNodes(0, "call_expression", func(idx uint32) {
			text := strings.TrimSpace(file.FlatNodeText(idx))
			key := fmt.Sprintf("%d:%d", file.FlatRow(idx)+1, file.FlatCol(idx)+1)
			switch {
			case strings.HasPrefix(text, "runCatching"):
				fake.CallTargets[file.Path][key] = "kotlin.runCatching"
				fake.CallTargetSuspend[file.Path][key] = false
			case suspendCalls[text]:
				fake.CallTargets[file.Path][key] = "kotlinx.coroutines.delay"
				fake.CallTargetSuspend[file.Path][key] = true
			default:
				fake.CallTargets[file.Path][key] = "kotlin.io.println"
				fake.CallTargetSuspend[file.Path][key] = false
			}
		})
		composite := oracle.NewCompositeResolver(fake, resolver)
		for _, r := range api.Registry {
			if r.ID == "SuspendFunSwallowedCancellation" {
				d := rules.NewDispatcher([]*api.Rule{r}, composite)
				cols := d.Run(file)
				return cols.Findings()
			}
		}
		t.Fatal("SuspendFunSwallowedCancellation not registered")
		return nil
	}
	got := run("package test\nsuspend fun f() {\n    runCatching { delay(1) }\n}\n", map[string]bool{"delay(1)": true})
	if len(got) != 1 || got[0].Line != 3 {
		t.Fatalf("runCatching around a suspend call: want 1 finding on line 3, got %v", got)
	}
	got = run("package test\nsuspend fun f() {\n    runCatching { println() }\n}\n", nil)
	if len(got) != 0 {
		t.Fatalf("runCatching without a suspend call: want no findings, got %v", got)
	}
}

func TestSuspendFunWithFlowReturnType_NullableAndQualifiedTypes(t *testing.T) {
	cases := map[string]string{
		"nullable flow":    "suspend fun f(): Flow<Int>? = null",
		"qualified flow":   "suspend fun f(): kotlinx.coroutines.flow.Flow<Int> = flowOf(1)",
		"extension flow":   "suspend fun String.f(): Flow<Int> = flowOf(1)",
		"state flow alias": "suspend fun f(): StateFlow<Int> = state",
	}
	for name, src := range cases {
		if got := runRuleByName(t, "SuspendFunWithFlowReturnType", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	if got := runRuleByName(t, "SuspendFunWithFlowReturnType", "package test\nsuspend fun f(): List<Flow<Int>> = emptyList()\n"); len(got) != 0 {
		t.Errorf("a list of flows is not a flow: want no findings, got %d", len(got))
	}
}

func TestSuspendFunWithCoroutineScopeReceiver_FollowsSupertypes(t *testing.T) {
	flagged := map[string]string{
		"type parameter bound": "suspend fun <T : CoroutineScope> T.f() {}",
		"parenthesized":        "suspend fun (CoroutineScope).f() {}",
		"typealias":            "typealias Sc = CoroutineScope\nsuspend fun Sc.f() {}",
		"project subclass":     "class MyScope : CoroutineScope { override val coroutineContext get() = TODO() }\nsuspend fun MyScope.f() {}",
	}
	for name, src := range flagged {
		if got := runRuleByNameWithResolver(t, "SuspendFunWithCoroutineScopeReceiver", "package test\n"+src+"\n"); len(got) != 1 {
			t.Errorf("%s: want 1 finding, got %d", name, len(got))
		}
	}
	if got := runRuleByNameWithResolver(t, "SuspendFunWithCoroutineScopeReceiver", "package test\nsuspend fun String.f() {}\nsuspend fun List<CoroutineScope>.g() {}\n"); len(got) != 0 {
		t.Errorf("other receivers: want no findings, got %d", len(got))
	}
}

func TestGlobalCoroutineUsage_OnlyLaunchAndAsync(t *testing.T) {
	if got := runRuleByName(t, "GlobalCoroutineUsage", "package test\nfun f() = GlobalScope.coroutineContext\n"); len(got) != 0 {
		t.Errorf("a GlobalScope property access is not launch or async: want no findings, got %d", len(got))
	}
	if got := runRuleByName(t, "GlobalCoroutineUsage", "package test\nfun f() { GlobalScope.launch { } }\n"); len(got) != 1 {
		t.Errorf("GlobalScope.launch: want 1 finding, got %d", len(got))
	}
}

func TestMutableStateInObject_KotlinAtomicsAreThreadSafe(t *testing.T) {
	findings := runRuleByName(t, "MutableStateInObject", `
package test
import kotlin.concurrent.atomics.AtomicInt
import kotlin.concurrent.atomics.AtomicReference
object Registry {
    var counter = AtomicInt(0)
    var ref = AtomicReference<String?>(null)
    var plain = 0
}
`)
	if len(findings) != 1 {
		t.Fatalf("only the plain var should be reported, got %d: %v", len(findings), findings)
	}
}
