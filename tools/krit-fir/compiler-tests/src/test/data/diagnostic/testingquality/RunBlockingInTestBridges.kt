// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 20
package test.runblocking.bridges

import kotlinx.coroutines.runBlocking
import org.junit.Test

class BlockingBridgeTests {
    @Test
    fun largerLegacyTest() {
        println("setup")
        runBlocking { assert(true) }
        println("teardown")
    }

    @Test
    fun wholeLegacyTest() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }

    @Test
    fun returnedLegacyTest() { return <!RunBlockingInTest!>runBlocking<!> { assert(true) } }

    @Test
    fun anonymousLegacyBridge() {
        val callback = fun() { runBlocking { assert(true) } }
        callback()
    }
}
