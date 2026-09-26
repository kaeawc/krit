// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives Go and FIR agree on: suspend calls outside finally blocks, and
// non-suspend calls inside them.
package test

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Deferred
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.channels.Channel
import kotlinx.coroutines.flow.MutableSharedFlow

suspend fun outsideFinally(job: Job) {
    try {
        delay(100)
        job.join()
    } catch (e: Exception) {
        delay(100)
    } finally {
        println("done")
    }
    delay(100)
}

fun nonSuspendCleanup(job: Job, channel: Channel<Int>, events: MutableSharedFlow<Int>, deferred: Deferred<Int>) {
    try {
        println("working")
    } finally {
        job.cancel()
        channel.close()
        channel.trySend(1)
        channel.tryReceive()
        events.tryEmit(1)
        deferred.getCompleted()
    }
}

// A qualified builder call on another scope: neither Go nor FIR reports it
// (Go only matches unqualified calls, and the target scope may be alive).
fun qualifiedBuilder(scope: CoroutineScope) {
    try {
        println("working")
    } finally {
        scope.launch { }
    }
}

class Timer {
    fun delay(ms: Long) {
        println(ms)
    }
}

// A member named like a suspend function, called with a receiver.
fun memberLookalike(timer: Timer) {
    try {
        println("working")
    } finally {
        timer.delay(5)
    }
}

// A suspend lambda defined in a finally block but not invoked there.
suspend fun storedLambda() {
    try {
        println("working")
    } finally {
        val later: suspend () -> Unit = { println("later") }
        println(later)
    }
}

// Iterating a channel suspends in the loop's compiler-generated hasNext();
// neither Go nor FIR reports it.
suspend fun drain(channel: Channel<Int>) {
    try {
        println("working")
    } finally {
        for (value in channel) println(value)
    }
}
