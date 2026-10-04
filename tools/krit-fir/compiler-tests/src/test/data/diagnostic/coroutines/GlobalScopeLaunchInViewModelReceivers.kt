// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go misses all of these, and FIR reports them because each launches a
// coroutine on kotlinx.coroutines.GlobalScope inside a *ViewModel:
// - a backticked class name: Go keeps the backticks, so `TickViewModel`
//   does not end in ViewModel for it;
// - a receiver Go does not read as a name: parenthesized, backticked, a
//   cast, a smart cast, `!!`, a call result, or `this.` plus a property
//   holding GlobalScope;
// - a backticked builder name, which Go compares with the backticks;
// - an explicit `this` inside a scope function on GlobalScope. Only the
//   implicit form (`with(GlobalScope) { launch {} }`) goes unreported, as in
//   Go (see GlobalScopeLaunchInViewModelNegative.kt).
package test.receivers

import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.async
import kotlinx.coroutines.launch

class `TickViewModel` {
    fun a() {
        <!GlobalScopeLaunchInViewModel!>kotlinx<!>.coroutines.GlobalScope.launch { }
    }
}

class ReceiverViewModel {
    private val gs: GlobalScope? = GlobalScope
    private val held = GlobalScope

    fun spellings() {
        <!GlobalScopeLaunchInViewModel!>(<!>GlobalScope).launch { }
        <!GlobalScopeLaunchInViewModel!>kotlinx<!>.coroutines.`GlobalScope`.launch { }
        <!GlobalScopeLaunchInViewModel!>`GlobalScope`<!>.async { 1 }
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.`launch` { }
    }

    fun casts(a: Any, b: Any, c: Any) {
        <!GlobalScopeLaunchInViewModel!>(<!>a as GlobalScope).launch { }
        <!GlobalScopeLaunchInViewModel!>(<!>b as? GlobalScope)?.launch { }
        if (c is GlobalScope) <!GlobalScopeLaunchInViewModel!>c<!>.launch { }
    }

    fun receivers() {
        GlobalScope.apply { <!GlobalScopeLaunchInViewModel!>this<!>.launch { } }
        with(GlobalScope) { <!GlobalScopeLaunchInViewModel!>this<!>.async { 1 } }
        <!GlobalScopeLaunchInViewModel!>gs<!>!!.launch { }
        <!GlobalScopeLaunchInViewModel!>this<!>.held.launch { }
        <!GlobalScopeLaunchInViewModel!>arrayOf<!>(GlobalScope).first().launch { }
    }
}
