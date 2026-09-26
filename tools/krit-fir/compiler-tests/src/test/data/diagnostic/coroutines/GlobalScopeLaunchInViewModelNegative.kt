// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: no GlobalScope receiver, no ViewModel/Presenter owner, or an owner
// that is only an object or an enclosing scope of the nearest class.
package test.negative

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.MainScope
import kotlinx.coroutines.async
import kotlinx.coroutines.launch

class UserViewModel(private val viewModelScope: CoroutineScope) {
    fun load(scope: CoroutineScope) {
        viewModelScope.launch { }
        scope.async { }
        MainScope().launch { }
        // An implicit GlobalScope receiver is not reported, as in Go. An
        // explicit `this.launch` is (GlobalScopeLaunchInViewModelReceivers.kt).
        with(GlobalScope) { launch { } }
    }

    // The nearest class is Inner, whose name does not end in ViewModel.
    class Inner {
        fun f() {
            GlobalScope.launch { }
        }
    }

    fun withLocalClass() {
        class Local {
            fun g() {
                GlobalScope.launch { }
            }
        }
        Local().g()
    }
}

class Repository {
    fun sync() {
        GlobalScope.launch { }
    }
}

class ViewModelFactory {
    fun create() {
        GlobalScope.launch { }
    }
}

// Go reads only class_declaration nodes; an object is not one.
object SingletonViewModel {
    fun start() {
        GlobalScope.launch { }
    }
}

fun topLevel() {
    GlobalScope.launch { }
}

fun UserViewModel.extension() {
    GlobalScope.launch { }
}
