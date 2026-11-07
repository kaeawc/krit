package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

const lambdaAssignedLocalSource = `
package test
class Ctx { fun run() {} }
fun register(cb: Map<String, () -> Unit>) {}
fun f() {
    var last: Ctx? = null
    register(mapOf("a" to { last = Ctx() }))
    last!!.run()
    last!!.run()
}
`

func TestUnnecessaryNotNullOperator_LambdaAssignedLocalIsNotStable(t *testing.T) {
	findings := runRuleByNameWithOracleDiagnosticForNode(t, "UnnecessaryNotNullOperator", lambdaAssignedLocalSource,
		"postfix_expression", "last!!", "UNNECESSARY_NOT_NULL_ASSERTION")
	if len(findings) != 0 {
		t.Fatalf("a local var assigned in a lambda is not smart-cast stable, got %v", findings)
	}
}

func TestUnnecessaryNotNullOperator_StableLocalStillProjectsCompilerVerdict(t *testing.T) {
	findings := runRuleByNameWithOracleDiagnosticForNode(t, "UnnecessaryNotNullOperator", `
package test
class Ctx { fun run() {} }
fun make(): Ctx? = null
fun f() {
    var last: Ctx? = make()
    last!!.run()
    last!!.run()
}
`, "postfix_expression", "last!!", "UNNECESSARY_NOT_NULL_ASSERTION")
	if len(findings) != 1 {
		t.Fatalf("a local var no lambda assigns keeps the compiler verdict, got %v", findings)
	}
}

func TestUnnecessarySafeCall_LambdaAssignedLocalIsNotStable(t *testing.T) {
	findings := runRuleByNameWithOracleDiagnosticForNode(t, "UnnecessarySafeCall", `
package test
class Ctx { fun run() {} }
fun register(cb: Map<String, () -> Unit>) {}
fun f() {
    var last: Ctx? = null
    register(mapOf("a" to { last = Ctx() }))
    last!!.run()
    last?.run
}
`, "navigation_expression", "last?.run", "UNNECESSARY_SAFE_CALL")
	if len(findings) != 0 {
		t.Fatalf("a local var assigned in a lambda is not smart-cast stable, got %v", findings)
	}
}

func TestElseCaseInsteadOfExhaustiveWhen_GenericSealedObjectVariantNotCovered(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", `
package test
sealed interface Outcome<out T> {
    data class Loaded<T>(val value: T) : Outcome<T>
    data object Loading : Outcome<Nothing>

    fun valueOrNull(): T? =
        when (this) {
            is Loaded -> value
            else -> null
        }
}
`)
	if len(findings) != 0 {
		t.Fatalf("Loading is not covered, so else is needed; got %v", findings)
	}
}

func TestElseCaseInsteadOfExhaustiveWhen_GenericSealedAllVariantsCovered(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "ElseCaseInsteadOfExhaustiveWhen", `
package test
sealed interface Outcome<out T> {
    data class Loaded<T>(val value: T) : Outcome<T>
    data object Loading : Outcome<Nothing>
}

fun <T> read(o: Outcome<T>): T? =
    when (o) {
        is Outcome.Loaded -> o.value
        Outcome.Loading -> null
        else -> null
    }
`)
	if len(findings) != 1 {
		t.Fatalf("every variant is covered, so else is redundant; got %v", findings)
	}
}

func runElseWhenWithFakeSealed(t *testing.T, code string, variants []string) int {
	t.Helper()
	file := parseInline(t, code)
	resolver := typeinfer.NewResolver()
	resolver.IndexFilesParallel([]*scanner.File{file}, 1)
	fake := oracle.NewFakeOracle()
	fake.Sealed["Outcome"] = variants
	composite := oracle.NewCompositeResolver(fake, resolver)
	for _, r := range api.Registry {
		if r.ID == "ElseCaseInsteadOfExhaustiveWhen" {
			cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
			return len(cols.Findings())
		}
	}
	t.Fatal("rule not registered")
	return 0
}

func TestElseCaseInsteadOfExhaustiveWhen_OracleVariantsMustAllBeCovered(t *testing.T) {
	const decls = `
package test
sealed interface Outcome<out T> {
    data class Loaded<T>(val value: T) : Outcome<T>
    data object Loading : Outcome<Nothing>
}
`
	partial := decls + `
fun <T> read(o: Outcome<T>): T? =
    when (o) {
        is Outcome.Loaded -> o.value
        else -> null
    }
`
	if n := runElseWhenWithFakeSealed(t, partial, []string{"Loaded", "Loading"}); n != 0 {
		t.Fatalf("the oracle lists Loading, which is not covered; got %d findings", n)
	}
	full := decls + `
fun <T> read(o: Outcome<T>): T? =
    when (o) {
        is Outcome.Loaded -> o.value
        Outcome.Loading -> null
        else -> null
    }
`
	if n := runElseWhenWithFakeSealed(t, full, []string{"Loaded", "Loading"}); n != 1 {
		t.Fatalf("every oracle variant is covered; got %d findings", n)
	}
}

func TestIgnoredReturnValue_StdlibCollectionMutatorsAreNeverReported(t *testing.T) {
	flowType := &typeinfer.ResolvedType{Name: "MutableStateFlow", FQN: "kotlinx.coroutines.flow.MutableStateFlow", Kind: typeinfer.TypeClass}
	for _, call := range []string{"remove", "put", "add", "set", "removeAll", "retainAll", "offer", "poll", "pop"} {
		t.Run(call, func(t *testing.T) {
			findings := runIgnoredReturnValueWithOracleEvidence(t, `package test
fun f(flows: MutableMap<String, Any>, key: String) {
    flows.`+call+`(key)
}
`, oracleCallEvidence{CallText: "flows." + call + "(key)", CallTarget: "kotlin.collections.MutableMap." + call, ReturnType: flowType})
			if len(findings) != 0 {
				t.Fatalf("%s is a stdlib mutator and must not be reported, got %v", call, findings)
			}
		})
	}
}

func TestIgnoredReturnValue_MessageNamesTheBasis(t *testing.T) {
	seq := &typeinfer.ResolvedType{Name: "Sequence", FQN: "kotlin.sequences.Sequence", Kind: typeinfer.TypeClass}
	pure := runIgnoredReturnValueWithOracleEvidence(t, `package test
fun f(xs: List<Int>) {
    xs.asSequence().map { it + 1 }
}
`, oracleCallEvidence{CallText: "xs.asSequence().map { it + 1 }", CallTarget: "kotlin.sequences.map", ReturnType: seq})
	if len(pure) != 1 || !strings.Contains(pure[0].Message, "pure stdlib function") {
		t.Fatalf("expected the pure-stdlib basis, got %v", pure)
	}
	annotated := runIgnoredReturnValueWithOracleEvidence(t, `package test
fun f(box: Box) {
    box.parse()
}
`, oracleCallEvidence{CallText: "box.parse()", CallTarget: "demo.Box.parse", Annotations: []string{"androidx.annotation.CheckResult"}})
	if len(annotated) != 1 || !strings.Contains(annotated[0].Message, "annotated with @CheckResult (androidx.annotation.CheckResult)") {
		t.Fatalf("expected the annotation basis, got %v", annotated)
	}
}

func TestIgnoredReturnValue_ProcessorOfferReportsOnlyWithResolvedAnnotation(t *testing.T) {
	src := `package test
fun f(processor: PublishProcessor<Int>) {
    processor.offer(1)
}
`
	// RxJava 3.1.x PublishProcessor.offer and BehaviorProcessor.offer carry the library's own @CheckReturnValue.
	annotated := runIgnoredReturnValueWithOracleEvidence(t, src, oracleCallEvidence{
		CallText: "processor.offer(1)", CallTarget: "io.reactivex.rxjava3.processors.PublishProcessor.offer",
		Annotations: []string{"io.reactivex.rxjava3.annotations.CheckReturnValue"},
	})
	if len(annotated) != 1 {
		t.Fatalf("an annotated offer is reported, got %v", annotated)
	}
	for _, want := range []string{"annotated with @CheckReturnValue (io.reactivex.rxjava3.annotations.CheckReturnValue)", "onNext"} {
		if !strings.Contains(annotated[0].Message, want) {
			t.Fatalf("message should contain %q, got %q", want, annotated[0].Message)
		}
	}
	// A resolved offer without the annotation is never reported on its name.
	plain := runIgnoredReturnValueWithOracleEvidence(t, src, oracleCallEvidence{
		CallText: "processor.offer(1)", CallTarget: "demo.Queue.offer",
	})
	if len(plain) != 0 {
		t.Fatalf("an unannotated offer must not be reported by name, got %v", plain)
	}
}

func TestComputeDescribedViewNames(t *testing.T) {
	src := []byte(`
package demo
fun bind(binding: Binding) {
    binding.avatarView.contentDescription = label
    binding.closeButton?.setContentDescription(label)
    binding.header.logoView.setContentDescription(label)
    binding.exportButton?.let {
        it.visibility = VISIBLE
        it.contentDescription = label
    }
    binding.muteButton?.apply { contentDescription = text }
    binding.plainButton.setIcon(R.drawable.plain, R.color.white, label)
    binding.otherButton.setOnClickListener { }
}
`)
	names := rules.ComputeDescribedViewNames([][]byte{src})
	for _, want := range []string{"avatarView", "closeButton", "logoView", "exportButton"} {
		if !names[want] {
			t.Errorf("expected %q to be described, got %v", want, names)
		}
	}
	for _, not := range []string{"plainButton", "otherButton", "header", "binding"} {
		if names[not] {
			t.Errorf("%q must not be described, got %v", not, names)
		}
	}
}

func TestClickableViewAccessibilityResource_SkipsViewsDescribedInSource(t *testing.T) {
	r := findResourceRule(t, "ClickableViewAccessibilityResource")
	view := func(id string) *android.View {
		return &android.View{
			Type: "com.example.ui.IconView", ID: "@+id/" + id, Line: 5,
			Attributes: map[string]string{"android:clickable": "true"},
		}
	}
	run := func(id string, described map[string]bool) int {
		idx := indexWithLayout("main", "res/layout/main.xml", view(id))
		collector := scanner.NewFindingCollector(0)
		ctx := &api.Context{ResourceIndex: idx, Rule: r, Collector: collector, DescribedViewNames: described}
		r.Check(ctx)
		return len(api.ContextFindings(ctx))
	}
	if n := run("export_item_button", map[string]bool{"exportItemButton": true}); n != 0 {
		t.Fatalf("a view described in source must not be reported, got %d", n)
	}
	if n := run("export_item_button", map[string]bool{"otherButton": true}); n != 1 {
		t.Fatalf("an undescribed clickable icon view is still reported, got %d", n)
	}
}

func bigNestedBody(n int, indent string) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		b.WriteString(indent + "val v" + strings.Repeat("x", 1) + string(rune('a'+i%26)) + string(rune('a'+(i/26)%26)) + " = " + "1\n")
	}
	return b.String()
}

func TestLargeClass_NestedClassLinesAreNotCountedForTheOuterClass(t *testing.T) {
	code := "package test\ninterface Outer {\n    fun small()\n" +
		"    class Inner {\n" + bigNestedBody(700, "        ") + "    }\n" +
		"    companion object {\n" + bigNestedBody(50, "        ") + "    }\n" +
		"}\n"
	findings := runRuleByName(t, "LargeClass", code)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "Class Inner ") {
		t.Fatalf("only the large nested class should be reported, got %v", findings)
	}
}

func TestLargeClass_OwnLinesStillReported(t *testing.T) {
	code := "package test\nclass Outer {\n" + bigNestedBody(650, "    ") + "    class Inner {\n        val a = 1\n    }\n}\n"
	findings := runRuleByName(t, "LargeClass", code)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "Class Outer ") {
		t.Fatalf("the outer class own lines exceed the limit, got %v", findings)
	}
}

func TestCoroutineScopeCreatedButNeverCancelled_FakeExposingScopeThroughInterfaceIsSkipped(t *testing.T) {
	findings := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", `
package test
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
interface Wrapper { val scope: CoroutineScope }
class FakeWrapper : Wrapper {
    override val scope: CoroutineScope = CoroutineScope(Dispatchers.IO)
}
`)
	if len(findings) != 0 {
		t.Fatalf("a fake exposes the scope to its owner, got %v", findings)
	}
}

func TestCoroutineScopeCreatedButNeverCancelled_NonFakeOverrideAndPrivateProductionScopeStillReported(t *testing.T) {
	for name, code := range map[string]string{
		"production override": `
package test
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
interface Wrapper { val scope: CoroutineScope }
class RealWrapper : Wrapper {
    override val scope: CoroutineScope = CoroutineScope(Dispatchers.IO)
}`,
		"private production scope": `
package test
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
class RealWrapper {
    private val scope: CoroutineScope = CoroutineScope(Dispatchers.IO)
}`,
	} {
		t.Run(name, func(t *testing.T) {
			if f := runRuleByName(t, "CoroutineScopeCreatedButNeverCancelled", code); len(f) != 1 {
				t.Fatalf("expected one finding, got %v", f)
			}
		})
	}
}

func TestComposePainterResourceInLoop_SlotLambdaAndInlineLoopAreNotReported(t *testing.T) {
	findings := runRuleByName(t, "ComposePainterResourceInLoop", `
package test
import androidx.compose.foundation.layout.Column
import androidx.compose.material.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.painterResource

@Composable
fun Form(fields: List<String>) {
    Column {
        fields.forEachIndexed { index, text ->
            Field(
                value = text,
                leadingIcon = { Icon(painterResource(id = R.drawable.email), contentDescription = null) },
            )
        }
        fields.forEach { Icon(painterResource(id = R.drawable.email), contentDescription = null) }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("slot lambdas and plain inline loops must not be reported, got %v", findings)
	}
}

func TestComposePainterResourceInLoop_ForLoopBodyAndLazyItemsStillReported(t *testing.T) {
	findings := runRuleByName(t, "ComposePainterResourceInLoop", `
package test
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.Icon
import androidx.compose.runtime.Composable
import androidx.compose.ui.res.painterResource

@Composable
fun List(rows: List<String>) {
    LazyColumn {
        items(rows) { Icon(painterResource(id = R.drawable.marker), contentDescription = null) }
    }
    for (row in rows) {
        Icon(painterResource(id = R.drawable.marker), contentDescription = null)
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected the items lambda and the for loop, got %v", findings)
	}
}

func TestComposeRememberWithoutKey_KeyListIncludesFactoryParameter(t *testing.T) {
	findings := runRuleByName(t, "ComposeRememberWithoutKey", `
package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun Chart(rows: List<Row>, presenterFactory: PresenterFactory) {
    val presenter = remember { presenterFactory.create(rows) }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %v", findings)
	}
	msg := findings[0].Message
	if !strings.Contains(msg, "rows") || !strings.Contains(msg, "presenterFactory") {
		t.Fatalf("the key list must name every captured parameter, got %q", msg)
	}
}
