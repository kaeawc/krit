// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 21
package test.runblocking.positive

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.runBlocking
import org.junit.Test

class OuterDispatcherTests {
    private val dispatcher = Dispatchers.IO

    @Deprecated("legacy")
    object AnnotatedObjectTests {
        @Test
        fun blocking() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }
    }

    /** Retains its own owner boundary. */
    object DocumentedObjectTests {
        @Test
        fun blocking() = <!RunBlockingInTest!>runBlocking<!> { assert(true) }
    }
}
