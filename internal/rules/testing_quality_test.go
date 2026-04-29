package rules_test

import (
	"strings"
	"testing"
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

func TestAssertNullableWithNotNullAssertion_UsesLocalASTOnly(t *testing.T) {
	rule := buildRuleIndex()["AssertNullableWithNotNullAssertion"]
	if rule == nil {
		t.Fatal("AssertNullableWithNotNullAssertion rule is not registered")
	}
	if rule.Needs != 0 {
		t.Fatalf("AssertNullableWithNotNullAssertion should remain AST-only, got needs %v", rule.Needs)
	}
	if rule.OracleCallTargets != nil || rule.OracleDeclarationNeeds != nil || rule.Oracle != nil {
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
        val repo = Repo(api)
        repo.load()
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for mock without verify")
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
	if rule.OracleCallTargets != nil || rule.OracleDeclarationNeeds != nil || rule.Oracle != nil {
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
import org.thoughtcrime.securesms.testing.assertIsSize

class TestWithoutAssertionInfixNegative {
    @Test
    fun loads() {
        val values = listOf(1, 2)
        values assertIsSize 2
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

func TestTestWithoutAssertion_UsesLocalASTOnly(t *testing.T) {
	rule := buildRuleIndex()["TestWithoutAssertion"]
	if rule == nil {
		t.Fatal("TestWithoutAssertion rule is not registered")
	}
	if rule.Needs != 0 {
		t.Fatalf("TestWithoutAssertion should remain AST-only, got needs %v", rule.Needs)
	}
	if rule.OracleCallTargets != nil || rule.OracleDeclarationNeeds != nil || rule.Oracle != nil {
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

class VerifyWithoutMockSignalStyleNegative {
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
