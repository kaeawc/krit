// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 34, 42, 43, 45, 46, 47, 49, 50, 51, 61, 62, 64, 65, 74, 75, 76, 84, 93, 103, 114, 122, 124, 126, 130
// Positives Go and FIR agree on: an unqualified call of a well-known suspend
// function or coroutine builder inside a finally block, including inside
// lambdas, local functions and object members declared there.
package test

import kotlin.coroutines.CoroutineContext
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.awaitAll
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.delay
import kotlinx.coroutines.joinAll
import kotlinx.coroutines.launch
import kotlinx.coroutines.supervisorScope
import kotlinx.coroutines.withContext
import kotlinx.coroutines.withTimeout
import kotlinx.coroutines.withTimeoutOrNull
import kotlinx.coroutines.channels.ReceiveChannel
import kotlinx.coroutines.channels.SendChannel
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.collect
import kotlinx.coroutines.flow.flow

suspend fun delayInFinally() {
    try {
        println("working")
    } finally {
        <!SuspendFunInFinallySection!>delay(100)<!>
    }
}

suspend fun builders() {
    try {
        println("working")
    } finally {
        <!SuspendFunInFinallySection!>withContext<!>(Dispatchers.IO, {
            <!SuspendFunInFinallySection!>delay(1)<!>
        })
        <!SuspendFunInFinallySection!>coroutineScope<!> {
            <!SuspendFunInFinallySection!>launch { }<!>
            <!SuspendFunInFinallySection!>async { 1 }<!>
        }
        <!SuspendFunInFinallySection!>supervisorScope { }<!>
        <!SuspendFunInFinallySection!>withTimeout(10, { })<!>
        <!SuspendFunInFinallySection!>withTimeoutOrNull(10, { })<!>
    }
}

suspend fun waits(a: Job, b: Job, d: Deferred<Int>) {
    try {
        println("working")
    } catch (e: Exception) {
        println(e)
    } finally {
        <!SuspendFunInFinallySection!>awaitAll(d)<!>
        <!SuspendFunInFinallySection!>joinAll(a, b)<!>
        with(d) {
            <!SuspendFunInFinallySection!>await()<!>
            <!SuspendFunInFinallySection!>join()<!>
        }
    }
}

suspend fun channels(inbox: ReceiveChannel<Int>, outbox: SendChannel<Int>, source: Flow<Int>) {
    try {
        println("working")
    } finally {
        with(outbox) { <!SuspendFunInFinallySection!>send(1)<!> }
        with(inbox) { <!SuspendFunInFinallySection!>receive()<!> }
        with(source) { <!SuspendFunInFinallySection!>collect()<!> }
    }
}

fun flowCleanup(): Flow<Int> = flow {
    try {
        emit(1)
    } finally {
        <!SuspendFunInFinallySection!>emit(2)<!>
    }
}

// yield of a sequence builder is a restricted suspend function.
val numbers = sequence {
    try {
        yield(1)
    } finally {
        <!SuspendFunInFinallySection!>yield(2)<!>
    }
}

// A non-suspend function can still start coroutines from its finally block.
fun blockingOwner() {
    try {
        println("working")
    } finally {
        GlobalScope.launch {
            <!SuspendFunInFinallySection!>delay(1)<!>
        }
        runCatching { println("done") }
    }
}

class Worker(override val coroutineContext: CoroutineContext) : CoroutineScope {
    fun stop() {
        try {
            println("stopping")
        } finally {
            <!SuspendFunInFinallySection!>launch { }<!>
        }
    }

    suspend fun nested() {
        try {
            println("working")
        } finally {
            runCatching { <!SuspendFunInFinallySection!>delay(1)<!> }
            try {
                <!SuspendFunInFinallySection!>delay(2)<!>
            } catch (e: Exception) {
                <!SuspendFunInFinallySection!>delay(3)<!>
            }
            val cleanup = object {
                suspend fun run() {
                    <!SuspendFunInFinallySection!>delay(5)<!>
                }
            }
            println(cleanup)
        }
    }
}
