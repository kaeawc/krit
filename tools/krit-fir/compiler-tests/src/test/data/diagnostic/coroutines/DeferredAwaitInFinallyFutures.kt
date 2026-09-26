// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 30, 39, 47
// Positive: the kotlinx.coroutines await extensions on futures (future, tasks,
// guava) inside a finally block. Each rethrows the future's failure, so like
// Deferred.await() it can replace the try block's exception.
package test

import com.google.android.gms.tasks.Task
import com.google.common.util.concurrent.ListenableFuture
import java.util.concurrent.CompletableFuture
import java.util.concurrent.CompletionStage
import kotlinx.coroutines.future.await
import kotlinx.coroutines.guava.await
import kotlinx.coroutines.tasks.await
import kotlinx.coroutines.future.await as awaitStage

suspend fun completionStage(stage: CompletionStage<Unit>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>stage.await()<!>
    }
}

suspend fun completableFuture(future: CompletableFuture<Int>): Int {
    var value = 0
    try {
        println("working")
    } finally {
        value = <!DeferredAwaitInFinally!>future.await()<!>
    }
    return value
}

suspend fun listenableFuture(future: ListenableFuture<String>?) {
    try {
        println("working")
    } finally {
        future?.<!DeferredAwaitInFinally!>await()<!>
    }
}

suspend fun playServicesTask(task: Task<String>) {
    try {
        println("working")
    } finally {
        println(<!DeferredAwaitInFinally!>task.await()<!>)
    }
}

// Go misses these because the call has no `.await` navigation: an import
// alias renames the call, and an implicit receiver drops the navigation. FIR
// reports them because the resolved callee is kotlinx.coroutines.future.await.
suspend fun aliasedAndImplicit(stage: CompletionStage<Unit>) {
    try {
        println("working")
    } finally {
        <!DeferredAwaitInFinally!>stage.awaitStage()<!>
        with(stage) { <!DeferredAwaitInFinally!>await()<!> }
    }
}

// Negative: wrapped in runCatching, or outside the finally block.
suspend fun guarded(stage: CompletionStage<Unit>, task: Task<Unit>, future: ListenableFuture<Unit>) {
    stage.await()
    try {
        task.await()
    } finally {
        runCatching { stage.await() }
        runCatching { task.await() }
        runCatching { future.await() }
    }
}
