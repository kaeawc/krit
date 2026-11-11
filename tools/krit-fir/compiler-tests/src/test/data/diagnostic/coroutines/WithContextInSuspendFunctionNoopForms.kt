// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import kotlinx.coroutines.Dispatchers as D
import kotlinx.coroutines.withContext

// Go misses the aliased import (it needs `import kotlinx.coroutines.Dispatchers`), the fully
// qualified spelling and the named `context =` argument; FIR resolves all three to kotlinx.coroutines.
suspend fun aliased() {
    withContext(D.IO) {
        <!WithContextInSuspendFunctionNoop!>withContext<!>(D.IO) { 1 }
    }
}

suspend fun qualified() {
    kotlinx.coroutines.withContext(kotlinx.coroutines.Dispatchers.Default) {
        <!WithContextInSuspendFunctionNoop!>kotlinx<!>.coroutines.withContext(kotlinx.coroutines.Dispatchers.Default) { 1 }
    }
}

suspend fun named() {
    withContext(context = D.IO) {
        <!WithContextInSuspendFunctionNoop!>withContext<!>(context = D.IO) { 1 }
    }
}

// An inline stdlib lambda between the two calls: the dispatcher is unchanged, but
// the rule only looks through withContext lambdas.
suspend fun throughRun() {
    withContext(D.IO) {
        run {
            withContext(D.IO) { 1 }
        }
    }
}
