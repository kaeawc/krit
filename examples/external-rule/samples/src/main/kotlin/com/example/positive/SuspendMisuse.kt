package com.example.positive

// Standalone, dependency-free reproduction: a local `suspend` function
// called from a non-suspend lambda body. The example rule's resolver
// queries (`isSuspendCall`, `isLambdaSuspend`) work off the project's
// own sources, so we don't need `kotlinx.coroutines` on the analyzer
// classpath to drive the type-aware path.
suspend fun doSuspendWork() {
    // Body intentionally empty — only the modifier matters.
}

inline fun runRegularBlock(block: () -> Unit) {
    block()
}

class SuspendMisuse {
    fun runIt() {
        runRegularBlock {
            // Suspend call inside a non-suspend `() -> Unit` lambda —
            // the rule must flag this line.
            doSuspendWork()
        }
    }
}
