// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test.testwithoutassertion.negative

import com.google.common.truth.Truth.assertThat
import io.mockk.coVerify
import io.mockk.verify
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import org.junit.Assert.assertEquals
import org.junit.Ignore
import org.junit.Test
import org.junit.jupiter.api.Assertions

class WithAssertions {
    @Test
    fun directAssertion() { assertEquals(1, 1) }

    @org.junit.jupiter.api.Test
    fun jupiterAssertion() { Assertions.assertEquals(1, 1) }

    @Test
    fun truthAssertion() { assertThat(1).isEqualTo(1) }

    @Test
    fun mockVerification() { verify { println("checked") } }

    @Test
    fun suspendMockVerification() = runTest { coVerify { println("checked") } }

    @Test
    fun coroutineAssertion() = runTest { assertEquals(1, 1) }

    @Test
    fun delegatedAssertion() { firstHop() }

    @Test
    fun throwingCast() {
        val value: Any = "value"
        value as String
    }

    @Test
    fun coroutineCast() = runBlocking {
        val value: Any = "value"
        value as String
    }

    @Test
    fun trailingNotNull() {
        val value: String? = "value"
        value!!
    }

    @Test
    fun conditionCheck() { require(true) }

    @Test
    fun plantedFailure() { if (false) throw IllegalStateException("unexpected") }

    @Test(expected = IllegalArgumentException::class)
    fun expectedException() { println("work") }

    @Test(expected = Test.None::class)
    fun explicitNoExceptionWithAssertion() { assertEquals(1, 1) }

    @Test
    fun emptyFixture() {}

    @Test
    fun unitFixture() = Unit

    @Test
    fun placeholder() { TODO("later") }

    @Test
    fun unimplementedPlaceholder() { throw NotImplementedError("later") }

    @Ignore("manual")
    @Test
    fun ignored() { println("work") }

    @org.junit.jupiter.api.Disabled
    @org.junit.jupiter.api.Test
    fun disabled() { println("work") }

    @Test
    fun renderSmoke() { println("work") }

    @Test
    fun resetIsNoOp() { println("work") }

    @Test
    fun `accepts input without throwing`() { println("work") }

    @Test
    fun documentedJourney() {
        // The operation should not crash.
        println("work")
    }

    private fun firstHop() { secondHop() }
    private fun secondHop() { assertEquals(1, 1) }
}

@Ignore("manual")
class IgnoredOwner {
    @Test
    fun loads() { println("work") }
}

class Constructed(val name: String)

class ConstructionChecks {
    @Test
    fun rejectsInvalidInput() {
        Assertions.assertThrows(IllegalArgumentException::class.java) { Constructed("") }
    }

    @Test
    fun createsValidInstance() { Constructed("valid") }
}

class PlantedCallbacks {
    @Test
    fun observerFailure() {
        val observer = object {
            fun onValue() { throw IllegalStateException("unexpected value") }
        }
        println(observer)
    }

    @Test
    fun lazyFailure() {
        val callback = { error("unexpected callback") }
        println(callback)
    }

    @Test
    fun invokedAssertion() {
        val callback = { assertEquals(1, 1) }
        callback()
    }

    @org.junit.jupiter.api.DisplayName("does not throw")
    @org.junit.jupiter.api.Test
    fun namedExpectation() { println("work") }
}
