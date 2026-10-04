// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: awaits outside a finally block, or wrapped in runCatching inside
// it. Go agrees on every case in this file.
package test

import kotlinx.coroutines.Deferred
import kotlinx.coroutines.awaitAll

suspend fun outsideFinally(cleanup: Deferred<Unit>) {
    cleanup.await()
    try {
        cleanup.await()
    } catch (e: Exception) {
        cleanup.await()
    }
}

suspend fun wrapped(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        runCatching { cleanup.await() }
        val result = runCatching { listOf(cleanup).map { it.await() } }
        println(result)
        cleanup.runCatching { await() }
        kotlin.runCatching { cleanup.await() }
    }
}

suspend fun notAwait(all: List<Deferred<Unit>>) {
    try {
        println("working")
    } finally {
        all.awaitAll()
        val reference = all.first()::await
        println(reference)
        all.first().cancel()
    }
}

suspend fun afterFinally(cleanup: Deferred<Unit>) {
    try {
        println("working")
    } finally {
        println("done")
    }
    cleanup.await()
}
