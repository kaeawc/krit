// Smoke: ListenableFuture is a java.util.concurrent.Future with an executor-bound listener.
package stubs

import com.google.common.util.concurrent.ListenableFuture
import java.util.concurrent.Executor
import java.util.concurrent.Future
import java.util.concurrent.TimeUnit

fun collectWhenDone(future: ListenableFuture<String>, executor: Executor, sink: MutableList<String>) {
    val plain: Future<String> = future
    future.addListener({ if (plain.isDone && !plain.isCancelled) sink.add(future.get(1, TimeUnit.SECONDS)) }, executor)
}
