// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 29
// Scopes declared inside a finally block. Go walks every call in the finally
// subtree, so a suspend call in a local class member or in a suspend lambda
// stored there is reported by both. A safe call (`job?.join()`) is qualified,
// so Go misses it; FIR reports it because join is a suspend function.
package test

import kotlinx.coroutines.Job
import kotlinx.coroutines.delay

suspend fun localClassInFinally() {
    try {
        println("working")
    } finally {
        class Cleanup {
            suspend fun run() {
                <!SuspendFunInFinallySection!>delay(1)<!>
            }
        }
        println(Cleanup())
    }
}

suspend fun storedSuspendLambda() {
    try {
        println("working")
    } finally {
        val later: suspend () -> Unit = { <!SuspendFunInFinallySection!>delay(2)<!> }
        println(later)
    }
}

suspend fun safeCall(job: Job?) {
    try {
        println("working")
    } finally {
        job?.<!SuspendFunInFinallySection!>join()<!>
    }
}
