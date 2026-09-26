// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go misses these: the receiver is kotlinx.coroutines.GlobalScope under
// another spelling (an import alias, a type alias, a property or local that
// holds it), and Go only matches a receiver spelled GlobalScope. FIR reports
// them, because each launches a coroutine on the global scope.
package test.alias

import kotlinx.coroutines.GlobalScope as AppScope
import kotlinx.coroutines.async
import kotlinx.coroutines.launch

typealias Background = kotlinx.coroutines.GlobalScope

class UserViewModel {
    private val scope = AppScope

    fun load() {
        <!GlobalScopeLaunchInViewModel!>AppScope<!>.launch { }
        <!GlobalScopeLaunchInViewModel!>Background<!>.launch { }
        <!GlobalScopeLaunchInViewModel!>scope<!>.async { }
        val local = AppScope
        <!GlobalScopeLaunchInViewModel!>local<!>.launch { }
    }
}
