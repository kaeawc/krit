package rules_test

import "testing"

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
	if len(findings) == 0 {
		t.Fatal("expected finding for verify without mock")
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
