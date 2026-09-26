// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20
// Go misses this: kotlinx.coroutines.launch is imported as `fire`, and Go
// matches the call's spelled name. FIR reports it under the builder's real
// name, because it launches a coroutine on the global scope.
//
// Message divergence on line 20: kotlinx.coroutines.async is imported as
// `launch`. Both report the line, but Go names the spelled builder
// ("GlobalScope.launch in UserViewModel") and FIR the resolved one
// ("GlobalScope.async in UserViewModel"), which is the call that runs.
package test.builderalias

import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.async as launch
import kotlinx.coroutines.launch as fire

class UserViewModel {
    fun load() {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.fire { }
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { 1 }
    }
}
