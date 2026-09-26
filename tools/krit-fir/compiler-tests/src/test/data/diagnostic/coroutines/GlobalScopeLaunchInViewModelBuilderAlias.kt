// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go misses this: kotlinx.coroutines.launch is imported as `fire`, and Go
// matches the call's spelled name. FIR reports it under the builder's real
// name, because it launches a coroutine on the global scope.
package test.builderalias

import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.launch as fire

class UserViewModel {
    fun load() {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.fire { }
    }
}
