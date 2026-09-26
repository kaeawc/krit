// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 27, 42, 46, 51, 57, 71
// A receiver typed only CoroutineScope that holds kotlinx.coroutines.GlobalScope:
// a val property or local initialized with it, directly or through another
// such val. Each launch runs on the global scope, so FIR reports it. Go
// reports the receivers spelled GlobalScope (lines 26 and 27) and misses
// `scope`, `chained`, `local`, and `maybe` (lines 28, 29, 31, and 32): it
// matches only a receiver spelled GlobalScope.
//
// A receiver spelled GlobalScope whose value cannot be known (a parameter, a
// constructor property, a var, a property with a getter) is a judgment call.
// FIR matches Go and reports it by its name. A parenthesized one is not a
// receiver Go reads, so neither reports it.
package test.holder

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.async
import kotlinx.coroutines.launch

class HolderViewModel {
    private val GlobalScope: CoroutineScope = kotlinx.coroutines.GlobalScope
    private val scope: CoroutineScope = kotlinx.coroutines.GlobalScope
    private val chained: CoroutineScope = scope

    fun a() {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
        <!GlobalScopeLaunchInViewModel!>this<!>.GlobalScope.async { 1 }
        <!GlobalScopeLaunchInViewModel!>scope<!>.launch { }
        <!GlobalScopeLaunchInViewModel!>chained<!>.launch { }
        val local: CoroutineScope = kotlinx.coroutines.GlobalScope
        <!GlobalScopeLaunchInViewModel!>local<!>.launch { }
        <!GlobalScopeLaunchInViewModel!>maybe<!>?.launch { }
    }

    private val maybe: CoroutineScope? = kotlinx.coroutines.GlobalScope
}

class ParamViewModel(private val GlobalScope: CoroutineScope) {
    private val getterScope: CoroutineScope get() = GlobalScope

    fun b() {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
    }

    fun e(GlobalScope: CoroutineScope) {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
        (GlobalScope).launch { }
    }

    fun n(GlobalScope: CoroutineScope?) {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>?.launch { }
    }

    fun f(other: CoroutineScope) {
        var GlobalScope: CoroutineScope = kotlinx.coroutines.GlobalScope
        GlobalScope = other
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
    }

    fun g(scope: CoroutineScope) {
        scope.launch { }
        getterScope.launch { }
    }
}

class GetterPresenter {
    private val GlobalScope: CoroutineScope
        get() = kotlinx.coroutines.GlobalScope

    fun h() {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.async { 1 }
    }
}
