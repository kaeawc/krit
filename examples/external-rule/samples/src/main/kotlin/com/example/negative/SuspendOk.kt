package com.example.negative

suspend fun doSuspendWork() {
    // Body intentionally empty.
}

fun runSuspendBlock(block: suspend () -> Unit) {
    // We never invoke `block` here; we only need the parameter type
    // signature so the analyzer reports the passed lambda as
    // `suspend`. Non-`inline` so the lambda's functional type is the
    // declared parameter type (inlined lambdas adopt the caller's
    // suspend context instead).
    @Suppress("UNUSED_VARIABLE")
    val captured = block
}

class SuspendOk {
    fun runIt() {
        // Suspend call inside a `suspend () -> Unit` lambda — the
        // resolver should report the lambda as `suspend`, so the rule
        // must skip this call.
        runSuspendBlock {
            doSuspendWork()
        }
    }

    // Suspend call inside a `suspend` named function — also fine.
    suspend fun runAlso() {
        doSuspendWork()
    }
}
