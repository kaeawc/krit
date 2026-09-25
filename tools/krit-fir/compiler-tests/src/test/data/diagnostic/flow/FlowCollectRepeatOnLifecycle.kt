// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: Flow.collect() wrapped in repeatOnLifecycle inside onCreate is
// lifecycle-aware — must NOT trigger CollectInOnCreateWithoutLifecycle.
package test

import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.collect

open class Fragment { open fun onCreate() {} }

class MyFragment : Fragment() {
    private val flow: Flow<Int> = TODO()

    override fun onCreate() {
        super.onCreate()
        repeatOnLifecycle {
            flow.collect { println(it) }
        }
    }
}
