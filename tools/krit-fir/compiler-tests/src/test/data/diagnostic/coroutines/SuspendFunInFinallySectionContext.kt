// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 29x2, 30, 32x2, 33, 36x2, 37, 51x2, 52, 67x2, 68, 70x2, 71, 73x2, 74, 87x2, 91x2, 101, 111, 126, 134, 141, 155x2, 156, 158x2, 159
// The coroutine context a builder is given decides whether its block runs
// after cancellation. CoroutineContext.plus is right-biased: an element on the
// right replaces the one on the left with the same key, so in
// `NonCancellable + ctx` the Job is ctx's, not NonCancellable.
package test

import kotlin.coroutines.AbstractCoroutineContextElement
import kotlin.coroutines.CoroutineContext
import kotlinx.coroutines.CoroutineName
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import kotlinx.coroutines.withContext

// A Job-bearing context to the right of NonCancellable replaces it: withContext
// throws on entry once that Job is cancelled, and its block never runs. Go and
// FIR both report withContext / launch and the delay inside (Go twice on the
// builder line, see trailingLambdas in SuspendFunInFinallySectionDivergence.kt).
suspend fun rightHandJob(scope: CoroutineScope, job: Job, ctx: CoroutineContext) {
    try {
        println("working")
    } finally {
        <!SuspendFunInFinallySection!>withContext<!>(NonCancellable + ctx) {
            <!SuspendFunInFinallySection!>delay(1)<!>
        }
        <!SuspendFunInFinallySection!>withContext<!>(NonCancellable + job) {
            <!SuspendFunInFinallySection!>delay(2)<!>
        }
        with(scope) {
            <!SuspendFunInFinallySection!>launch<!>(NonCancellable + job) {
                <!SuspendFunInFinallySection!>delay(3)<!>
            }
        }
    }
}

// Job-free elements (a dispatcher, a name) on the right keep NonCancellable as
// the Job. Go reports withContext twice and the delay; FIR drops them, as for
// withContext(NonCancellable) (nonCancellable in
// SuspendFunInFinallySectionDivergence.kt).
suspend fun jobFreeRight() {
    try {
        println("working")
    } finally {
        withContext(NonCancellable + Dispatchers.IO + CoroutineName("cleanup")) {
            delay(1)
        }
    }
}

// runBlocking is not a suspend function, so neither form reports it (Go does,
// by the name). Given the caller's context, its coroutine is a child of the
// caller's Job and is cancelled with it: Go and FIR both report the delay.
// Without a Job-bearing context it is a fresh coroutine the caller's
// cancellation does not reach, so its block runs: Go reports the delay, FIR
// drops it.
suspend fun blockingWithContext(ctx: CoroutineContext) {
    try {
        println("working")
    } finally {
        runBlocking(ctx) {
            <!SuspendFunInFinallySection!>delay(3)<!>
        }
        runBlocking(Dispatchers.IO) {
            delay(4)
        }
        runBlocking(NonCancellable) {
            delay(5)
        }
    }
}

// A try inside withContext(NonCancellable) runs its finally block under
// NonCancellable, so a suspend call there is not skipped. Go reports the delay
// (twice in the first function: once per enclosing finally); FIR drops it,
// also through a Job-free withContext and an inline lambda in between.
suspend fun nestedInNonCancellable() {
    try {
        println("outer")
    } finally {
        withContext(NonCancellable) {
            try {
                println("inner")
            } finally {
                delay(5)
            }
        }
    }
}

suspend fun wholeBodyNonCancellable() = withContext(NonCancellable) {
    try {
        println("working")
    } finally {
        delay(6)
    }
}

suspend fun throughJobFree() = withContext(NonCancellable) {
    withContext(Dispatchers.IO) {
        run {
            try {
                println("working")
            } finally {
                delay(7)
            }
        }
    }
}

// Not under NonCancellable: a launched child has its own cancellable Job, and
// withContext(ctx + NonCancellable) is still NonCancellable only when nothing
// Job-bearing follows it. Go and FIR both report each delay.
suspend fun notUnderNonCancellable(scope: CoroutineScope, ctx: CoroutineContext) {
    withContext(NonCancellable) {
        scope.launch {
            try {
                println("working")
            } finally {
                <!SuspendFunInFinallySection!>delay(8)<!>
            }
        }
    }
    withContext(NonCancellable + ctx) {
        try {
            println("working")
        } finally {
            <!SuspendFunInFinallySection!>delay(9)<!>
        }
    }
    withContext(Dispatchers.IO) {
        try {
            println("working")
        } finally {
            <!SuspendFunInFinallySection!>delay(10)<!>
        }
    }
}

// A context element FIR cannot prove Job-free (an anonymous element keyed on
// Job, a local element class) is treated as Job-bearing, and its type is
// compared without resolving the local or anonymous class. Go and FIR both
// report withContext and the delay (Go twice on the builder line).
suspend fun localElements() {
    class Tagged : AbstractCoroutineContextElement(CoroutineName)
    try {
        println("working")
    } finally {
        <!SuspendFunInFinallySection!>withContext<!>(object : AbstractCoroutineContextElement(Job) {}) {
            <!SuspendFunInFinallySection!>delay(11)<!>
        }
        <!SuspendFunInFinallySection!>withContext<!>(NonCancellable + Tagged()) {
            <!SuspendFunInFinallySection!>delay(12)<!>
        }
    }
}
