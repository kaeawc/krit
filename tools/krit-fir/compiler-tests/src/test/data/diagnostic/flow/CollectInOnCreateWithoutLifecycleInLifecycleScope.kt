// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26
// Positive: Flow.collect() in lifecycleScope.launch from onStart() without
// repeatOnLifecycle keeps the upstream active past the STOPPED state, the same
// leak as in onCreate. The rule covers onCreate/onStart/onViewCreated — should
// trigger.
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
        // no collect here
    }

    override fun onStart() {
        super.onStart()
        lifecycleScope.launch {
            <!CollectInOnCreateWithoutLifecycle!>flow.collect<!> { <!PrintlnInProduction!>println<!>(it) }
        }
    }
}
