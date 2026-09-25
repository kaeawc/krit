// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: Flow.collect() wrapped in repeatOnLifecycle inside onCreate is
// lifecycle-aware — must NOT trigger CollectInOnCreateWithoutLifecycle.
package test

import android.os.Bundle
import androidx.fragment.app.Fragment
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.lifecycleScope
import androidx.lifecycle.repeatOnLifecycle
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.launch

class MyFragment : Fragment() {
    private val flow: Flow<Int> = TODO()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        lifecycleScope.launch {
            repeatOnLifecycle(Lifecycle.State.STARTED) {
                flow.collect { println(it) }
            }
        }
    }
}
