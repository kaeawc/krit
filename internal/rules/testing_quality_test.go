package rules_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kaeawc/krit/internal/rules"
	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
)

func TestAssertEqualsArgumentOrder_Positive(t *testing.T) {
	findings := runRuleByName(t, "AssertEqualsArgumentOrder", `
package test

import org.junit.Assert.assertEquals

fun testCompute() {
    val actual = compute()
    val expected = 42
    assertEquals(actual, expected)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for assertEquals(actual, expected)")
	}
}

func TestAssertEqualsArgumentOrder_Negative(t *testing.T) {
	findings := runRuleByName(t, "AssertEqualsArgumentOrder", `
package test

import org.junit.Assert.assertEquals

fun testCompute() {
    val actual = compute()
    val expected = 42
    assertEquals(expected, actual)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestAssertEqualsArgumentOrder_MessageAndDelta(t *testing.T) {
	findings := runRuleByName(t, "AssertEqualsArgumentOrder", `
package test

import org.junit.Assert.assertEquals

fun testCompute() {
    val actual = compute()
    val expected = 42
    assertEquals("message", actual, expected)
    assertEquals(actual, expected, "message")
    assertEquals(actual, expected, 0.1)
    assertEquals("message", expected, actual)
    assertEquals(expected, actual, "message")
}
`)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings, got %d", len(findings))
	}
}

func TestAssertTrueOnComparison_Positive(t *testing.T) {
	findings := runRuleByName(t, "AssertTrueOnComparison", `
package test

import org.junit.Assert.assertTrue

fun testCompute() {
    val actual = compute()
    val expected = 42
    assertTrue(actual == expected)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for assertTrue(actual == expected)")
	}
}

func TestAssertTrueOnComparison_Negative(t *testing.T) {
	findings := runRuleByName(t, "AssertTrueOnComparison", `
package test

import org.junit.Assert.assertEquals
import org.junit.Assert.assertTrue

fun testCompute() {
    val actual = compute()
    val expected = 42
    assertTrue(actual > 0)
    assertEquals(expected, actual)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestAssertTrueOnComparison_NegativeBooleanLiteralOperand(t *testing.T) {
	findings := runRuleByName(t, "AssertTrueOnComparison", `
package test

import org.junit.Assert.assertFalse
import org.junit.Assert.assertTrue

fun testNullSafe(x: String?, flag: Boolean?) {
    assertTrue(x?.contains("a") == true)
    assertTrue(flag == true)
    assertFalse(flag == false)
    assertTrue(false == flag)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestAssertTrueOnComparison_PositiveSizeComparison(t *testing.T) {
	findings := runRuleByName(t, "AssertTrueOnComparison", `
package test

import org.junit.Assert.assertTrue

fun testSize(list: List<Int>, a: Int, b: Int) {
    assertTrue(list.size == 1)
    assertTrue(a == b)
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}

func TestAssertTrueOnComparison_PositiveJUnitReceiver(t *testing.T) {
	findings := runRuleByName(t, "AssertTrueOnComparison", `
package test

import org.junit.Assert

fun testCompute() {
    val actual = compute()
    val expected = 42
    Assert.assertTrue(actual == expected)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for Assert.assertTrue(actual == expected)")
	}
}

func TestAssertTrueOnComparison_PositiveJUnitTestCaseReceiver(t *testing.T) {
	findings := runRuleByName(t, "AssertTrueOnComparison", `
package test

import junit.framework.TestCase

fun testCompute() {
    val actual = compute()
    val expected = 42
    TestCase.assertTrue(actual == expected)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for TestCase.assertTrue(actual == expected)")
	}
}

func TestAssertTrueOnComparison_NegativeCustomReceiver(t *testing.T) {
	findings := runRuleByName(t, "AssertTrueOnComparison", `
package test

object CustomAssert {
    fun assertTrue(value: Boolean) = check(value)
}

fun testCompute() {
    val actual = compute()
    val expected = 42
    CustomAssert.assertTrue(actual == expected)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestMixedAssertionLibraries_Positive(t *testing.T) {
	findings := runRuleByName(t, "MixedAssertionLibraries", `
package test

import org.junit.Assert.assertEquals
import com.google.common.truth.Truth.assertThat

fun testCompute() {
    val actual = compute()
    assertEquals(42, actual)
    assertThat(actual).isEqualTo(42)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for mixed JUnit Assert and Truth imports")
	}
}

func TestMixedAssertionLibraries_Negative(t *testing.T) {
	findings := runRuleByName(t, "MixedAssertionLibraries", `
package test

import com.google.common.truth.Truth.assertThat

fun testCompute() {
    val actual = compute()
    assertThat(actual).isEqualTo(42)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

// Single emit per file even when many imports of both libraries are
// present — source_file dispatches once, regardless of import count.
func TestMixedAssertionLibraries_EmitsOnce(t *testing.T) {
	findings := runRuleByName(t, "MixedAssertionLibraries", `
package test

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull
import com.google.common.truth.Truth.assertThat
import com.google.common.truth.Truth.assertWithMessage

fun testCompute() {
    val actual = compute()
    assertEquals(42, actual)
    assertNotNull(actual)
    assertThat(actual).isEqualTo(42)
    assertWithMessage("x").that(actual).isEqualTo(42)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected exactly one finding, got %d", len(findings))
	}
}

// Wildcard imports for both libraries trigger the rule via ImportFacts'
// Wildcards prefix match.
func TestMixedAssertionLibraries_WildcardImports(t *testing.T) {
	findings := runRuleByName(t, "MixedAssertionLibraries", `
package test

import org.junit.Assert.*
import com.google.common.truth.Truth.*

fun testCompute() {
    val actual = compute()
    assertEquals(42, actual)
    assertThat(actual).isEqualTo(42)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for wildcard mixed imports, got %d", len(findings))
	}
}

// Aliased imports still match — ImportFacts indexes the alias-stripped FQN.
func TestMixedAssertionLibraries_AliasedImports(t *testing.T) {
	findings := runRuleByName(t, "MixedAssertionLibraries", `
package test

import org.junit.Assert.assertEquals as junitEquals
import com.google.common.truth.Truth.assertThat as truthAssertThat

fun testCompute() {
    val actual = compute()
    junitEquals(42, actual)
    truthAssertThat(actual).isEqualTo(42)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for aliased mixed imports, got %d", len(findings))
	}
}

// Comments and KDoc that mention the FQNs but never import them must not
// trigger the rule — ImportFacts only inspects import_header AST nodes.
func TestMixedAssertionLibraries_CommentMentionsOnly(t *testing.T) {
	findings := runRuleByName(t, "MixedAssertionLibraries", `
package test

// Not actually importing: org.junit.Assert.assertEquals
/* Block comment referencing com.google.common.truth.Truth.assertThat */
/**
 * KDoc that mentions org.junit.Assert.assertEquals and
 * com.google.common.truth.Truth.assertThat.
 */
import com.google.common.truth.Truth.assertThat

fun testCompute() {
    val actual = compute()
    assertThat(actual).isEqualTo(42)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for comment-only mentions, got %d", len(findings))
	}
}

// Only one library imported is the common case and must stay clean.
func TestMixedAssertionLibraries_OnlyJUnit(t *testing.T) {
	findings := runRuleByName(t, "MixedAssertionLibraries", `
package test

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull

fun testCompute() {
    val actual = compute()
    assertEquals(42, actual)
    assertNotNull(actual)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestAssertNullableWithNotNullAssertion_Positive(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

import org.junit.Assert.assertEquals

fun testNullableAssertion(maybeX: String?) {
    assertEquals("x", maybeX!!)
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for assertEquals with !! argument")
	}
}

func TestAssertNullableWithNotNullAssertion_Negative(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull

fun testNullableAssertion(maybeX: String?) {
    assertNotNull(maybeX)
    assertEquals("x", maybeX)
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestAssertNullableWithNotNullAssertion_NegativeProductionCheck(t *testing.T) {
	file := parseInline(t, `
package prod

fun runPayment(inAppPayment: Payment?) {
    requireNotNull(inAppPayment)
    check(inAppPayment!!.type == "gift")
}

class Payment(val type: String)
`)
	file.Path = "/repo/app/src/main/kotlin/prod/PaymentJob.kt"
	findings := runRuleByNameOnFile(t, "AssertNullableWithNotNullAssertion", file)
	if len(findings) != 0 {
		t.Fatalf("expected production check() call to be ignored, got %d", len(findings))
	}
}

func TestAssertNullableWithNotNullAssertion_NegativeVerifyNamedHelper(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

fun testHelper(result: Result?) {
    verifyRendered(result?.items!!)
}

private fun verifyRendered(items: List<String>) {}

class Result(val items: List<String>?)
`)
	if len(findings) != 0 {
		t.Fatalf("expected verify-named helper to be ignored, got %d", len(findings))
	}
}

func TestAssertNullableWithNotNullAssertion_NegativeLambdaArgumentBody(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

fun testHelper(result: Result?) {
    assertAll {
        result!!.items
    }
}

private fun assertAll(block: () -> Unit) {}

class Result(val items: List<String>)
`)
	if len(findings) != 0 {
		t.Fatalf("expected nested lambda body to be ignored, got %d", len(findings))
	}
}

func TestAssertNullableWithNotNullAssertion_NegativeExpectedPrefixHelper(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

fun testHelper(result: Result?) {
    expectedState(result!!.items)
}

private fun expectedState(items: List<String>) {}

class Result(val items: List<String>)
`)
	if len(findings) != 0 {
		t.Fatalf("expected expected-prefix helper to be ignored, got %d", len(findings))
	}
}

func TestAssertNullableWithNotNullAssertion_NegativeProjectAssertHelper(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

fun testHelper(result: Result?) {
    assertState(result!!.items)
}

private fun assertState(items: List<String>) {}

class Result(val items: List<String>)
`)
	if len(findings) != 0 {
		t.Fatalf("expected project assert-prefixed helper to be ignored, got %d", len(findings))
	}
}

func TestAssertNullableWithNotNullAssertion_UsesLocalASTOnly(t *testing.T) {
	rule := buildRuleIndex()["AssertNullableWithNotNullAssertion"]
	if rule == nil {
		t.Fatal("AssertNullableWithNotNullAssertion rule is not registered")
	}
	if rule.Needs != api.NeedsResolver|api.NeedsOracleDiagnostics|api.NeedsOracleCallTargets {
		t.Fatalf("AssertNullableWithNotNullAssertion should only need oracle diagnostics and call targets, got needs %v", rule.Needs)
	}
	if rule.Oracle != nil {
		t.Fatal("AssertNullableWithNotNullAssertion should not declare oracle metadata")
	}
}

func TestMockWithoutVerify_Positive(t *testing.T) {
	findings := runRuleByName(t, "MockWithoutVerify", `
package test

import io.mockk.mockk
import org.junit.Test

class MockWithoutVerifyPositive {
    @Test
    fun load() {
        val api = mockk<Api>()
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for mock without verify")
	}
}

func TestMockWithoutVerify_PositiveUnusedMockWithUnrelatedStubbing(t *testing.T) {
	findings := runRuleByName(t, "MockWithoutVerify", `
package test

import io.mockk.every
import io.mockk.mockk
import org.junit.Test

class MockWithoutVerifyUnusedWithUnrelatedStubbingPositive {
    @Test
    fun load() {
        val api = mockk<Api>()
        val service = mockk<Service>()
        every { service.load() } returns "data"
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for the unused mock only, got %d", len(findings))
	}
	if findings[0].Line != 11 {
		t.Fatalf("expected finding on the unused mock declaration, got line %d", findings[0].Line)
	}
}

func TestMockWithoutVerify_Negative(t *testing.T) {
	findings := runRuleByName(t, "MockWithoutVerify", `
package test

import io.mockk.every
import io.mockk.mockk
import io.mockk.verify
import org.junit.Test

class MockWithoutVerifyNegative {
    @Test
    fun load() {
        val api = mockk<Api>()
        every { api.get() } returns "data"
        val repo = Repo(api)
        repo.load()
        verify { api.get() }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestMockWithoutVerify_NegativeMockKInitializerStubbing(t *testing.T) {
	findings := runRuleByName(t, "MockWithoutVerify", `
package test

import io.mockk.every
import io.mockk.mockk
import org.junit.Test

class MockWithoutVerifyInitializerStubbingNegative {
    @Test
    fun mapsData() {
        val envelope = mockk<Envelope> {
            every { eventTemplateID } returns "template123"
            every { eventTypeID } returns "type789"
        }

        val result = envelope.convert()

        assertEquals("template123", result.templateId)
        assertEquals("type789", result.typeId)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for mockk initializer stubbing, got %d", len(findings))
	}
}

func TestMockWithoutVerify_NegativeMockKReturnsMockValue(t *testing.T) {
	findings := runRuleByName(t, "MockWithoutVerify", `
package test

import io.mockk.every
import io.mockk.mockk
import org.junit.Test

class MockWithoutVerifyReturnsMockValueNegative {
    private val builder = mockk<Builder>()

    @Test
    fun mapsData() {
        val domain = mockk<Domain>(relaxed = true)
        every { builder.convert(any()) } returns domain

        val result = Subject(builder).load()

        assertSame(domain, result)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for mockk return-value stubbing, got %d", len(findings))
	}
}

func TestMockWithoutVerify_NegativeMockitoWheneverStubbing(t *testing.T) {
	findings := runRuleByName(t, "MockWithoutVerify", `
package test

import org.junit.Test
import org.mockito.kotlin.mock
import org.mockito.kotlin.whenever

class MockWithoutVerifyMockitoWheneverNegative {
    @Test
    fun load() {
        val api = mock<Api>()
        whenever(api.get()).thenReturn("data")
        val repo = Repo(api)

        repo.load()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for whenever() stubbing, got %d", len(findings))
	}
}

func TestMockWithoutVerify_NegativeConstructorInjection(t *testing.T) {
	findings := runRuleByName(t, "MockWithoutVerify", `
package test

import org.junit.Test
import org.mockito.kotlin.mock

class MockWithoutVerifyConstructorInjectionNegative {
    @Test
    fun load() {
        val api = mock<Api>()
        val repo = Repo(api)

        repo.load()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for mock passed to object under test, got %d", len(findings))
	}
}

func TestMockWithoutVerify_NegativeMockitoWhenStubbing(t *testing.T) {
	findings := runRuleByName(t, "MockWithoutVerify", `
package test

import org.junit.Test
import org.mockito.Mockito
import org.mockito.kotlin.mock

class MockWithoutVerifyMockitoWhenNegative {
    @Test
    fun load() {
        val api = mock<Api>()
        Mockito.`+"`when`"+`(api.get()).thenReturn("data")

        Repo(api).load()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for Mockito.when() stubbing, got %d", len(findings))
	}
}

func TestRunTestWithDelay_Positive(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithDelayPositive {
    @Test
    fun works() = runTest {
        delay(1000)
        assert(true)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for delay inside runTest")
	}
}

func TestRunTestWithDelay_NegativeDelayDrivesOtherWork(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithDelayDrivesWork {
    @Test
    fun launched() = runTest {
        launch { emit(1) }
        delay(100)
        assert(true)
    }

    @Test
    fun scheduled() = runTest {
        start()
        delay(100)
        advanceUntilIdle()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when delay lets other work run, got %d: %v", len(findings), findings)
	}
}

func TestRunTestWithDelay_PositiveIsInfoAndDoesNotClaimDefect(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithDelayLone {
    @Test
    fun works() = runTest {
        delay(1000)
        assert(true)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != "info" || !strings.Contains(findings[0].Message, "virtual time") {
		t.Fatalf("unexpected severity or message: %q %q", findings[0].Severity, findings[0].Message)
	}
}

func TestRunTestWithDelay_ChildCoroutineDelayIgnored(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithDelayChild {
    @Test
    fun launched() = runTest {
        launch { delay(100) }
    }

    @Test
    fun flowBuilder() = runTest {
        val f = flow { delay(100); emit(1) }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for delay in child coroutines, got %d", len(findings))
	}
}

func TestRunTestWithDelay_InlineScopeDelayStillFlagged(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithDelayInline {
    @Test
    fun works() = runTest {
        repeat(2) { delay(100) }
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for delay in an inline scope inside runTest")
	}
}

func TestRunTestWithDelay_PositiveFullyQualifiedDelay(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithDelayPositiveFqn {
    @Test
    fun works() = runTest {
        kotlinx.coroutines.delay(1000)
        assert(true)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for fully-qualified coroutine delay inside runTest")
	}
}

func TestRunTestWithDelay_Negative(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithDelayNegative {
    @Test
    fun works() = runTest {
        advanceTimeBy(1000)
        assert(true)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestRunTestWithDelay_NegativeProjectDelayReceiver(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class FakeTimer {
    suspend fun delay(millis: Long) {}
}

class RunTestWithDelayProjectReceiver {
    private val timer = FakeTimer()

    @Test
    fun works() = runTest {
        timer.delay(1000)
        assert(true)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for project-local delay receiver, got %d", len(findings))
	}
}

func TestRunTestWithDelay_NegativeLocalDelayLookalike(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

suspend fun delay(millis: Long) {}

class RunTestWithDelayLocalLookalike {
    @Test
    fun works() = runTest {
        delay(1000)
        assert(true)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for local delay lookalike, got %d", len(findings))
	}
}

func TestRunTestWithDelay_NegativeMockKAnswerDelay(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import io.mockk.coEvery
import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import org.junit.Test

interface Api {
    suspend fun fetch(): String
}

class RunTestWithDelayMockKAnswer {
    private val api: Api = TODO()

    @Test
    fun works() = runTest {
        coEvery { api.fetch() } coAnswers {
            delay(1000)
            "ok"
        }

        advanceTimeBy(1000)
        assert(true)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for delay inside MockK answer lambda, got %d", len(findings))
	}
}

func TestRunTestWithDelay_PositiveOutsideMockKAnswer(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import io.mockk.coEvery
import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import org.junit.Test

interface Api {
    suspend fun fetch(): String
}

class RunTestWithDelayAfterMockKAnswer {
    private val api: Api = TODO()

    @Test
    fun works() = runTest {
        coEvery { api.fetch() } coAnswers {
            "ok"
        }

        delay(1000)
        assert(true)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for delay outside MockK answer lambda")
	}
}

func TestRunTestWithThreadSleep_Positive(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithThreadSleep", `
package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithThreadSleepPositive {
    @Test
    fun works() = runTest {
        Thread.sleep(100)
        assert(true)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for Thread.sleep inside runTest")
	}
}

func TestRunTestWithThreadSleep_Negative(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithThreadSleep", `
package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunTestWithThreadSleepNegative {
    @Test
    fun works() = runTest {
        advanceTimeBy(100)
        assert(true)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestRunBlockingInTest_Positive(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingInTestPositive {
    @Test
    fun works() = runBlocking {
        assert(true)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for runBlocking in test")
	}
}

func TestRunBlockingInTest_NestedInRunTestSaysRemoveNested(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunBlockingInTestNested {
    @Test
    fun works() = runTest {
        runBlocking { assert(true) }
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "nested `runBlocking`") || strings.Contains(findings[0].Message, "Use `runTest`") {
		t.Fatalf("unexpected message: %s", findings[0].Message)
	}
}

func TestRunBlockingInTest_Negative(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunBlockingInTestNegative {
    @Test
    fun works() = runTest {
        assert(true)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestRunBlockingInTest_NegativeAndroidInstrumentedTest(t *testing.T) {
	file := parseInline(t, `
package test

import kotlinx.coroutines.runBlocking
import org.junit.Test

class InstrumentedRunBlockingTest {
    @Test
    fun waitsForDeviceCallback() {
        runBlocking {
            waitForCamera()
        }
    }
}

suspend fun waitForCamera() = Unit
`)
	file.Path = "/repo/camera/camera-core/src/androidTest/java/androidx/camera/core/DeviceTest.kt"
	findings := runRuleByNameOnFile(t, "RunBlockingInTest", file)
	if len(findings) != 0 {
		t.Fatalf("expected androidTest runBlocking to be ignored, got %d", len(findings))
	}
}

func TestRunBlockingInTest_UsesLocalASTOnly(t *testing.T) {
	rule := buildRuleIndex()["RunBlockingInTest"]
	if rule == nil {
		t.Fatal("RunBlockingInTest rule is not registered")
	}
	if rule.Needs != 0 {
		t.Fatalf("RunBlockingInTest should remain AST-only, got needs %v", rule.Needs)
	}
	if rule.Oracle != nil {
		t.Fatal("RunBlockingInTest should not declare oracle metadata")
	}
}

func TestRunBlockingInTest_NegativeInsideAssertionBoundary(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlin.test.assertFailsWith
import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingInAssertionNegative {
    @Test
    fun failsWithoutClock() {
        assertFailsWith<IllegalStateException> {
            runBlocking { error("boom") }
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for runBlocking inside assertion boundary, got %d", len(findings))
	}
}

func TestRunBlockingInTest_NegativeInsideRunOnIdle(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import org.junit.Test

class ComposeRule {
    fun runOnIdle(block: () -> Unit) = block()
}

class RunBlockingInRunOnIdleNegative {
    val rule = ComposeRule()

    @Test
    fun waitsForIdleCallback() {
        rule.runOnIdle {
            runBlocking { delay(700) }
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for runBlocking inside runOnIdle boundary, got %d", len(findings))
	}
}

func TestRunBlockingInTest_NegativeDispatcherThreadIdentity(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import org.junit.Test

class RunBlockingDispatcherThreadNegative {
    @Test
    fun preservesThreadIdentityAcrossDispatcherSwitch() {
        runBlocking {
            val original = Thread.currentThread()
            val switched = withContext(Dispatchers.Default) {
                Thread.currentThread()
            }

            assert(original !== switched)
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for dispatcher/thread identity test, got %d", len(findings))
	}
}

func TestRunBlockingInTest_NegativeThreadCurrentThreadIntent(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingThreadIntentNegative {
    @Test
    fun capturesRealThread() {
        runBlocking {
            val thread = Thread.currentThread()
            assert(thread.name.isNotBlank())
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for runBlocking test inspecting current thread, got %d", len(findings))
	}
}

func TestRunBlockingInTest_NegativeThreadIdentityAssertionIntent(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingThreadIdentityAssertionNegative {
    @Test
    fun comparesThreadIdentity() {
        runBlocking {
            assertThat(workerThread).isSameInstanceAs(callerThread)
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for thread identity assertion in runBlocking test, got %d", len(findings))
	}
}

func TestRunBlockingInTest_NegativeDispatcherIsDispatchNeeded(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import org.junit.Test

class RunBlockingDispatcherCheckNegative {
    @Test
    fun switchingPoolsRequiresDispatch() = runBlocking {
        var switchNeeded: Boolean? = null
        withContext(sensorPools.sampling) {
            switchNeeded = sensorPools.upload.isDispatchNeeded(coroutineContext)
        }
        assertThat(switchNeeded).isTrue()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for dispatcher isDispatchNeeded test, got %d", len(findings))
	}
}

func TestRunBlockingInTest_NegativeRealDispatcherConcurrency(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import java.util.concurrent.CountDownLatch
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingRealConcurrencyNegative {
    @Test
    fun backgroundJobSignalsStart() = runBlocking {
        val workScope = CoroutineScope(Dispatchers.Default)
        val startedLatch = CountDownLatch(1)
        val job = workScope.launch {
            startedLatch.countDown()
        }
        startedLatch.await()
        job.cancel()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for real dispatcher concurrency test, got %d", len(findings))
	}
}

func TestRunBlockingInTest_PositiveNonThreadIdentityAssertion(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingObjectIdentityPositive {
    @Test
    fun comparesObjectIdentity() {
        runBlocking {
            assertThat(first).isSameInstanceAs(second)
        }
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for runBlocking with non-thread identity assertion")
	}
}

func TestRunBlockingInTest_NegativeIntentionalComment(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import org.junit.Test

class RunBlockingIntentionalCommentNegative {
    @Test
    fun preservesDispatcherBehavior() {
        // Intentionally use runBlocking here to test real dispatcher behavior.
        runBlocking {
            withContext(Dispatchers.Default) {
                delay(1)
            }
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for intentionally documented runBlocking test, got %d", len(findings))
	}
}

func TestRunBlockingInTest_NegativeRxJavaTestObserverBridge(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import io.reactivex.rxjava3.observers.TestObserver
import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingRxJavaBridgeNegative {
    @Test
    fun bridgesSuspendSetupIntoRxAssertion() {
        runBlocking {
            whenever(invoiceDao.findOrNull(INVOICE_ID)).thenReturn(draftInvoice)
        }

        val observer = TestObserver.create<Invoice>()
        invoiceRepository.watch(INVOICE_ID).subscribe(observer)

        runBlocking {
            verify(invoiceDao).findOrNull(INVOICE_ID)
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for RxJava TestObserver bridge test, got %d", len(findings))
	}
}

func TestRunBlockingInTest_PositiveRxJavaExpressionBody(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import io.reactivex.rxjava3.observers.TestObserver
import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingRxJavaExpressionPositive {
    @Test
    fun blocksWholeTest() = runBlocking {
        val observer = TestObserver.create<Invoice>()
        invoiceRepository.watch(INVOICE_ID).subscribe(observer)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected top-level runBlocking expression body to still be reported")
	}
}

func TestRunBlockingInTest_NegativeInlineResultAsserted(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import com.google.common.truth.Truth.assertThat
import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingInlineResultNegative {
    @Test
    fun rendersSingleLineItem() {
        val result = runBlocking { invoiceMapper.render(listOf(lineItem)) }
        assertThat(result).hasSize(1)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for runBlocking bridge whose result is asserted, got %d", len(findings))
	}
}

func TestRunBlockingInTest_PositiveInlineResultNotAsserted(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import org.junit.Test

class RunBlockingInlineResultPositive {
    @Test
    fun otherTest() = runTest { assert(true) }

    @Test
    fun rendersSingleLineItem() {
        val result = runBlocking { invoiceMapper.render(listOf(lineItem)) }
        println(result)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for runBlocking bridge whose result is not asserted")
	}
}

func TestRunBlockingInTest_NegativeTurbineRunBlockingBody(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import app.cash.turbine.test
import kotlinx.coroutines.runBlocking
import org.junit.Test
import kotlin.test.assertFalse

class RunBlockingTurbineNegative {
    @Test
    fun emitsFalseByDefault() = runBlocking {
        featureToggles.watchBetaEnabled().test {
            assertFalse(awaitItem())
            cancelAndIgnoreRemainingEvents()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for runBlocking body containing Turbine assertions, got %d", len(findings))
	}
}

func TestRunBlockingInTest_PositiveVagueRunBlockingComment(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingVagueCommentPositive {
    @Test
    fun waitsForResult() {
        // Use runBlocking for this coroutine.
        runBlocking {
            delay(1)
        }
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding when comment does not document intentional dispatcher/thread behavior")
	}
}

func TestTestWithoutAssertion_Positive(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class TestWithoutAssertionPositive {
    @Test
    fun loads() {
        val x = 42
        println(x)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for test without assertion")
	}
}

func TestTestWithoutAssertion_Negative(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test
import org.junit.Assert.assertEquals

class TestWithoutAssertionNegative {
    @Test
    fun loads() {
        val x = 42
        assertEquals(42, x)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeAssertingHelpersAndSmokeTests(t *testing.T) {
	tests := map[string]string{
		"same-file helper using check": `
    private fun assertLoaded(items: List<Int>) {
        check(items.isNotEmpty())
    }
    @Test
    fun loads() {
        assertLoaded(listOf(1))
    }`,
		"same-file helper using require": `
    private fun ensureLoaded(items: List<Int>) {
        require(items.isNotEmpty()) { "empty" }
    }
    @Test
    fun loads() {
        ensureLoaded(listOf(1))
    }`,
		"direct require": `
    @Test
    fun loads() {
        require(listOf(1).isNotEmpty())
    }`,
		"mockito verify": `
    @Test
    fun notifies() {
        listener.onDone()
        verify(listener).onDone()
    }`,
		"verifyNoMoreInteractions": `
    @Test
    fun notifies() {
        listener.onDone()
        verifyNoMoreInteractions(listener)
    }`,
		"espresso check matches": `
    @Test
    fun shows() {
        onView(withId(1)).check(matches(isDisplayed()))
    }`,
		"no exception comment": `
    @Test
    fun renders() {
        // Smoke test: no exception is thrown while rendering.
        render()
    }`,
		"does not throw comment": `
    @Test
    fun renders() {
        render() // does not throw
    }`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class HelperAssertionTest {`+body+`
}
`)
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
			}
		})
	}
}

func TestTestWithoutAssertion_PositiveHelperWithoutAssertion(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class HelperPositiveTest {
    private fun prepare(items: List<Int>) {
        println(items)
    }
    @Test
    fun loads() {
        prepare(listOf(1))
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeAndroidLintExpect(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class AndroidLintExpectNegative {
    @Test
    fun reportsLintError() {
        lint()
            .files(kotlin("fun broken() = Unit"))
            .run()
            .expect("1 errors, 0 warnings")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Android lint expect chain to count as verification, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeMacrobenchmarkScopeJourney(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import androidx.benchmark.macro.MacrobenchmarkScope
import org.junit.Test

class JourneyTest {
    private val scope = MacrobenchmarkScope("com.example", true)

    @Test
    fun validateJourney() = with(scope) { runFlow() }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected macrobenchmark journey to be skipped, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeUiAutomatorJourney(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import androidx.test.uiautomator.UiDevice
import org.junit.Test

class JourneyTest {
    @Test
    fun opensSettings() {
        device.pressHome()
        device.wait(1000)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected UI Automator journey to be skipped, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeLargeTestInBenchmarkSourceSet(t *testing.T) {
	src := `
package test

import androidx.test.filters.LargeTest
import org.junit.Test

@LargeTest
class JourneyTest {
    @Test
    fun runsFlow() {
        flow.run()
    }
}
`
	if got := runRuleByNameOnPath(t, "TestWithoutAssertion", "app/benchmark/src/main/kotlin/JourneyTest.kt", src); len(got) != 0 {
		t.Fatalf("expected @LargeTest in benchmark source set to be skipped, got %d", len(got))
	}
	got := runRuleByNameOnPath(t, "TestWithoutAssertion", "app/src/test/kotlin/JourneyTest.kt", src)
	if len(got) != 1 {
		t.Fatalf("expected finding outside benchmark source set, got %d", len(got))
	}
	if got[0].Line != 10 || got[0].Col != 9 {
		t.Fatalf("expected finding anchored on function name at 10:9, got %d:%d", got[0].Line, got[0].Col)
	}
}

func TestTestWithoutAssertion_NegativeInfixAssertionHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Assert.assertEquals
import org.junit.Test

class InfixAssertionHelperNegative {
    @Test
    fun mapsInput() {
        "raw" gives "mapped"
    }
}

private infix fun String.gives(expected: String) {
    assertEquals(expected, transform(this))
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected local infix assertion helper to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeShouldStyleInfixAssertion(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class ShouldStyleInfixAssertionNegative {
    @Test
    fun routesValidUri() {
        handler shouldMatch VALID_URI
        handler shouldNotMatch WRONG_URI
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected should-style infix assertions to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeTruthChain(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import com.google.common.truth.Truth.assertThat
import org.junit.Test

class TruthChainNegative {
    @Test
    fun comparesResult() {
        assertThat(loadValue()).isEqualTo("ready")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Truth assertion chain to count as assertion, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeVerificationHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class HelperVerificationNegative {
    @Test
    fun lintPasses() {
        verifyCompoundDrawableLintPass()
    }

    private fun verifyCompoundDrawableLintPass() {
        println("fixture")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected verify-prefixed helper to count as verification, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeHelperDelegatesToLaterAssertionHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class DelegatingAssertionHelperNegative {
    @Test
    fun lintReportsErrors() {
        runLintCase()
    }

    private fun runLintCase() {
        lint().run().expectResult()
    }

    private fun Result.expectResult() {
        expect("1 errors, 0 warnings")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected helper delegating to later assertion helper to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeCheckNamedHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class CheckNamedHelperNegative {
    @Test
    fun templateIsFormatted() {
        checkTemplateFormat()
    }

    private fun checkTemplateFormat() {
        println("delegates to snapshot framework")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected check-named helper to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeVerificationNamedHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class VerificationNamedHelperNegative {
    @Test
    fun templateIsFormatted() {
        performTemplateVerification()
    }

    private fun performTemplateVerification() {
        println("delegates to a custom verification wrapper")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected verification-named helper to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeAwaitNamedHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class AwaitNamedHelperNegative {
    @Test
    fun waitsForExpectedState() {
        awaitAnd(::assertReady, expected = "ready")
    }

    private fun assertReady(expected: String) {}
    private fun awaitAnd(assertion: (String) -> Unit, expected: String) {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected await-named helper to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativePresenterRunTestDsl(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class PresenterRunTestDslNegative {
    @Test
    fun rendersInitialState() {
        presenter(SCREEN).runTest {
            state
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected presenter runTest DSL to count as assertion-bearing, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_PositiveLocalRunTestLookalike(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class LocalRunTestLookalikePositive {
    @Test
    fun doesSetupOnly() {
        helper.runTest {
            state
        }
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected non-presenter runTest lookalike without assertion evidence to be reported")
	}
}

func TestTestWithoutAssertion_NegativeSnapshotHelpers(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class SnapshotHelperNegative {
    @Test
    fun fullScreenMatchesGolden() {
        snapshotEntireScreen()
    }

    @Test
    fun contentMatchesWithInsets() {
        snapshotWithInsets {
            Content()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected snapshot-named helpers to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeSnapshotRuleGif(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Rule
import org.junit.Test

class SnapshotRuleGifNegative {
    @get:Rule
    val rule = SnapshotTestRule()

    @Test
    fun layoutMatchesGolden() {
        rule.gif(fps = 5) { context ->
            Content(context)
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected SnapshotTestRule gif to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeSnapshotBaseTestContent(t *testing.T) {
	findings := runRuleByNameOnPath(t, "TestWithoutAssertion", "src/test/kotlin/com/example/snapshot/CardSnapshotTest.kt", `
package test.snapshot

import org.junit.Test

class CardSnapshotTest : BaseSnapshotTest() {
    @Test
    fun default() {
        testContent { Card() }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected snapshot-path testContent helper to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeTurbineTestAwaitItem(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import app.cash.turbine.test
import org.junit.Test

class TurbineTestNegative {
    @Test
    fun emitsLoadedState() {
        states.test {
            awaitItem()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Turbine test block with awaitItem to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeTurbineTestWithLifecycleAwaitItem(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import app.cash.turbine.testWithLifecycle
import org.junit.Test

class TurbineLifecycleTestNegative {
    @Test
    fun emitsLoadedState() {
        states.testWithLifecycle {
            awaitItem()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Turbine testWithLifecycle block with awaitItem to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeTurbineEnsureAllEventsConsumed(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import app.cash.turbine.test
import org.junit.Test

class TurbineNoEventsTestNegative {
    @Test
    fun doesNotEmit() {
        states.test {
            ensureAllEventsConsumed()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Turbine ensureAllEventsConsumed to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeStandaloneTurbineExpectNoEvents(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import app.cash.turbine.Turbine
import org.junit.Test

class StandaloneTurbineNoEventsTestNegative {
    private val events = Turbine<String>()

    @Test
    fun doesNotEmit() {
        events.expectNoEvents()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected standalone Turbine expectNoEvents to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeCoroutineLaunchAssertion(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import com.google.common.truth.Truth.assertThat
import kotlinx.coroutines.launch
import kotlinx.coroutines.test.runTest
import org.junit.Test

class CoroutineLaunchAssertionNegative {
    @Test
    fun assertsInsideLaunch() = runTest {
        val job = launch {
            assertThat(loadState()).isEqualTo("ready")
        }
        job.join()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected assertion inside launch coroutine scope to count as assertion, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeCoroutineAsyncAssertion(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import com.google.common.truth.Truth.assertThat
import kotlinx.coroutines.async
import kotlinx.coroutines.test.runTest
import org.junit.Test

class CoroutineAsyncAssertionNegative {
    @Test
    fun assertsInsideAsync() = runTest {
        val deferred = async {
            assertThat(loadState()).isEqualTo("ready")
        }
        deferred.await()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected assertion inside async coroutine scope to count as assertion, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeReceiverLaunchAssertion(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import com.google.common.truth.Truth.assertThat
import kotlinx.coroutines.CoroutineScope
import org.junit.Test

class ReceiverLaunchAssertionNegative {
    private val scope = CoroutineScope()

    @Test
    fun assertsInsideReceiverLaunch() {
        val job = scope.launch {
            assertThat(loadState()).isEqualTo("ready")
        }
        job.cancel()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected assertion inside receiver launch coroutine scope to count as assertion, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeEspressoIntentVerification(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import androidx.test.espresso.intent.Intents.intended
import androidx.test.espresso.intent.Intents.intending
import org.junit.Test

class EspressoIntentNegative {
    @Test
    fun opensShareSheet() {
        intending(hasAction(Intent.ACTION_SEND)).respondWith(result)
        button.performClick()
        intended(hasAction(Intent.ACTION_SEND))
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Espresso intended/intending calls to count as verification, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeMockitoVerificationVariants(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test
import org.mockito.kotlin.verifyNoInteractions

class MockitoVerificationNegative {
    @Test
    fun doesNotBindUnknownText() {
        underTest.bindText(mockView, unknownFormatText)
        verifyNoInteractions(mockTextFormatter)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Mockito verification variants to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeMockitoVerifyPropertySetter(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test
import org.mockito.kotlin.verify

class MockitoVerifySetterNegative {
    @Test
    fun hidesLoading() {
        playlistController.showSpinner(true)
        verify(playlistListView).visibility = View.INVISIBLE
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Mockito verify property-setter chain to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeEspressoOnViewMatches(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import androidx.test.espresso.Espresso.onView
import androidx.test.espresso.matcher.ViewMatchers.isDisplayed
import androidx.test.espresso.matcher.ViewMatchers.withId
import org.junit.Test

class EspressoOnViewNegative {
    @Test
    fun showsChannelPurpose() {
        onView(withId(R.id.channel_purpose)).check(matches(isDisplayed()))
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Espresso onView/check/matches chain to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeComposeUiIsDisplayed(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import androidx.compose.ui.test.onNodeWithContentDescription

class ComposeUiAssertionNegative {
    @Test
    fun loading() {
        with(composeTestRule) {
            onNodeWithContentDescription("Loading")
                .isDisplayed()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Compose UI isDisplayed chain to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeComposeUiDoesNotExist(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import androidx.compose.ui.test.onNodeWithText

class ComposeUiAssertionNegative {
    @Test
    fun noError() {
        composeTestRule.onNodeWithText("Error")
            .doesNotExist()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Compose UI doesNotExist chain to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeComposeUiWaitUntilExists(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import androidx.compose.ui.test.hasTestTag
import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Rule
import org.junit.Test

class ComposeUiWaitUntilExistsNegative {
    @get:Rule
    val composeTestRule = createComposeRule()

    @Test
    fun waitsForContent() {
        composeTestRule.waitUntilAtLeastOneExists(hasTestTag("content"))
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Compose UI waitUntilAtLeastOneExists to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_PositiveLocalIsDisplayedLookalike(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

class LocalNode {
    fun isDisplayed(): Boolean = true
}

class LocalIsDisplayedLookalikePositive {
    @Test
    fun setupOnly() {
        LocalNode().isDisplayed()
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected local isDisplayed lookalike without Compose UI test evidence to be reported")
	}
}

func TestTestWithoutAssertion_PositiveWaitUntilExistsWithoutComposeEvidence(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

class LocalWaitLookalikePositive {
    @Test
    fun setupOnly() {
        waitUntilAtLeastOneExists("not compose")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected waitUntilAtLeastOneExists lookalike without Compose evidence to be reported")
	}
}

func TestTestWithoutAssertion_NegativeUiVisibilityHelpers(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import androidx.compose.ui.test.junit4.createComposeRule
import org.junit.Rule
import org.junit.Test

class UiVisibilityHelpersNegative {
    @get:Rule val composeTestRule = createComposeRule()

    @Test
    fun displaysPage() {
        detailsPage(composeTestRule) {
            isTitleDisplayed("Details")
            waitUntilAtLeastOneExists(hasText("Details"))
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected UI visibility helpers to count as assertion-equivalent, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_PositiveUiVisibilityLookalikeWithoutFrameworkEvidence(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class UiVisibilityLookalikePositive {
    @Test
    fun callsLocalWaitHelper() {
        isTitleDisplayed("Details")
    }

    private fun isTitleDisplayed(text: String) {
        println(text)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected local UI visibility lookalike without framework evidence to be reported")
	}
}

func TestTestWithoutAssertion_PositiveUncheckedLookalike(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class UncheckedLookalikePositive {
    @Test
    fun templateIsFormatted() {
        uncheckedTemplateFormat()
    }

    private fun uncheckedTemplateFormat() {
        println("not a check helper")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected unchecked lookalike not to count as assertion-equivalent")
	}
}

func TestTestWithoutAssertion_NegativeGradleOutputAssertionHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class GradleOutputTest {
    @Test
    fun verifiesDryRunTasks() {
        gradleRunner.buildAndAssertThatOutput("test", "--dry-run") {
            contains(":testDebugUnitTest ")
            doesNotContain(":testBenchmarkReleaseUnitTest ")
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Gradle output assertion helper to count as assertion, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeAssertionErrorHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class AssertionErrorHelperTest {
    @Test
    fun generatedFilesMatchGolden() {
        runGoldenTest()
    }

    private fun runGoldenTest() {
        if (!goldenMatches()) {
            throw AssertionError("golden mismatch")
        }
    }
}

fun goldenMatches(): Boolean = true
`)
	if len(findings) != 0 {
		t.Fatalf("expected AssertionError helper to count as assertion, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeNoCrashName(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class NoCrashTest {
    @Test
    fun expect_no_crash() {
        logger.logFields(null)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no-crash test name to count as deliberate smoke test, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeDoesNotThrowBacktickName(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class NoThrowNameNegative {
    @Test
    fun `+"`handler does not throw for malformed input`"+`() {
        handler.handle("")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected backtick no-throw test name to be allowed, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeNoThrowComment(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class NoThrowCommentTest {
    @Test
    fun reportFullyDrawnOnOldApi() {
        // This test makes sure that this method does not throw an exception.
        reportFullyDrawn()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no-throw comment to count as deliberate smoke test, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeShouldNotFailComment(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class ShouldNotFailCommentTest {
    @Test
    fun pluginAppliesToLibraryModule() {
        gradleRunner.withArguments("generateBaselineProfile").build()
        // This should not fail.
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected should-not-fail comment to count as deliberate smoke test, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativePollingCheckWaitFor(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import androidx.testutils.PollingCheck
import org.junit.Test

class PollingCheckWaitForNegative {
    @Test
    fun backDismissesActionMode() {
        PollingCheck.waitFor { destroyed.get() }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected PollingCheck.waitFor to count as assertion-by-timeout, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeComposeExpectError(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import androidx.compose.testutils.expectError
import org.junit.Test

class ComposeExpectErrorNegative {
    @Test
    fun moveToWithoutDown() {
        expectError<IllegalStateException> { gesture.moveTo(position) }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected Compose expectError to count as verification, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_PositiveLocalExpectErrorLookalike(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class LocalExpectErrorLookalikePositive {
    @Test
    fun moveToWithoutDown() {
        expectError<IllegalStateException> { gesture.moveTo(position) }
    }

    private fun <T : Throwable> expectError(block: () -> Unit) {
        block()
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected local expectError lookalike without Compose testutils import to be reported")
	}
}

func TestTestWithoutAssertion_PositiveLocalWaitForLookalike(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class LocalWaitForLookalikePositive {
    @Test
    fun waitsWithoutVerification() {
        waitFor { destroyed.get() }
    }

    private fun waitFor(block: () -> Boolean) {
        block()
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected local waitFor lookalike without PollingCheck import to be reported")
	}
}

func TestTestWithoutAssertion_PositiveLocalTurbineTestLookalike(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class LocalTurbineTestLookalikePositive {
    @Test
    fun waitsWithoutVerification() {
        states.test {
            awaitItem()
        }
    }

    private fun <T> Flow<T>.test(block: () -> Unit) {
        block()
    }

    private fun awaitItem() = Unit
}
`)
	if len(findings) == 0 {
		t.Fatal("expected local test lookalike without Turbine import to be reported")
	}
}

func TestTestWithoutAssertion_NegativeLocalAssertionHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Assert.assertArrayEquals
import org.junit.Test

class LocalAssertionHelperNegative {
    @Test
    fun delegatesToHelper() {
        testMacEquality()
    }

    private fun testMacEquality() {
        assertArrayEquals(byteArrayOf(1), byteArrayOf(1))
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected local helper with assertion to count as assertion, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeExplicitThisHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class ExplicitThisHelperNegative {
    @Test
    fun delegatesToHarness() {
        this.runHarness()
    }

    private fun runHarness() {
        println("external framework assertion")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected explicit this helper call to count as delegated assertion, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeExplicitSuperHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

open class BaseHarness {
    fun runHarness() {}
}

class ExplicitSuperHelperNegative : BaseHarness() {
    @Test
    fun delegatesToBaseHarness() {
        super.runHarness()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected explicit super helper call to count as delegated assertion, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_PositiveImplicitNeutralHelper(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class ImplicitNeutralHelperPositive {
    @Test
    fun delegatesToSetupOnly() {
        runHarness()
    }

    private fun runHarness() {
        println("setup only")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected implicit neutral helper call without assertion evidence to be reported")
	}
}

func TestTestWithoutAssertion_NegativeCompilerHarnessCompile(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class CompilerHarnessNegative : MetroCompilerTest() {
    @Test
    fun generatedCodeCompiles() {
        compile(source("class Example"))
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected compiler harness compile call to count as verification, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_PositiveLocalCompileLookalike(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class LocalCompileLookalikePositive {
    @Test
    fun generatedCodeCompiles() {
        compile(source("class Example"))
    }

    private fun compile(source: String) {
        println(source)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected local compile lookalike without harness evidence to be reported")
	}
}

func TestTestWithoutAssertion_NegativeGradleBuilderBuild(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import com.autonomousapps.kit.GradleBuilder.build
import org.junit.Test

class GradleBuilderBuildNegative {
    @Test
    fun projectBuilds() {
        build(project.rootDir, "compileKotlin")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected imported GradleBuilder.build call to count as verification, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeIncrementalCompileKotlinAndFail(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class IncrementalBuildNegative : BaseIncrementalCompilationTest() {
    @Test
    fun projectFailsAfterChange() {
        project.compileKotlinAndFail()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected incremental compileKotlinAndFail call to count as verification, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeInfixAssertion(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test
import com.example.testing.assertHasSize

class TestWithoutAssertionInfixNegative {
    @Test
    fun loads() {
        val values = listOf(1, 2)
        values assertHasSize 2
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for infix assertion, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeIgnoredClass(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Ignore
import org.junit.Test

@Ignore("manual preview")
class TestWithoutAssertionIgnoredNegative {
    @Test
    fun preview() {
        println("manual")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for ignored test class, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeExpectedException(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class TestWithoutAssertionExpectedExceptionNegative {
    @Test(expected = IllegalArgumentException::class)
    fun throwsOnBadInput() {
        parse("")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for expected-exception test, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeAssertionErrorThrow(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class AssertionErrorThrowNegative {
    @Test
    fun all() {
        runTests()
    }

    private fun runTests() {
        if (failed()) {
            throw AssertionError("Some tests failed")
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected helper throwing AssertionError to count as assertion, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeNoCrashSmokeTestName(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", ""+
		`
package test

import org.junit.Test

class NoCrashSmokeNegative {
    @Test
    fun `+"`"+`Given null, when I logFields, then I expect no crash`+"`"+`() {
        logFields(null)
    }

    @Test
    fun `+"`"+`delay completes without real wall-clock wait`+"`"+`() {
        providerDelay()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no-crash/no-throw smoke tests to be accepted, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeIdeaTemplatePath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), ".claude", "skills", ".idea", "fileTemplates")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "GeneratedTest.kt")
	code := `package test

import org.junit.Test

class GeneratedTestTemplate {
    @Test
    fun generatedTemplate() {
        println("template placeholder")
    }
}
`
	if err := os.WriteFile(path, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}
	file, err := scanner.ParseFile(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	findings := runRuleByNameOnFile(t, "TestWithoutAssertion", file)
	if len(findings) != 0 {
		t.Fatalf("expected IDE template paths to be ignored for %q, got %d: %#v", file.Path, len(findings), findings)
	}
}

func TestTestWithoutAssertion_ConfigAllowsNoAssertionTests(t *testing.T) {
	rule := buildRuleIndex()["TestWithoutAssertion"]
	if rule == nil {
		t.Fatal("TestWithoutAssertion rule is not registered")
	}
	metaProvider, ok := rule.Implementation.(api.MetaProvider)
	if !ok {
		t.Fatal("TestWithoutAssertion implementation does not expose metadata")
	}
	meta := metaProvider.Meta()
	cfg := api.NewFakeConfigSource()
	cfg.Set("testing-quality", "TestWithoutAssertion", "allowNoAssertionTests", true)
	if active := api.ApplyConfig(rule.Implementation, meta, cfg); !active {
		t.Fatal("expected TestWithoutAssertion to stay active")
	}
	t.Cleanup(func() {
		reset := api.NewFakeConfigSource()
		reset.Set("testing-quality", "TestWithoutAssertion", "allowNoAssertionTests", false)
		api.ApplyConfig(rule.Implementation, meta, reset)
	})

	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class SmokeTest {
    @Test
    fun attachesAndDetachesPresenter() {
        timerPresenter.attach(stubTimerView)
        timerPresenter.detach()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected configured no-assertion tests to be accepted, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_ConfigAssertionMethodPatterns(t *testing.T) {
	rule := buildRuleIndex()["TestWithoutAssertion"]
	if rule == nil {
		t.Fatal("TestWithoutAssertion rule is not registered")
	}
	metaProvider, ok := rule.Implementation.(api.MetaProvider)
	if !ok {
		t.Fatal("TestWithoutAssertion implementation does not expose metadata")
	}
	meta := metaProvider.Meta()
	cfg := api.NewFakeConfigSource()
	cfg.Set("testing-quality", "TestWithoutAssertion", "assertionMethodPatterns", []string{"confirm*", "requireScreenState"})
	if active := api.ApplyConfig(rule.Implementation, meta, cfg); !active {
		t.Fatal("expected TestWithoutAssertion to stay active")
	}
	t.Cleanup(func() {
		reset := api.NewFakeConfigSource()
		reset.Set("testing-quality", "TestWithoutAssertion", "assertionMethodPatterns", []string{})
		api.ApplyConfig(rule.Implementation, meta, reset)
	})

	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class TestDslAssertionPatternNegative {
    private val robot = Robot()

    @Test
    fun rendersInitialState() {
        robot.confirmInitialState()
    }

    @Test
    fun rendersScreenState() {
        requireScreenState()
    }
}

class Robot {
    fun confirmInitialState() {}
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected configured assertion method patterns to be accepted, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_PositiveUnconfiguredNeutralDslCall(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class NeutralDslPositive {
    private val robot = Robot()

    @Test
    fun rendersInitialState() {
        robot.confirmInitialState()
    }
}

class Robot {
    fun confirmInitialState() {}
}
`)
	if len(findings) == 0 {
		t.Fatal("expected neutral test DSL calls to require configured assertion method patterns")
	}
}

func TestTestWithoutAssertion_SeverityWarning(t *testing.T) {
	rule := buildRuleIndex()["TestWithoutAssertion"]
	if rule == nil {
		t.Fatal("TestWithoutAssertion rule is not registered")
	}
	if rule.Sev != api.Severity("warning") {
		t.Fatalf("expected TestWithoutAssertion registry severity warning, got %q", rule.Sev)
	}
	meta, ok := rules.MetaForRule(rule)
	if !ok {
		t.Fatal("TestWithoutAssertion rule has no descriptor")
	}
	if meta.Severity != "warning" {
		t.Fatalf("expected TestWithoutAssertion metadata severity warning, got %q", meta.Severity)
	}
}

func TestTestWithoutAssertion_UsesLocalASTOnly(t *testing.T) {
	rule := buildRuleIndex()["TestWithoutAssertion"]
	if rule == nil {
		t.Fatal("TestWithoutAssertion rule is not registered")
	}
	if rule.Needs != api.NeedsCrossFile|api.NeedsConcurrent {
		t.Fatalf("TestWithoutAssertion should need only the cross-file index (and run concurrently), got needs %v", rule.Needs)
	}
	if rule.Oracle != nil {
		t.Fatal("TestWithoutAssertion should not declare oracle metadata")
	}
}

func TestTestWithOnlyTodo_Positive(t *testing.T) {
	findings := runRuleByName(t, "TestWithOnlyTodo", `
package test

import org.junit.Test

class TestWithOnlyTodoPositive {
    @Test
    fun loads() {
        TODO()
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for test with only TODO")
	}
}

func TestTestWithOnlyTodo_Negative(t *testing.T) {
	findings := runRuleByName(t, "TestWithOnlyTodo", `
package test

import org.junit.Ignore
import org.junit.Test

class TestWithOnlyTodoNegative {
    @Test
    @Ignore
    fun loads() {
        TODO()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestTestFunctionReturnValue_Positive(t *testing.T) {
	findings := runRuleByName(t, "TestFunctionReturnValue", `
package test

import org.junit.Test

class TestFunctionReturnValuePositive {
    @Test
    fun fingerprint(): String = "abc"
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for test function with return value")
	}
}

func TestTestFunctionReturnValue_Negative(t *testing.T) {
	findings := runRuleByName(t, "TestFunctionReturnValue", `
package test

import org.junit.Test

class TestFunctionReturnValueNegative {
    @Test
    fun fingerprint() {
        assert("abc" == "abc")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestRelaxedMockUsedForValueClass_Positive(t *testing.T) {
	findings := runRuleByName(t, "RelaxedMockUsedForValueClass", `
package test

import io.mockk.mockk
import org.junit.Test

class RelaxedMockPositive {
    @Test
    fun works() {
        val id = mockk<Long>(relaxed = true)
        assert(id != null)
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for relaxed mock of primitive type")
	}
}

func TestRelaxedMockUsedForValueClass_Negative(t *testing.T) {
	findings := runRuleByName(t, "RelaxedMockUsedForValueClass", `
package test

import io.mockk.mockk
import org.junit.Test

interface Api {
    fun get(): String
}

class RelaxedMockNegative {
    @Test
    fun works() {
        val api = mockk<Api>(relaxed = true)
        assert(api != null)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestSpyOnDataClass_Positive(t *testing.T) {
	findings := runRuleByName(t, "SpyOnDataClass", `
package test

import io.mockk.spyk
import org.junit.Test

data class User(val name: String)

class SpyOnDataClassPositive {
    @Test
    fun works() {
        val user = spyk(User("alice"))
        assert(user.name == "alice")
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for spy on data class")
	}
}

func TestSpyOnDataClass_Negative(t *testing.T) {
	findings := runRuleByName(t, "SpyOnDataClass", `
package test

import io.mockk.spyk
import org.junit.Test

open class Service {
    fun compute(): Int = 42
}

class SpyOnDataClassNegative {
    @Test
    fun works() {
        val service = spyk<Service>()
        assert(service.compute() == 42)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Positive(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.verify
import org.junit.Test

class RealApi {
    fun get(): String = "data"
}

class VerifyWithoutMockPositive {
    @Test
    fun works() {
        val api = RealApi()
        api.get()
        verify { api.get() }
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for verify without mock, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Negative(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.mockk
import io.mockk.verify
import org.junit.Test

interface Api {
    fun get(): String
}

class VerifyWithoutMockNegative {
    @Test
    fun works() {
        val api = mockk<Api>()
        api.get()
        verify { api.get() }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Negative_ClassAndHelperMocks(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.mockk
import io.mockk.spyk
import io.mockk.verify
import org.junit.Before
import org.junit.Test

interface Api {
    fun get(value: String): String
}

object DataSet {
    val VALUE = "data"
}

class VerifyWithoutMockAppStyleNegative {
    private val fieldMock = mockk<Api>()
    private lateinit var setupMock: Api
    private lateinit var helperMock: Api
    private lateinit var spyMock: Api

    @Before
    fun setUp() {
        setupMock = mockk()
        helperMock = buildMock()
        spyMock = spyk(fieldMock)
    }

    @Test
    fun works() {
        val localMock = buildMock()
        verify { fieldMock.get(DataSet.VALUE) }
        verify { setupMock.get(DataSet.VALUE) }
        verify { helperMock.get(DataSet.VALUE) }
        verify { spyMock.get(DataSet.VALUE) }
        verify { localMock.get(DataSet.VALUE) }
    }

    private fun buildMock(): Api {
        val mock = mockk<Api>()
        return mock
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Negative_ChainedMockInitializer(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.every
import io.mockk.mockk
import io.mockk.verify
import org.junit.Test

interface Api {
    fun get(): String
}

class VerifyWithoutMockChainedInitializerNegative {
    @Test
    fun works() {
        val api = mockk<Api>().apply {
            every { get() } returns "data"
        }

        verify { api.get() }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Negative_ExpressionBodyMockHelper(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.every
import io.mockk.mockk
import io.mockk.verify
import org.junit.Test

interface Logger {
    fun verbose(value: String)
}

class VerifyWithoutMockExpressionBodyHelperNegative {
    @Test
    fun works() {
        val logger = verboseMockLogger()

        verify { logger.verbose("saved") }
    }
}

private fun verboseMockLogger() = mockk<Logger> {
    every { verbose(any()) } returns Unit
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Negative_StaticMockClassLiteral(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.mockkStatic
import io.mockk.verify
import org.junit.Test

object Toast {
    fun makeText(value: String) {}
}

class VerifyWithoutMockStaticClassLiteralNegative {
    @Test
    fun works() {
        mockkStatic(Toast::class)

        verify { Toast.makeText("saved") }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Negative_TestHelperFunction(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.MockKVerificationScope
import io.mockk.verify
import javax.inject.Provider

fun assertDelegation(
    provider: Provider<*>,
    verificationBlock: MockKVerificationScope.() -> Unit,
) {
    verify(exactly = 1) {
        provider.get()
        verificationBlock()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Negative_ChainedReceiverFromMock(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.MockKAnnotations
import io.mockk.impl.annotations.MockK
import io.mockk.verify
import org.junit.Before
import org.junit.Test

interface SearchBarManager {
    fun close()
}

interface Repository {
    val searchBarManager: SearchBarManager?
}

class VerifyWithoutMockChainedReceiverNegative {
    @MockK
    private lateinit var repository: Repository

    @Before
    fun setup() {
        MockKAnnotations.init(this)
    }

    @Test
    fun works() {
        verify {
            repository.searchBarManager?.close()
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Negative_MockNamedSetupProperty(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.verify
import org.junit.Test

interface Repository {
    fun fetch()
}

class Setup(val mockRepository: Repository)

class VerifyWithoutMockNamedSetupPropertyNegative {
    @Test
    fun works() {
        val setup = Setup(createRepository())

        verify {
            setup.mockRepository.fetch()
        }
    }

    private fun createRepository(): Repository = TODO()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Negative_AssertionInsideVerify(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.mockk
import io.mockk.verify
import org.junit.Assert
import org.junit.Test

interface Repository {
    fun save(value: String)
}

class VerifyWithoutMockAssertionInsideVerifyNegative {
    @Test
    fun works() {
        val repository = mockk<Repository>()
        val slot = mutableListOf<String>()

        verify {
            repository.save("value")
            Assert.assertEquals("value", slot.firstOrNull())
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Negative_ForEachWrapper(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.MockKAnnotations
import io.mockk.impl.annotations.MockK
import io.mockk.verify
import org.junit.Before
import org.junit.Test

interface Telemetry {
    fun log(value: String)
}

class VerifyWithoutMockForEachWrapperNegative {
    @MockK
    private lateinit var telemetry: Telemetry

    @Before
    fun setup() {
        MockKAnnotations.init(this)
    }

    @Test
    fun works() {
        val values = listOf("a", "b")

        verify {
            values.forEach {
                telemetry.log(it)
            }
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Negative_NonMockKVerifyLambda(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import org.mockito.MockedStatic
import org.mockito.kotlin.verify
import org.junit.Test

object IdentityUtil {
    fun saveIdentity(value: String) {}
}

class VerifyWithoutMockMockitoStaticNegative {
    lateinit var staticIdentityUtil: MockedStatic<IdentityUtil>

    @Test
    fun works() {
        val otherAci = "aci"
        staticIdentityUtil.verify { IdentityUtil.saveIdentity(otherAci.toString()) }
        verify(staticIdentityUtil).close()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestVerifyWithoutMock_Positive_DirectReceiverOnly(t *testing.T) {
	findings := runRuleByName(t, "VerifyWithoutMock", `
package test

import io.mockk.verify
import org.junit.Test

class Api {
    fun update(value: String) {}
}

object DataSet {
    val VALUE = "data"
}

class VerifyWithoutMockDirectReceiverPositive {
    @Test
    fun works() {
        val api = Api()
        verify { api.update(DataSet.VALUE) }
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding for the direct verified receiver, got %d", len(findings))
	}
	if findings[0].Message == "" || !strings.Contains(findings[0].Message, "`api`") {
		t.Fatalf("expected finding to mention api receiver, got %q", findings[0].Message)
	}
}

func TestTestDispatcherNotInjected_PositiveConstructorArgument(t *testing.T) {
	findings := runRuleByName(t, "TestDispatcherNotInjected", `
package test

import kotlinx.coroutines.Dispatchers
import org.junit.Test

class DispatcherInjectionPositive {
    @Test
    fun works() {
        val repository = Repository(Dispatchers.IO)
        assert(repository.load() == "data")
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestTestDispatcherNotInjected_NegativeTestSideCoroutines(t *testing.T) {
	findings := runRuleByName(t, "TestDispatcherNotInjected", `
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.async
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext
import org.junit.Test

class DispatcherInjectionNegative {
    @Test
    fun works() = runTest {
        val a = withContext(Dispatchers.IO) { readFile() }
        launch(Dispatchers.Default) { readFile() }
        val b = async(Dispatchers.IO) { readFile() }
        assert(a == b.await())
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for test-side dispatcher use, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_PlantedFailuresAreImplicitAssertions(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class PlantedFailures {
    @Test
    fun observerThrows() {
        val sink = object : AuditSink() {
            override fun write(entry: String) {
                throw IllegalStateException()
            }
        }
        AuditLog.attach(sink)
        try {
            ReceiptBuilder().setNote("<>").build()
        } finally {
            AuditLog.detach(sink)
        }
    }

    @Test
    fun lazyFails() {
        buildScreen(enabled = false, loader = Lazy { error("loaded while disabled") })
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
	findings = runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class StillReported {
    @Test
    fun noChecks() {
        val sink = AuditSink()
        AuditLog.attach(sink)
        try {
            ReceiptBuilder().setNote("<>").build()
        } finally {
            AuditLog.detach(sink)
        }
    }

    @Test
    fun mockCallOnly() {
        presenter.attach(view)
        presenter.detach()
    }
}
`)
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}
}

func TestRunTestWithThreadSleep_SkipsRealConcurrency(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithThreadSleep", `
package test

import java.util.concurrent.CountDownLatch
import kotlinx.coroutines.test.runTest
import org.junit.Test

class RealThreads {
    @Test
    fun latch() = runTest {
        val started = CountDownLatch(1)
        val first = launch(realDispatchers.default) { poll() }
        started.await(5, TimeUnit.SECONDS)
        Thread.sleep(50)
        started.countDown()
        first.join()
        assertThat(state()).isEqualTo(1)
    }

    @Test
    fun ioDispatcher() = runTest {
        val job = launch(Dispatchers.IO) { work() }
        Thread.sleep(50)
        job.join()
        assertThat(state()).isEqualTo(1)
    }

}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for real concurrency, got %d", len(findings))
	}
	findings = runRuleByName(t, "RunTestWithThreadSleep", `
package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class VirtualTime {
    @Test
    fun plainVirtualTime() = runTest {
        work()
        Thread.sleep(50)
        assertThat(state()).isEqualTo(1)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected the virtual-time test to be reported, got %d", len(findings))
	}
}

func TestRunTestWithThreadSleep_NegativeNestedScopes(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithThreadSleep", `
package test

import kotlinx.coroutines.test.runTest
import org.junit.Test

class ThreadSleepNested {
    @Test
    fun works() = runTest {
        val worker = object : Runnable {
            override fun run() {
                Thread.sleep(10)
            }
        }
        fun pause() {
            Thread.sleep(10)
        }
        while (!worker.isDone()) {
            Thread.sleep(5)
        }
        assert(true)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestRunBlockingInTest_NegativeClassUsesRealDispatchers(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext
import org.junit.Test

class RealDispatcherTest {
    @Test
    fun works() = runBlocking {
        val value = withContext(Dispatchers.IO) { compute() }
        assert(value == 1)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

const realDispatcherMarkerSource = `
package test

import kotlinx.coroutines.runBlocking
import org.junit.Test

@Suppress("UsesRealDispatchers")
class MarkedTest {
    @Test
    fun works() = runBlocking {
        assert(true)
    }
}
`

func TestRunBlockingInTest_NegativeConfiguredRealDispatcherMarker(t *testing.T) {
	var impl *rules.RunBlockingInTestRule
	for _, r := range api.Registry {
		if r.ID == "RunBlockingInTest" {
			impl = r.Implementation.(*rules.RunBlockingInTestRule)
		}
	}
	if impl == nil {
		t.Fatal("RunBlockingInTest not registered")
	}
	saved := impl.RealDispatcherMarkers
	t.Cleanup(func() { impl.RealDispatcherMarkers = saved })
	impl.RealDispatcherMarkers = []string{"UsesRealDispatchers"}
	if findings := runRuleByName(t, "RunBlockingInTest", realDispatcherMarkerSource); len(findings) != 0 {
		t.Fatalf("expected no findings with a configured marker, got %d", len(findings))
	}
}

func TestRunBlockingInTest_PositiveMarkerNotConfiguredByDefault(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", realDispatcherMarkerSource)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding without a configured marker, got %d", len(findings))
	}
}

func TestSharedMutableStateInObject_NegativeResetInAfter(t *testing.T) {
	findings := runRuleByName(t, "SharedMutableStateInObject", `
package test

import org.junit.After
import org.junit.Test

class SharedStateTest {
    @After
    fun tearDown() {
        Holder.value = 0
    }

    @Test
    fun works() {
        Holder.value = 1
        assert(Holder.value == 1)
    }

    companion object Holder {
        var value = 0
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestSharedMutableStateInObject_PositiveUnresetVar(t *testing.T) {
	findings := runRuleByName(t, "SharedMutableStateInObject", `
package test

import org.junit.Test

class SharedStatePositive {
    @Test
    fun works() {
        assert(true)
    }

    companion object {
        var value = 0
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestSharedMutableStateInObject_OneFindingPerProperty(t *testing.T) {
	findings := runRuleByName(t, "SharedMutableStateInObject", `
package test

import org.junit.Test

class SharedStateMany {
    @Test
    fun works() {
        assert(true)
    }

    companion object {
        var first = 0
        var second = ""
        var third: Int? = null
    }
}
`)
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings (one per property), got %d", len(findings))
	}
	for i, want := range []string{"first", "second", "third"} {
		if !strings.Contains(findings[i].Message, "'"+want+"'") {
			t.Fatalf("finding %d = %q, want property %s", i, findings[i].Message, want)
		}
	}
}

func TestSharedMutableStateInObject_SkipsTestSupportObjects(t *testing.T) {
	for name, src := range map[string]string{
		"fake name":      "object FakeClock { var now = 0L }",
		"stub name":      "object ClockStub { var now = 0L }",
		"annotated":      "@VisibleForTesting object Clock { var now = 0L }",
		"annotated prop": "object Clock { @VisibleForTesting var now = 0L }",
	} {
		t.Run(name, func(t *testing.T) {
			findings := runRuleByName(t, "SharedMutableStateInObject", `
package test

import org.junit.Test

annotation class VisibleForTesting

class SomethingTest {
    @Test
    fun works() {}
}

`+src+"\n")
			if len(findings) != 0 {
				t.Fatalf("expected no findings, got %d", len(findings))
			}
		})
	}
}

func TestAssertNullableWithNotNullAssertion_NegativeEarlierTruthIsNotNull(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

import com.google.common.truth.Truth.assertThat

fun testNullable(vm: Vm) {
    assertThat(vm.state.user).isNotNull()
    assertThat(vm.state.user!!.name).isEqualTo("x")
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings after an earlier isNotNull assertion, got %d", len(findings))
	}
}

func TestAssertNullableWithNotNullAssertion_PositiveAssertionOnOtherExpression(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

import com.google.common.truth.Truth.assertThat

fun testNullable(vm: Vm) {
    assertThat(vm.other).isNotNull()
    assertThat(vm.state.user!!.name).isEqualTo("x")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected a finding when the earlier assertion targets a different expression")
	}
}

func TestAssertNullableWithNotNullAssertion_EarlierCheckMustBeAReachingCall(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"comment", "// assertNotNull(maybe)\n    assertEquals(\"x\", maybe!!)", 1},
		{"string", "println(\"assertNotNull(maybe)\")\n    assertEquals(\"x\", maybe!!)", 1},
		{"other branch", "if (flag) {\n        assertNotNull(maybe)\n    }\n    assertEquals(\"x\", maybe!!)", 1},
		{"braceless branch", "if (flag) assertNotNull(maybe)\n    assertEquals(\"x\", maybe!!)", 1},
		{"lambda", "run { assertNotNull(maybe) }\n    assertEquals(\"x\", maybe!!)", 1},
		{"same block", "assertNotNull(maybe)\n    assertEquals(\"x\", maybe!!)", 0},
		{"declaration", "val checked = requireNotNull(maybe)\n    assertEquals(checked, maybe!!)", 0},
		{"message first", "assertNotNull(\"must exist\", maybe)\n    assertEquals(\"x\", maybe!!)", 0},
		{"enclosing block", "checkNotNull(maybe)\n    if (flag) {\n        assertEquals(\"x\", maybe!!)\n    }", 0},
		{"hamcrest", "assertThat(maybe, notNullValue())\n    assertEquals(\"x\", maybe!!)", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

import org.hamcrest.MatcherAssert.assertThat
import org.hamcrest.Matchers.notNullValue
import org.junit.Assert.assertEquals
import org.junit.Assert.assertNotNull

fun check(maybe: String?, flag: Boolean) {
    `+tc.body+`
}
`)
			if len(findings) != tc.want {
				t.Fatalf("want %d findings, got %d", tc.want, len(findings))
			}
		})
	}
}

func TestTestWithoutAssertion_ExpectAndAwaitCallsCount(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

class SampleTest {
    @Test
    fun noEvents() {
        navigator.expectNoEvents()
    }

    @Test
    fun nextScreen() {
        navigator.awaitNextScreen()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_ErrorRecoveryBodyAbstains(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

class SampleTest {
    @Test
    fun stored() {
        handles.single().mutable.value = State.Done(stats(6))
        assertThat(handles.single().cancels).isEqualTo(1)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a body damaged by error recovery, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_StillReportsPlainCalls(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

class SampleTest {
    @Test
    fun plain() {
        doSomething()
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
}

func TestRunTestWithDelay_FakeLambdaPropertyIgnored(t *testing.T) {
	findings := runRuleByName(t, "RunTestWithDelay", `
package test

import kotlinx.coroutines.delay
import kotlinx.coroutines.test.runTest
import org.junit.Test

class FakeRepo { var onFetch: suspend () -> Unit = {} }

class RunTestWithDelayFakeLambda {
    @Test
    fun applied() = runTest {
        val fake = FakeRepo().apply { onFetch = { delay(100) } }
        val other = FakeRepo()
        other.onFetch = { delay(50) }
        assert(fake !== other)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for delay in a stored lambda, got %d", len(findings))
	}
}

func TestRunBlockingInTest_RxTestSchedulerIgnored(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import io.reactivex.rxjava3.schedulers.TestScheduler
import kotlinx.coroutines.runBlocking
import org.junit.Test

class RunBlockingRxScheduler {
    @Test
    fun drives() {
        val scheduler = TestScheduler()
        val result = runBlocking { compute() }
        scheduler.triggerActions()
        println(result)
    }
    suspend fun compute(): Int = 1
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings with an Rx TestScheduler, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_RoboImageHelpersAndEmptyFixturesAreNotReported(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class SnapshotSuite {
    @Test fun popupExpanded() = captureExpanded()

    @Test fun direct() {
        captureScreenRoboImage()
    }

    @Test fun compared() {
        compareRoboImage("golden.png")
    }

    @Config(sdk = [36]) @Test fun anchor() = Unit

    @Test fun placeholder() {
    }

    private fun captureExpanded() {
        composeTestRule.waitForIdle()
        captureScreenRoboImage()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("snapshot captures and empty fixture tests are not missing assertions, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_AnchorsAtFunctionNameNotAnnotation(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class Suite {
    @Test
    fun loads() {
        println(42)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(findings))
	}
	if findings[0].Line != 8 {
		t.Fatalf("expected the finding on the fun name line 8, got %d", findings[0].Line)
	}
}

func TestAssertNullableWithNotNullAssertion_NegativeFileDeclaredLibraryName(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

fun testHelper(result: Result?) {
    assertEquals(result!!.items)
}

private fun assertEquals(items: List<String>) {}

class Result(val items: List<String>)
`)
	if len(findings) != 0 {
		t.Fatalf("expected file-declared function to shadow the library name, got %d", len(findings))
	}
}

func TestAssertNullableWithNotNullAssertion_PositiveTruthAssertThat(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

import com.google.common.truth.Truth.assertThat

fun testNullable(maybeX: String?) {
    assertThat(maybeX!!).isEqualTo("x")
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for assertThat with !! argument")
	}
}

func TestAssertNullableWithNotNullAssertion_NegativeAssertNotNullArgument(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

import kotlin.test.assertNotNull

fun testNullable(maybeX: String?) {
    assertNotNull(maybeX!!)
}
`)
	if len(findings) != 0 {
		t.Fatalf("assertNotNull cannot replace itself, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_NegativeWaitAndEnsureHelpers(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class RobotTest {
    private val robot = ScreenRobot()

    @Test
    fun waitsForRow() {
        robot.open()
        robot.waitForRowToAppear("a")
    }

    @Test
    fun ensuresNoEvents() {
        robot.open()
        ensureNoRemainingEvents()
    }

    @Test
    fun waitsUntilGone() {
        robot.open()
        waitUntilRowGone("a")
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected wait/ensure helpers to count as assertions, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_PositiveWaiterLookalike(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class RobotTest {
    @Test
    fun opens() {
        waiter()
        ensured()
        waitForward()
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected the empty test to be reported, got %d", len(findings))
	}
}

func TestRunBlockingInTest_SkipsSingleCallInNonCoroutineTest(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.runBlocking
import org.junit.Test

class SchedulerBasedTest {
    @Test
    fun works() {
        val subject = Subject()
        subject.configure()
        val result = runBlocking { subject.load() }
        subject.finish(result)
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestRunBlockingInTest_ReportsInCoroutineTestFile(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import org.junit.Test

class CoroutineBasedTest {
    @Test
    fun usesRunTest() = runTest { assert(true) }

    @Test
    fun usesRunBlocking() {
        val subject = Subject()
        val result = runBlocking { subject.load() }
        subject.finish(result)
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestRunBlockingInTest_ReportsWholeBodyBlock(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", `
package test

import kotlinx.coroutines.runBlocking
import org.junit.Test

class WholeBodyTest {
    @Test
    fun works() {
        runBlocking { assert(true) }
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

const runBlockingSchedulerFileHeader = `
package test

import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.TestCoroutineScheduler
import kotlinx.coroutines.test.runTest
import org.junit.Test

class SchedulerDrivenTest {
    private val scheduler = TestCoroutineScheduler()
    private val controller = FakeController()

`

func TestRunBlockingInTest_SkipsFakeCallWhenTestDrivesOwnScheduler(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", runBlockingSchedulerFileHeader+`
    @Test
    fun holdsReply() {
        runBlocking { controller.start() }
        val job = hold()
        scheduler.runCurrent()
        assertThat(job.isActive).isTrue()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d: %v", len(findings), findings)
	}
}

func TestRunBlockingInTest_SchedulerDrivenFileStillReportsWholeBody(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", runBlockingSchedulerFileHeader+`
    @Test
    fun wholeBody() = runBlocking {
        controller.start()
        scheduler.advanceUntilIdle()
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestRunBlockingInTest_SchedulerDrivenFileStillReportsNestedInRunTest(t *testing.T) {
	findings := runRuleByName(t, "RunBlockingInTest", runBlockingSchedulerFileHeader+`
    @Test
    fun nested() = runTest {
        runBlocking { controller.start() }
        scheduler.advanceUntilIdle()
    }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
	}
}

func TestAssertNullableWithNotNullAssertion_TruncatesLongOperand(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

fun testItems(repository: Repository?) {
    assertEquals("x", repository
        ?.loadSomethingWithAVeryLongDescriptiveNameForTheFixture(argumentNumberOne, argumentNumberTwo)
        ?.items!!.size)
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(findings))
	}
	msg := findings[0].Message
	if strings.Contains(msg, "\n") || len(msg) > 140 {
		t.Fatalf("message should be one short line, got %q", msg)
	}
	if !strings.Contains(msg, "...") {
		t.Fatalf("long operand should be truncated, got %q", msg)
	}
}

func TestAssertNullableWithNotNullAssertion_TruthFilesDoNotSuggestJUnitAssertNotNull(t *testing.T) {
	findings := runRuleByName(t, "AssertNullableWithNotNullAssertion", `
package test

import com.google.common.truth.Truth.assertThat

fun testItems(maybe: Item?, other: String?) {
    assertEquals("x", maybe!!.name)
    assertEquals("x", other!!)
}
`)
	if len(findings) != 2 {
		t.Fatalf("both bangs stay flagged, got %d", len(findings))
	}
	for _, f := range findings {
		if strings.Contains(f.Message, "assertNotNull") {
			t.Fatalf("Truth file should not be told to use assertNotNull: %q", f.Message)
		}
		if !strings.Contains(f.Message, "isNotNull()") {
			t.Fatalf("Truth file should be told to use isNotNull(): %q", f.Message)
		}
	}
}

func TestTestFunctionReturnValue_AllowsTestResult(t *testing.T) {
	findings := runRuleByName(t, "TestFunctionReturnValue", `
package test

import kotlinx.coroutines.test.TestResult
import kotlinx.coroutines.test.runTest
import org.junit.Test

class ResultTest {
    @Test
    fun runs(): TestResult = runTest { }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for TestResult, got %d", len(findings))
	}
}

func TestTestWithOnlyTodo_SoleTodoOrNotImplementedOnly(t *testing.T) {
	findings := runRuleByName(t, "TestWithOnlyTodo", `
package test

import org.junit.Test

class SoleTest {
    @Test
    fun a() {
        TODO("later")
    }

    @Test
    fun b() {
        // placeholder
        throw NotImplementedError()
    }

    @Test
    fun c() = TODO()

    @Test
    fun d() {
        prepare()
        TODO()
    }

    @Test
    fun e() {
        fail(buildMessage())
    }
}
`)
	if len(findings) != 3 {
		t.Fatalf("expected findings for a, b, c only, got %d", len(findings))
	}
}

func TestTestWithOnlyTodo_ImplementedTestsWithFailOrLambdaAreSilent(t *testing.T) {
	findings := runRuleByName(t, "TestWithOnlyTodo", `
package test

import org.junit.Test

class ImplementedTest {
    @Test
    fun branch() {
        if (compute() > 1) fail("too big")
    }

    @Test
    fun whenBranch() {
        when (compute()) {
            1 -> error("one")
            else -> TODO()
        }
    }

    @Test
    fun lambdaTail() {
        runCase { fail("boom") }
    }

    @Test
    fun throwing() {
        throw IllegalStateException("x")
    }

    @Test
    fun todoInLambda() {
        run { TODO() }
    }

    @Test
    fun tryCatch() {
        try {
            compute()
        } catch (e: Exception) {
            fail("unexpected")
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestTestInheritanceDepth_IgnoresInterfacesAndStripsGenerics(t *testing.T) {
	findings := runRuleByName(t, "TestInheritanceDepth", `
package test

interface Marker
open class IfaceTest : Marker
class TwoTest : IfaceTest()
`)
	if len(findings) != 0 {
		t.Fatalf("interfaces must not add depth, got %d", len(findings))
	}
	findings = runRuleByName(t, "TestInheritanceDepth", `
package test

open class RootTest<T>
open class MidTest : RootTest<String>()
class LeafTest : MidTest()
`)
	if len(findings) != 1 {
		t.Fatalf("expected one finding through generic superclass, got %d", len(findings))
	}
}

func TestTestWithOnlyTodo_ClassLevelIgnoreIsSkipped(t *testing.T) {
	findings := runRuleByName(t, "TestWithOnlyTodo", `
package test

import org.junit.Ignore
import org.junit.Test

@Ignore("flaky")
class TodoInIgnoredClass {
    @Test
    fun loads() {
        TODO()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for a test in an @Ignore class, got %d", len(findings))
	}
}

func TestTestWithoutAssertion_TodoOnlyBodyIsLeftToTestWithOnlyTodo(t *testing.T) {
	findings := runRuleByName(t, "TestWithoutAssertion", `
package test

import org.junit.Test

class TodoOnly {
    @Test
    fun loads() {
        TODO()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings, got %d", len(findings))
	}
}

func TestRelaxedMockUsedForValueClass_SameFileValueClass(t *testing.T) {
	findings := runRuleByName(t, "RelaxedMockUsedForValueClass", `
package test

import io.mockk.mockk

@JvmInline
value class UserId(val raw: String)

class Plain(val raw: String)

class RelaxedValueTest {
    fun a() { val id = mockk<UserId>(relaxed = true) }
    fun b() { val p = mockk<Plain>(relaxed = true) }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for the value class only, got %d", len(findings))
	}
}

func TestTestNameContainsUnderscore_SkipsInstrumentedTests(t *testing.T) {
	src := `
package test

import org.junit.Test

class LoginTest {
    @Test
    fun login_succeeds() {
        assertTrue(true)
    }
}
`
	if got := runRuleByNameOnPath(t, "TestNameContainsUnderscore", "app/src/androidTest/kotlin/LoginTest.kt", src); len(got) != 0 {
		t.Fatalf("instrumented tests cannot use spaced names; got %d findings", len(got))
	}
	if got := runRuleByNameOnPath(t, "TestNameContainsUnderscore", "app/src/test/kotlin/LoginTest.kt", src); len(got) != 1 {
		t.Fatalf("JVM unit tests should still be flagged; got %d findings", len(got))
	}
}

func TestAssertTrueOnComparison_NullOperandSuggestsNullAssertions(t *testing.T) {
	cases := []struct {
		name, call, want string
	}{
		{"right null equals", "assertTrue(actual == null)", "Use `assertNull` instead of `assertTrue` for null comparisons."},
		{"left null equals", "assertTrue(null == actual)", "Use `assertNull` instead of `assertTrue` for null comparisons."},
		{"not equals null", "assertTrue(actual != null)", "Use `assertNotNull` instead of `assertTrue` for null comparisons."},
		{"qualified junit", "Assert.assertTrue(actual == null)", "Use `assertNull` instead of `assertTrue` for null comparisons."},
		{"plain equality", "assertTrue(actual == expected)", "Use `assertEquals` instead of `assertTrue` for equality comparisons."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			findings := runRuleByName(t, "AssertTrueOnComparison", "package test\n\nfun testCompute(actual: Int?, expected: Int?) {\n    "+tc.call+"\n}\n")
			if len(findings) != 1 {
				t.Fatalf("expected one finding, got %d: %v", len(findings), findings)
			}
			if findings[0].Message != tc.want {
				t.Errorf("message = %q, want %q", findings[0].Message, tc.want)
			}
		})
	}
}
