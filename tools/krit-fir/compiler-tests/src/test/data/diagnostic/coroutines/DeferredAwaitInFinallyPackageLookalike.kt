// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
// Divergence: an await extension declared in a package that only shares the
// kotlinx.coroutines prefix (kotlinx.coroutinesextra) is not a kotlinx.coroutines
// await. Go reports it because the call is written `.await()`; FIR drops it.
package kotlinx.coroutinesextra

import java.util.concurrent.CompletionStage

suspend fun <T> CompletionStage<T>.await(): T = TODO()

suspend fun prefixLookalike(stage: CompletionStage<Unit>) {
    try {
        println("working")
    } finally {
        stage.await()
    }
}
