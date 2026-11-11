// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 19
package test

import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.withContext

suspend fun loadData() {
    withContext(Dispatchers.IO) {
        <!WithContextInSuspendFunctionNoop!>withContext<!>(Dispatchers.IO) {
            fetch()
        }
    }
}

suspend fun loadDeep() {
    withContext(Dispatchers.Default) {
        println("before")
        val x = <!WithContextInSuspendFunctionNoop!>withContext<!>(Dispatchers.Default) {
            fetch()
        }
    }
}

suspend fun fetch() {}
