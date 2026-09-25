// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: Flow.collect() launched from onCreate() without repeatOnLifecycle — should trigger CollectInOnCreateWithoutLifecycle
package test

import android.os.Bundle
import androidx.fragment.app.Fragment
import androidx.lifecycle.lifecycleScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.launch

class MyFragment : Fragment() {
    private val flow: Flow<Int> = TODO()

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        lifecycleScope.launch {
            flow.<!CollectInOnCreateWithoutLifecycle!>collect<!> { <!PrintlnInProduction!>println<!>(it) }
        }
    }
}
