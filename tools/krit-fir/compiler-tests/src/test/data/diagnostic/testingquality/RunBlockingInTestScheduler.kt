// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 25
package test.runblocking.scheduler

import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.TestCoroutineScheduler
import org.junit.Test

class BlockingSchedulerTests {
    private val testScheduler = TestCoroutineScheduler()

    @Test
    fun manuallyDriven() {
        runBlocking { assert(true) }
        testScheduler.advanceUntilIdle()
    }

    @Test
    suspend fun suspendTest() {
        <!RunBlockingInTest!>runBlocking<!> { assert(true) }
        testScheduler.advanceUntilIdle()
    }

    @Test
    fun wholeBodyStillReported() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }
}
