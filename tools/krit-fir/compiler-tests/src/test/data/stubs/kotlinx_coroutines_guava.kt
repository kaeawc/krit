// Compiler-test source stubs; never packaged in the production artifact.
package kotlinx.coroutines.guava

import com.google.common.util.concurrent.ListenableFuture

suspend fun <T> ListenableFuture<T>.await(): T = TODO()
