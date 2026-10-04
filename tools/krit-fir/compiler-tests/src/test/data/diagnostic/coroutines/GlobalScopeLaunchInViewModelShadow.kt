// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 31, 33
// Divergence: a property named GlobalScope that holds a class-owned scope
// shadows kotlinx.coroutines.GlobalScope. The launch runs on that scope, not
// on the global one. Go reports it by the receiver's name; FIR is correct to
// drop it. A property initialized with GlobalScope itself is still reported
// (see GlobalScopeLaunchInViewModelHolder.kt).
package test.shadow

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.GlobalScope as Global
import kotlinx.coroutines.Job
import kotlinx.coroutines.MainScope
import kotlinx.coroutines.launch
import kotlinx.coroutines.plus

class UserViewModel {
    private val GlobalScope: CoroutineScope = CoroutineScope(Job())

    fun load() {
        GlobalScope.launch { }
    }
}

// The same for a new MainScope() and for GlobalScope + Job(), which is a new
// scope with its own Job, not the global scope.
class MainViewModel {
    private val GlobalScope: CoroutineScope = MainScope()

    fun load() {
        GlobalScope.launch { }
        val GlobalScope = Global + Job()
        GlobalScope.launch { }
    }
}
