// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: Flow.collect() called from onStart(), not onCreate() — should NOT trigger
package test

import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.collect

open class Fragment {
    open fun onCreate() {}
    open fun onStart() {}
}

class MyFragment : Fragment() {
    private val flow: Flow<Int> = TODO()

    override fun onCreate() {
        super.onCreate()
        // no collect here
    }

    override fun onStart() {
        flow.collect { println(it) }
    }
}
