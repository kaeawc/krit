// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: Flow.collect() called bare inside onCreate() — should trigger CollectInOnCreateWithoutLifecycle
package test

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.collect

open class Fragment { open fun onCreate() {} }

class MyFragment : Fragment() {
    private val flow: Flow<Int> = TODO()

    override fun onCreate() {
        super.onCreate()
        flow.<!CollectInOnCreateWithoutLifecycle!>collect<!> { println(it) }
    }
}
