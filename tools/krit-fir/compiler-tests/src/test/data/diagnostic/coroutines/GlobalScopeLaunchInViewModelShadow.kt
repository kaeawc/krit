// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17
// Divergence: a property named GlobalScope that holds a class-owned scope
// shadows kotlinx.coroutines.GlobalScope. The launch runs on that scope, not
// on the global one. Go reports it by the receiver's name; FIR is correct to
// drop it.
package test.shadow

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.launch

class UserViewModel {
    private val GlobalScope: CoroutineScope = CoroutineScope(Job())

    fun load() {
        GlobalScope.launch { }
    }
}
