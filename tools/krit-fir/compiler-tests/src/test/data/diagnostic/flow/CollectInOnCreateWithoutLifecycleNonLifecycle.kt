// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: Flow.collect() in a regular method (not a lifecycle callback) is
// out of scope — must NOT trigger CollectInOnCreateWithoutLifecycle.
package test

import kotlinx.coroutines.flow.Flow

class Repository {
    private val flow: Flow<Int> = TODO()

    suspend fun observe() {
        flow.collect { <!PrintlnInProduction!>println<!>(it) }
    }
}
