// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: Flow.collect() in onStart() without repeatOnLifecycle keeps the
// upstream active past the STOPPED state, the same leak as in onCreate. The
// rule covers onCreate/onStart/onViewCreated — should trigger.
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
        <!CollectInOnCreateWithoutLifecycle!>flow.collect { println(it) }<!>
    }
}
