// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14
package test.runblocking.positive

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.test.runTest
import org.junit.Test

@Test
fun blocking() {
    class Fixture {
        val io = Dispatchers.IO
        init { <!RunBlockingInTest!>runBlocking<!> { assert(true) } }
    }
    Fixture()
}
