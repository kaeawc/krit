package rules_test

import (
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/android"
	"github.com/kaeawc/krit/internal/oracle"
	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/typeinfer"
)

func TestUnusedVariable_SoftKeywordCallArgumentsAreUses(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun render(isCustom: Boolean, position: Int) {
  val open: (String, Int) -> Unit = { name, at -> println(name + at) }
  show(
    onClick = if (isCustom) null else { item -> open(item, position) },
    onLongClick =
      if (isCustom) null
      else
        { item -> open(item, position) },
  )
}
`)
	if len(findings) != 0 {
		t.Fatalf("a call to a soft-keyword named function is not a declaration, got %+v", findings)
	}
}

func TestUnusedVariable_SoftKeywordCallStillReportsUnusedLocal(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun render(position: Int) {
  val open: (Int) -> Unit = { println(it) }
  val spare = position + 1
  show(onClick = { item -> println(item) })
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected the unused open and spare, got %d", len(findings))
	}
}

func TestUnusedVariable_MisLexedNegationAfterReturnCountsAsUse(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun shouldShip(order: Order, previous: Order?): Boolean {
  val isCancelled = order.isCancelled()
  val previousFailed = previous != null && previous.hasFailed
  return !isCancelled &&
    (!previousFailed || order.hasFailed) &&
    !order.isArchived
}
`)
	if len(findings) != 0 {
		t.Fatalf("!isCancelled reads the local, got %+v", findings)
	}
}

func TestUnusedVariable_MisLexedNegationStillReportsOtherLocal(t *testing.T) {
	findings := runRuleByName(t, "UnusedVariable", `
package test

fun shouldShip(order: Order): Boolean {
  val isCancelled = order.isCancelled()
  val isOther = order.isOther()
  return !isOther &&
    order.visible
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected only isCancelled, got %d", len(findings))
	}
}

func TestUnusedParameter_SoftKeywordCallArgumentIsUse(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

fun render(index: Int, open: (Int) -> Unit) {
  show { item -> open(index) }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %+v", findings)
	}
}

func TestUnusedParameter_UsedInNestedObjectLiteralArgument(t *testing.T) {
	findings := runRuleByName(t, "UnusedParameter", `
package test

internal class InvoiceRow(private val compact: Boolean) {
  fun render(
    view: RowView,
    invoice: Invoice,
    style: RowStyle,
    onTap: TapHandler?,
  ) =
    with(view) {
      clear()
      if (!invoice.visible) return@with

      if (compact || invoice.lines.isEmpty()) {
        title.text = invoice.title
        return@with
      }

      layout.attach(
        owner = this,
        title = invoice.title,
        dense = style.dense,
        expander =
          object : Expander {
            override fun onExpand(line: Line) {
              onTap?.tapped(line)
            }

            override fun onCollapse() = Unit
          },
      )
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("style and onTap are used inside the nested object literal, got %+v", findings)
	}
}

func TestRunTestWithDelay_SkipsWhenFileUsesTestDispatcherRule(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import org.junit.Rule
import org.junit.Test

class DebouncerTest {
    @get:Rule val mainRule = MainDispatcherRule()
    private val debouncer = Debouncer(mainRule.testDispatcher)

    @Test
    fun emitsAfterDebounce() = runTest {
        delay(3.seconds)
        assert(true)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("a delay advances the scheduler shared with the test dispatchers, got %d", len(findings))
	}
}

func TestRunTestWithDelay_SkipsWhenTestSharesSchedulerViaDispatcherArgument(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.test.StandardTestDispatcher
import kotlinx.coroutines.test.runTest
import org.junit.Test

class SchedulerTest {
    @Test
    fun usesScheduler() = runTest {
        val subject = Subject(StandardTestDispatcher(testScheduler))
        delay(1000)
        assert(true)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestRunTestWithDelay_SkipsWhenTestCallsOtherObjectsBetweenDelays(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import org.junit.Test

class CallsOtherCodeTest {
    private val repo = Repo()

    @Test
    fun readsAfterDelay() = runTest {
        repo.update("a")
        delay(100)
        assert(repo.value == "a")
    }

    @Test
    fun changesStateBetweenDelays() = runTest {
        var count = 0
        delay(100)
        count++
        delay(100)
        assert(count == 1)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("code under test can depend on the elapsed time, got %d", len(findings))
	}
}

func TestRunTestWithDelay_LoneDelayWithAssertionsOnlyKeepsAccurateMessage(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import org.junit.Test

class LoneDelayTest {
    @Test
    fun pointless() = runTest {
        delay(1000)
        assertEquals(1000, currentTime)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected the lone delay to be reported, got %d", len(findings))
	}
	if !strings.Contains(findings[0].Message, "calls no other code") {
		t.Fatalf("unexpected message %q", findings[0].Message)
	}
}

func TestIgnoredReturnValue_ProjectFlowReturningCallIsNotFlagged(t *testing.T) {
	flow := &typeinfer.ResolvedType{Name: "MutableStateFlow", FQN: "kotlinx.coroutines.flow.MutableStateFlow", Kind: typeinfer.TypeClass}
	code := `package test
class Registry
fun <T> MutableStateFlow<T>.registerIn(registry: Registry, key: String, overwrite: Boolean = false): MutableStateFlow<T> = this

fun f(flow: MutableStateFlow<Int>, registry: Registry) {
    flow.registerIn(registry, "key", overwrite = true)
}
`
	resolved := runIgnoredReturnValueWithOracleEvidence(t, code, oracleCallEvidence{
		CallText: "flow.registerIn", CallTarget: "test.registerIn", ReturnType: flow,
	})
	if len(resolved) != 0 {
		t.Fatalf("a project call that registers state eagerly is not a discarded result, got %d", len(resolved))
	}
	unresolved := runIgnoredReturnValueWithOracleEvidence(t, code, oracleCallEvidence{
		CallText: "flow.registerIn", ReturnType: flow,
	})
	if len(unresolved) != 0 {
		t.Fatalf("a Flow-typed call outside the pure allowlist is not flagged, got %d", len(unresolved))
	}
}

func TestIgnoredReturnValue_StdlibFlowOperatorIsStillFlagged(t *testing.T) {
	findings := runIgnoredReturnValueWithOracleEvidence(t, `package test
fun f(flow: Flow<Int>) {
    flow.onEach { println(it) }
}
`, oracleCallEvidence{
		CallText: "flow.onEach", CallTarget: "kotlinx.coroutines.flow.FlowKt.onEach",
		ReturnType: &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass},
	})
	if len(findings) != 1 {
		t.Fatalf("a discarded stdlib Flow operator is a bug, got %d", len(findings))
	}
}

func TestIgnoredReturnValue_ProjectCallWithCheckReturnValueIsStillFlagged(t *testing.T) {
	findings := runIgnoredReturnValueWithOracleEvidence(t, `package test
fun f(flow: Flow<Int>, registry: Registry) {
    flow.registerIn(registry)
}
`, oracleCallEvidence{
		CallText: "flow.registerIn", CallTarget: "test.registerIn",
		ReturnType:  &typeinfer.ResolvedType{Name: "Flow", FQN: "kotlinx.coroutines.flow.Flow", Kind: typeinfer.TypeClass},
		Annotations: []string{"com.google.errorprone.annotations.CheckReturnValue"},
	})
	if len(findings) != 1 {
		t.Fatalf("an annotated callee must be flagged, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_TrailingCastCountsAsAssertion(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class CastTest {
    @Test
    fun resultIsLoaded() {
        val result = load()
        result as Loaded
    }

    @Test
    fun expressionBody() = load() as Loaded
}
`)
	if len(findings) != 0 {
		t.Fatalf("a trailing cast fails on the wrong type, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_SafeCastIsNotAnAssertion(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class SafeCastTest {
    @Test
    fun resultIsLoadedSafely() {
        val result = load()
        result as? Loaded
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("a safe cast never throws, so the test has no check, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_CastInsideRunTestCountsAsAssertion(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class RunnerCastTest {
    @Test
    fun expressionBodyRunTest() = runTest {
        val result = load()
        result as Loaded
    }

    @Test
    fun blockBodyRunBlocking() {
        runBlocking {
            val result = load()
            result.inner as Inner
        }
    }

    @Test
    fun runTestWithoutCheck() = runTest {
        load()
    }

    @Test
    fun unknownWrapperCast() = withContext {
        load() as Loaded
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("only runner-wrapped trailing casts count, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_SmokeNamesAndComposeSmokeComment(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class SmokeTest {
    @Test
    fun rendersSmoke() {
        render()
    }

    @Test
    fun doesNotCrash() {
        render()
    }

    @Test
    fun doesNotThrowOnEmptyInput() {
        render()
    }

    @Test
    fun rendersEmptyState() {
        composeRule.setContent { EmptyState() }
        // Passes when composition completes without crashing.
        composeRule.waitForIdle()
    }

    @Test
    fun survivesUnboundedWidth() {
        composeRule.setContent {
            Row(modifier = Modifier.horizontalScroll(rememberScrollState())) { Table() }
        }
        // Reaching here means measure and layout completed without the width error.
        composeRule.waitForIdle()
    }

    @Test
    fun rendersLoadingState() {
        composeRule.setContent { LoadingState() }
        composeRule.waitForIdle() // no exception
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("intentional smoke tests are not missing assertions, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_StillReportsPlainTestWithoutCheck(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class PlainTest {
    @Test
    fun rendersEmptyState() {
        composeRule.setContent { EmptyState() }
        composeRule.waitForIdle()
    }

    @Test
    fun loads() {
        val result = load()
        println(result)
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected both tests reported, got %d", len(findings))
	}
}

func TestAssertNullableWithNotNullAssertion_SkipsOperandProvenNonNull(t *testing.T) {
	file := parseInline(t, `
package test

import com.google.common.truth.Truth.assertThat

fun testItems(viewModels: MutableList<Item>) {
    assertThat(viewModels[1]!!.isFirst).isFalse()
}
`)
	fake := oracle.NewFakeOracle()
	file.FlatWalkNodes(0, "postfix_expression", func(idx uint32) {
		if strings.TrimSpace(file.FlatNodeText(idx)) == "viewModels[1]!!" {
			fake.Diagnostics[file.Path] = []oracle.Diagnostic{{
				FactoryName: "UNNECESSARY_NOT_NULL_ASSERTION",
				Severity:    "WARNING",
				Line:        file.FlatRow(idx) + 1,
				Col:         file.FlatCol(idx) + 1,
				StartByte:   int(file.FlatStartByte(idx)),
				EndByte:     int(file.FlatEndByte(idx)),
			}}
		}
	})
	if len(fake.Diagnostics[file.Path]) == 0 {
		t.Fatal("postfix node not found")
	}
	composite := oracle.NewCompositeResolver(fake, typeinfer.NewResolver())
	for _, r := range api.Registry {
		if r.ID == "AssertNullableWithNotNullAssertion" {
			cols := rules.NewDispatcher([]*api.Rule{r}, composite).Run(file)
			findings := cols.Findings()
			if len(findings) != 0 {
				t.Fatalf("a !! the compiler marks unnecessary is not a nullable assertion, got %d", len(findings))
			}
			return
		}
	}
	t.Fatal("rule not found")
}

func TestAssertNullableWithNotNullAssertion_SkipsBangInNestedCallArguments(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

import com.google.common.truth.Truth.assertThat

fun testItems(results: List<Item>, message: Message) {
    assertThat(results[0])
        .isEqualTo(Saved(ts = message.ts!!, id = Id(message.itemId)))
    assertEquals(3, countNodes(item.label!!).size)
}
`)
	if len(findings) != 0 {
		t.Fatalf("a !! inside a nested call's arguments is not the subject, got %d", len(findings))
	}
}

func TestAssertNullableWithNotNullAssertion_StillFlagsSubjectBang(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

import com.google.common.truth.Truth.assertThat

fun testItems(maybe: Item?, other: String?) {
    assertThat(maybe!!.name).isEqualTo("x")
    assertEquals("x", other!!)
}
`)
	if len(findings) != 2 {
		t.Fatalf("a !! on the assertion subject must stay flagged, got %d", len(findings))
	}
	for _, f := range findings {
		if f.Line == 8 {
			want := "Avoid `!!` in assertions; use `assertThat(other).isNotNull()` first, then use a local val or smart-cast."
			if f.Message != want {
				t.Errorf("subject message = %q, want %q", f.Message, want)
			}
		}
	}
}

func TestAssertNullableWithNotNullAssertion_TruthSubjectMessageNamesOperand(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

import com.google.common.truth.Truth.assertThat

fun testItems(maybe: Item?) {
    assertThat(maybe!!).isEqualTo(Item())
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(findings))
	}
	want := "Avoid `!!` in assertions; use `assertThat(maybe).isNotNull()` first, then use a local val or smart-cast."
	if findings[0].Message != want {
		t.Errorf("message = %q, want %q", findings[0].Message, want)
	}
}

func TestAssertNullableWithNotNullAssertion_MessageForMemberReceiverBang(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

fun testItems(maybe: Item?, other: String?) {
    assertEquals("x", maybe!!.name)
    assertEquals("x", other!!)
}
`)
	if len(findings) != 2 {
		t.Fatalf("both bangs stay flagged, got %d: %v", len(findings), findings)
	}
	for _, f := range findings {
		switch f.Line {
		case 5:
			if want := "Avoid `!!` in assertion arguments; use `assertNotNull(maybe)` first and smart-cast."; f.Message != want {
				t.Fatalf("member receiver message = %q, want %q", f.Message, want)
			}
		case 6:
			if want := "Avoid `!!` in assertions; use `assertNotNull(other)` first and smart-cast."; f.Message != want {
				t.Fatalf("subject message = %q, want %q", f.Message, want)
			}
		}
	}
}

func TestComposeDerivedStateMisuse_NegativeRememberKeyedOnStateHolder(t *testing.T) {
	findings := runRuleByName(t, "ComposeDerivedStateMisuse", `
package test
import androidx.compose.runtime.*

@Composable
fun Overlay(swiped: State<Float>) {
    val visible by remember(swiped) { derivedStateOf { swiped.value != 0f } }
}
`)
	if len(findings) != 0 {
		t.Fatalf("a key on the State holder never changes, so derivedStateOf is needed, got %d", len(findings))
	}
}

func TestComposeDerivedStateMisuse_PositiveRememberKeyedOnStateValue(t *testing.T) {
	findings := runRuleByName(t, "ComposeDerivedStateMisuse", `
package test
import androidx.compose.runtime.*

@Composable
fun Overlay(swiped: State<Float>) {
    val visible by remember(swiped.value) { derivedStateOf { swiped.value != 0f } }
}
`)
	if len(findings) != 1 {
		t.Fatalf("a key on the read value makes derivedStateOf redundant, got %d", len(findings))
	}
}

func TestPrintlnInProduction_SkipsPrintNamedHelperAndSystemOutKDoc(t *testing.T) {
	code := `
package test

/** Records reads and prints to [System.out] whenever state changes. */
fun observe(id: String) {
    printChange(id)
}

private fun printChange(id: String) {
    println("$id changed")
}

/** Writes the line to System.err for the developer. */
fun report(line: String) {
    System.err.println(line)
}

class Tracer {
    fun print(line: String) {
        println(line)
    }
}
`
	if got := runRuleByName(t, "PrintlnInProduction", code); len(got) != 0 {
		t.Fatalf("print-named helpers and System.out KDoc are intentional console output, got %d", len(got))
	}
}

func TestPrintlnInProduction_StillFlagsPrintLookalikeNames(t *testing.T) {
	got := runRuleByName(t, "PrintlnInProduction", `
package test

fun printer(line: String) {
    println(line)
}

fun reprint(line: String) {
    println(line)
}
`)
	if len(got) != 2 {
		t.Fatalf("names that only contain print stay flagged, got %d", len(got))
	}
}

// cyclomaticRankSource has 10 when entries plus an else, 8 `&&` and 10 `||`.
const cyclomaticRankSource = `package test

fun rank(c: Channel): Int {
    return when {
        // Entry.
        (c.a || c.b) && c.m() -> RANK_1
        // Entry.
        (c.a || c.b) && c.n() -> RANK_2
        // Entry.
        (c.a || c.b) && c.o() -> RANK_3
        // Entry.
        (c.a || c.b) && c.p() -> RANK_4
        // Entry.
        (c.a || c.b) && c.q() -> RANK_5
        // Entry.
        (c.a || c.b) && c.r() -> RANK_6
        // Entry.
        (c.a || c.b) && c.s() -> RANK_7
        // Entry.
        (c.a || c.b) && c.t() -> RANK_8
        // Entry.
        c.a || c.b -> RANK_9
        // Entry.
        c.a || c.c -> RANK_10
        else -> {
            log("unexpected")
            0
        }
    }
}
`

func cyclomaticRule(t *testing.T) *rules.CyclomaticComplexMethodRule {
	t.Helper()
	for _, candidate := range api.Registry {
		if candidate.ID == "CyclomaticComplexMethod" {
			rule, ok := candidate.Implementation.(*rules.CyclomaticComplexMethodRule)
			if !ok {
				t.Fatalf("expected CyclomaticComplexMethodRule, got %T", candidate.Implementation)
			}
			return rule
		}
	}
	t.Fatal("CyclomaticComplexMethod rule not registered")
	return nil
}

func TestCyclomaticComplexMethod_CountsWhenEntriesAsDetektDoes(t *testing.T) {
	rule := cyclomaticRule(t)
	original := *rule
	defer func() { *rule = original }()
	rule.AllowedComplexity = 0
	rule.IgnoreSimpleWhenEntries = false

	// 1 + 11 when entries (the else entry counts) + 8 `&&` + 10 `||`, which is what detekt 1.23.8 reports.
	findings := runRuleByName(t, "CyclomaticComplexMethod", cyclomaticRankSource)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "(complexity: 30)") {
		t.Fatalf("expected one finding with complexity 30, got %+v", findings)
	}
}

func TestCyclomaticComplexMethod_IgnoreSimpleWhenEntriesCountsLikeDetekt(t *testing.T) {
	rule := cyclomaticRule(t)
	original := *rule
	defer func() { *rule = original }()
	rule.AllowedComplexity = 0
	rule.IgnoreSimpleWhenEntries = true

	// 1 + 1 for the braced `else` entry + 8 `&&` + 10 `||`.
	findings := runRuleByName(t, "CyclomaticComplexMethod", cyclomaticRankSource)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "(complexity: 20)") {
		t.Fatalf("expected one finding with complexity 20, got %+v", findings)
	}

	// Braced entries are counted: three more braced entries add three.
	braced := strings.Replace(cyclomaticRankSource, "-> RANK_1\n", "-> { RANK_1 }\n", 1)
	braced = strings.Replace(braced, "-> RANK_2\n", "-> { RANK_2 }\n", 1)
	braced = strings.Replace(braced, "-> RANK_3\n", "-> { RANK_3 }\n", 1)
	findings = runRuleByName(t, "CyclomaticComplexMethod", braced)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "(complexity: 23)") {
		t.Fatalf("expected one finding with complexity 23, got %+v", findings)
	}
}

func TestCyclomaticComplexMethod_CountsBranchesLoopsCatchesAndElvis(t *testing.T) {
	rule := cyclomaticRule(t)
	original := *rule
	defer func() { *rule = original }()
	rule.AllowedComplexity = 0

	// 1 + if + else if + for + while + two catches + two elvis; the safe call adds nothing.
	findings := runRuleByName(t, "CyclomaticComplexMethod", `package test

fun mixed(a: Int?, xs: List<Int>): Int {
    if (a == null) {
        log("none")
    } else if (a > 1) {
        log("big")
    }
    for (x in xs) {
        log(x)
    }
    while (xs.isEmpty()) {
        log("empty")
    }
    try {
        log(a?.toString() ?: "")
    } catch (e: IllegalStateException) {
        log("state")
    } catch (e: IllegalArgumentException) {
        log("argument")
    }
    return a ?: 0
}
`)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "(complexity: 9)") {
		t.Fatalf("expected one finding with complexity 9, got %+v", findings)
	}
}

func TestStringFormatCountResource_IgnoresTemplatePlaceholders(t *testing.T) {
	r := findResourceRule(t, "StringFormatCountResource")
	idx := emptyIndex()
	idx.Strings["quota"] = "Il reste ^1 sur ^2 et ^3"
	idx.StringsLocation["quota"] = android.StringLocation{FilePath: "res/values-fr/strings.xml", Line: 4}
	idx.StringSources = map[string]android.StringSource{
		"quota": {Value: "^1 of ^2 left", FilePath: "res/values/strings.xml", Line: 3},
	}
	if f := runResourceRule(r, idx); len(f) != 0 {
		t.Fatalf("^N tokens are not format arguments, got %v", f)
	}

	idx.Strings["quota"] = "^1"
	idx.StringSources["quota"] = android.StringSource{Value: "%% done: ^1 of ^2", FilePath: "res/values/strings.xml", Line: 3}
	if f := runResourceRule(r, idx); len(f) != 0 {
		t.Fatalf("an escaped percent and ^N tokens are not format arguments, got %v", f)
	}
}

func TestStringFormatCountResource_StillReportsPercentSpecifierMismatch(t *testing.T) {
	r := findResourceRule(t, "StringFormatCountResource")
	idx := emptyIndex()
	idx.Strings["quota"] = "Il reste ^1 sur %1$s"
	idx.StringsLocation["quota"] = android.StringLocation{FilePath: "res/values-fr/strings.xml", Line: 4}
	idx.StringSources = map[string]android.StringSource{
		"quota": {Value: "%1$s of %2$s left ^1", FilePath: "res/values/strings.xml", Line: 3},
	}
	f := runResourceRule(r, idx)
	if len(f) != 1 || !strings.Contains(f[0].Message, "found both 1 here and 2 in values/strings.xml") {
		t.Fatalf("expected the 1 vs 2 mismatch, got %v", f)
	}
}

func TestStringFormatCountResource_TokenDefaultWithSpecifierTranslation(t *testing.T) {
	r := findResourceRule(t, "StringFormatCountResource")
	idx := emptyIndex()
	idx.Strings["invite"] = "Invitez %1$s"
	idx.StringsLocation["invite"] = android.StringLocation{FilePath: "res/values-fr/strings.xml", Line: 4}
	idx.StringSources = map[string]android.StringSource{
		"invite": {Value: "Invite ^1", FilePath: "res/values/strings.xml", Line: 3},
	}
	if f := runResourceRule(r, idx); len(f) != 0 {
		t.Fatalf("^1 in the default and %%1$s in the translation is a pipeline convention, got %v", f)
	}

	idx.Strings["invite"] = "Invitez %1$s"
	idx.StringSources["invite"] = android.StringSource{Value: "Invite ^1 to ^2", FilePath: "res/values/strings.xml", Line: 3}
	f := runResourceRule(r, idx)
	if len(f) != 1 || !strings.Contains(f[0].Message, "found both 1 here and 2 in values/strings.xml") || !strings.Contains(f[0].Message, "`^N` tokens") {
		t.Fatalf("expected a token-count mismatch, got %v", f)
	}
}

func TestHardcodedDateFormat_DisplayNamePatternsFollowTheUserLocale(t *testing.T) {
	for _, pattern := range []string{"MMM d, yyyy", "EEEE", "EEE", "MMMM", "EEE, MMM d", "hh:mm a", "MMMM yyyy"} {
		t.Run(pattern, func(t *testing.T) {
			findings := runRuleByName(t, "HardcodedDateFormat", `
package test
import java.time.format.DateTimeFormatter
val dayLabelFormatter = DateTimeFormatter.ofPattern("`+pattern+`")
`)
			if len(findings) != 0 {
				t.Fatalf("a display pattern %q should use the default locale, got %d", pattern, len(findings))
			}
		})
	}
}

func TestHardcodedDateFormat_FlagsNamePatternsThatAreStoredOrExchanged(t *testing.T) {
	cases := map[string]string{
		"file name":  `val backupFileName = DateTimeFormatter.ofPattern("MMM-d-yyyy")`,
		"cache key":  `fun cacheKeyFormat() = DateTimeFormatter.ofPattern("EEE-HH")`,
		"timestamp":  `val formatter = DateTimeFormatter.ofPattern("EEE, d MMM yyyy HH:mm:ss 'GMT' z")`,
		"week based": `val formatter = DateTimeFormatter.ofPattern("YYYY-'W'ww")`,
	}
	for name, decl := range cases {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByName(t, "HardcodedDateFormat", `
package test
import java.time.format.DateTimeFormatter
`+decl+"\n")
			if len(findings) != 1 {
				t.Fatalf("expected the machine-consumed %s to be flagged, got %d", name, len(findings))
			}
		})
	}
}

func TestTestWithoutAssertion_TrailingThrowingCheckCountsAsAssertion(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class ThrowingCheckTest {
    @Test
    fun notNullBang() {
        val result = load()
        result!!
    }

    @Test
    fun checkNotNullCall() {
        checkNotNull(load())
    }

    @Test
    fun requireNotNullCall() {
        requireNotNull(load()) { "missing" }
    }

    @Test
    fun assertIsCall() {
        assertIs<Loaded>(load())
    }

    @Test
    fun assertIsNotCall() {
        assertIsNot<Failed>(load())
    }

    @Test
    fun checkCall() {
        check(load() != null)
    }

    @Test
    fun requireCall() {
        require(load() != null)
    }

    @Test
    fun castBoundToValue() {
        val loaded = load() as Loaded
    }

    @Test
    fun expressionBodyBang() = load()!!
}
`)
	if len(findings) != 0 {
		t.Fatalf("a trailing throwing check is an assertion, got %d: %v", len(findings), findings)
	}
}

func TestTestWithoutAssertion_MockCalledWithoutVerifyIsReported(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test
import org.mockito.Mockito.mock
import org.mockito.Mockito.spy

class MockOnlyTest {
    @Test
    fun callsMock() {
        val service = mock(Service::class.java)
        service.refresh()
    }

    @Test
    fun callsSpy() {
        val service = spy(RealService())
        service.refresh()
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("a test that only calls a mock or spy has no check, got %d: %v", len(findings), findings)
	}
}

func TestAssertNullableWithNotNullAssertion_MessageNamesEveryBangInChain(t *testing.T) {
	cases := []struct {
		name, src, want string
	}{
		{"truth", `
package test

import com.google.common.truth.Truth.assertThat

fun testItems(receipt: Receipt?) {
    assertThat(receipt!!.payload!!.items).isEmpty()
}
`, "Avoid `!!` in assertion arguments; use `assertThat(receipt).isNotNull()` and `assertThat(receipt.payload).isNotNull()` first, then use a local val or smart-cast."},
		{"junit", `
package test

fun testPlain(receipt: Receipt?) {
    assertEquals(1, receipt!!.payload!!.items)
}
`, "Avoid `!!` in assertion arguments; use `assertNotNull(receipt)` and `assertNotNull(receipt.payload)` first and smart-cast."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", tc.src)
			if len(findings) != 1 {
				t.Fatalf("expected one finding per assertion, got %d: %v", len(findings), findings)
			}
			if findings[0].Message != tc.want {
				t.Errorf("message = %q, want %q", findings[0].Message, tc.want)
			}
		})
	}
}
