// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 14, 17, 21, 26, 33, 38, 43, 46, 49, 53, 57, 62
package test.runblocking.positive

import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import org.junit.Test

class BlockingPositiveTests {
    @Test
    fun expressionBody() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }

    @Test
    fun singleStatement() { <!RunBlockingInTest!>runBlocking<!> { assert(true) } }

    @Test
    fun returnedBody() { return <!RunBlockingInTest!>runBlocking<!> { assert(true) } }

    @Test
    fun nestedRunner() = runTest {
        <!RunBlockingInTest!>runBlocking<!> { assert(true) }
    }

    @Test
    fun unassertedResult() {
        val result = <!RunBlockingInTest!>runBlocking<!> { 1 }
        println(result)
    }

    @Test
    fun vagueComment() {
        // Use runBlocking for this coroutine.
        <!RunBlockingInTest!>runBlocking<!> { assert(true) }
    }

    @Test
    fun lambdaInTest() {
        listOf(1).forEach { <!RunBlockingInTest!>runBlocking<!> { assert(true) } }
    }

    @Test
    fun qualifiedMultiline() = kotlinx.coroutines
        .<!RunBlockingInTest!>runBlocking<!> { assert(true) }

    @org.junit.jupiter.api.Test
    fun junitFive() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }

    @org.junit.Test ()
    fun spacedAnnotationArguments() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }
}

@Test
fun topLevelBlockingTest() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }

object BlockingObjectTests {
    @Test
    fun objectTest() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }
}

class TestNGBlockingTests {
    @org.testng.annotations.Test
    fun testNgTest() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }
}
