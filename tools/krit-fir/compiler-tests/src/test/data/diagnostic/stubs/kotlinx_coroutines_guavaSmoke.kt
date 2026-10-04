// Smoke: suspending on a Guava ListenableFuture with kotlinx.coroutines.guava.await.
package stubs

import com.google.common.util.concurrent.ListenableFuture
import kotlinx.coroutines.guava.await

suspend fun awaitListenableFutures(name: ListenableFuture<String>, size: ListenableFuture<Int>): Boolean {
    val value: String = name.await()
    return value.length == size.await()
}
