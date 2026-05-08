// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: Flow.collect() called bare inside onCreate() — should trigger FLOW_COLLECT_IN_ON_CREATE
package test

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.collect

open class Fragment { open fun onCreate() {} }

class MyFragment : Fragment() {
    private val flow: Flow<Int> = TODO()

    override fun onCreate() {
        super.onCreate()
        flow.<!FLOW_COLLECT_IN_ON_CREATE!>collect<!> { println(it) }
    }
}
