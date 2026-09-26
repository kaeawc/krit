// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 21, 25, 26, 27, 28, 29, 30, 32, 33, 34, 35, 40, 41, 43, 51, 56, 61, 68, 74, 81, 89, 93, 97, 101, 103, 110
// Positive: GlobalScope.launch/async inside a class whose name ends in
// ViewModel or Presenter. The nearest enclosing class decides; objects,
// companion objects, object literals, and enum-entry bodies are looked
// through, as Go's class_declaration walk does. Reported on the call's first
// line (the receiver), the line the Go rule reports.
package test

import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.launch

class UserViewModel {
    val job = <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }

    init {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
    }

    fun load() {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch(Dispatchers.IO) { }
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch(context = Dispatchers.IO, start = CoroutineStart.LAZY) { }
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.async { 1 }
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.async(Dispatchers.Default) { 1 }.start()
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>
            .launch { }
        <!GlobalScopeLaunchInViewModel!>kotlinx<!>.coroutines.GlobalScope.launch { }
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>?.launch { }
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch {
            <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.async { 2 }
        }
    }

    fun nested() {
        listOf(1).forEach { <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { } }
        val block = fun() { <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { } }
        fun local() {
            <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
        }
        block()
        local()
    }

    private val listener = object : Runnable {
        override fun run() {
            <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
        }
    }

    object Helper {
        fun start() = <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
    }

    companion object {
        fun start() {
            <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.async { }
        }
    }
}

class LoginPresenter {
    fun attach() {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.async { "user" }
    }
}

interface ScreenPresenter {
    fun start() {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
    }
}

enum class ModeViewModel {
    FAST {
        override fun go() {
            <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
        }
    },
    ;

    abstract fun go()
}

class FeedViewModel(val job: Job = <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { })

abstract class BaseViewModel(val job: Job)

class DetailViewModel : BaseViewModel(<!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { })

class EditViewModel(id: Int) : BaseViewModel(Job()) {
    constructor() : this(0) {
        <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
    }

    val lazyJob: Job
        get() = <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }

    fun restart(job: Job = <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }) = job
}

// Only the class's name decides; a local class counts as its own owner.
fun outer() {
    class LocalViewModel {
        fun f() {
            <!GlobalScopeLaunchInViewModel!>GlobalScope<!>.launch { }
        }
    }
}
