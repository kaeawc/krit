// Compiler-test source stubs; never packaged in the production artifact.
package kotlinx.coroutines.future

import java.util.concurrent.CompletionStage

suspend fun <T> CompletionStage<T>.await(): T = TODO()
