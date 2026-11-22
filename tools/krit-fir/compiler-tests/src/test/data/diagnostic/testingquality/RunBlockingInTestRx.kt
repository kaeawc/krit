// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 36
package test.runblocking.rx

import io.reactivex.rxjava3.observers.*
import io.reactivex.rxjava3.schedulers.*
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import org.junit.Test

private fun subscribe(observer: TestObserver<String>) { assert(observer != null) }

class BlockingRxBridgeTests {
    @Test
    fun observerBridge() {
        runBlocking { assert(true) }
        val observer = TestObserver<String>()
        subscribe(observer)
    }

    @Test
    fun observerExpressionBody() = <!RunBlockingInTest!>runBlocking<!> {
        subscribe(TestObserver<String>())
    }

    @Test
    fun schedulerBridge() {
        val scheduler = TestScheduler()
        runBlocking { assert(true) }
        println(scheduler)
    }

    @Test
    suspend fun suspendSchedulerTest() {
        val scheduler = TestScheduler()
        <!RunBlockingInTest!>runBlocking<!> { assert(true) }
        println(scheduler)
    }
}
