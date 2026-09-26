// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 33
// A package that only shares the kotlinx.coroutines prefix
// (kotlinx.coroutinesextra) is not kotlinx.coroutines.
package kotlinx.coroutinesextra

import java.util.concurrent.CompletionStage

suspend fun <T> CompletionStage<T>.await(): T = TODO()

class Barrier {
    suspend fun await() {}
}

// Go reports this and so does FIR: a suspend await extension on a future
// rethrows the future's failure like kotlinx.coroutines.future.await, in any
// package.
suspend fun prefixExtension(stage: CompletionStage<Unit>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>stage.await()<!>
    }
}

// Go reports this because the call is written `.await()`; FIR is correct to
// drop it: Barrier is not a Deferred, and its package is not kotlinx.coroutines
// or a subpackage, so its member await is a lookalike.
suspend fun prefixMember(barrier: Barrier) {
    try {
        println("working")
    } finally {
        barrier.await()
    }
}
