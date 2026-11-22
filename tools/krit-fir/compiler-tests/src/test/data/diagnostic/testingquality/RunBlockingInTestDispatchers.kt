// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test.runblocking.dispatchers

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import org.junit.Test

class BlockingRealDispatcherTests {
    private val dispatcher = Dispatchers.Default

    @Test
    fun ownerHasRealDispatcher() = runBlocking { assert(true) }
}

object BlockingRealIoTests {
    private val dispatcher = Dispatchers.IO

    @Test
    fun ownerHasRealIo() = runBlocking { assert(true) }
}

@Test
fun dispatchNeededIntent() = runBlocking {
    Dispatchers.Unconfined.isDispatchNeeded(kotlin.coroutines.EmptyCoroutineContext)
}

@Test
fun realScopeIntent() = runBlocking {
    val scope = kotlinx.coroutines.CoroutineScope(kotlin.coroutines.EmptyCoroutineContext)
    val latch = java.util.concurrent.CountDownLatch(1)
    println(scope)
    println(latch)
}
