// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

object Dispatchers2 {
    val IO = Dispatchers.IO
}

class Wrapper {
    suspend fun <T> withContext(dispatcher: Any, block: suspend () -> T): T = block()

    suspend fun member() {
        withContext(Dispatchers.IO) {
            withContext(Dispatchers.IO) { 1 }
        }
    }
}

suspend fun otherHolder() {
    withContext(Dispatchers.IO) {
        withContext(Dispatchers2.IO) { 1 }
    }
}
