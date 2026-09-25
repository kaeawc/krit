// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: Flow.collect() in a regular method (not a lifecycle callback) is
// out of scope — must NOT trigger CollectInOnCreateWithoutLifecycle.
package test

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.collect

class Repository {
    private val flow: Flow<Int> = TODO()

    fun observe() {
        flow.collect { println(it) }
    }
}
