// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 22, 26, 27, 28, 29, 30, 32, 33, 34, 35, 36, 41, 42, 44, 52, 57, 62, 69, 75, 82, 90, 94, 98, 102, 104, 111
// Positive: GlobalScope.launch/async inside a class whose name ends in
// ViewModel or Presenter. The nearest enclosing class decides; objects,
// companion objects, object literals, and enum-entry bodies are looked
// through, as Go's class_declaration walk does. Reported on the builder's
// name, so a chain split across lines is reported on its `.launch` line, the
// line the Go rule reports.
package test

import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.GlobalScope
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.launch

class UserViewModel {
    val job = GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }

    init {
        GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }
    }

    fun load() {
        GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }
        GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!>(Dispatchers.IO) { }
        GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!>(context = Dispatchers.IO, start = CoroutineStart.LAZY) { }
        GlobalScope.<!GlobalScopeLaunchInViewModel!>async<!> { 1 }
        GlobalScope.<!GlobalScopeLaunchInViewModel!>async<!>(Dispatchers.Default) { 1 }.start()
        GlobalScope
            .<!GlobalScopeLaunchInViewModel!>launch<!> { }
        <!GlobalScopeLaunchInViewModel!>kotlinx<!>.coroutines.GlobalScope.launch { }
        GlobalScope?.<!GlobalScopeLaunchInViewModel!>launch<!> { }
        GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> {
            GlobalScope.<!GlobalScopeLaunchInViewModel!>async<!> { 2 }
        }
    }

    fun nested() {
        listOf(1).forEach { GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { } }
        val block = fun() { GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { } }
        fun local() {
            GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }
        }
        block()
        local()
    }

    private val listener = object : Runnable {
        override fun run() {
            GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }
        }
    }

    object Helper {
        fun start() = GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }
    }

    companion object {
        fun start() {
            GlobalScope.<!GlobalScopeLaunchInViewModel!>async<!> { }
        }
    }
}

class LoginPresenter {
    fun attach() {
        GlobalScope.<!GlobalScopeLaunchInViewModel!>async<!> { "user" }
    }
}

interface ScreenPresenter {
    fun start() {
        GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }
    }
}

enum class ModeViewModel {
    FAST {
        override fun go() {
            GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }
        }
    },
    ;

    abstract fun go()
}

class FeedViewModel(val job: Job = GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { })

abstract class BaseViewModel(val job: Job)

class DetailViewModel : BaseViewModel(GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { })

class EditViewModel(id: Int) : BaseViewModel(Job()) {
    constructor() : this(0) {
        GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }
    }

    val lazyJob: Job
        get() = GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }

    fun restart(job: Job = GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }) = job
}

// Only the class's name decides; a local class counts as its own owner.
fun outer() {
    class LocalViewModel {
        fun f() {
            GlobalScope.<!GlobalScopeLaunchInViewModel!>launch<!> { }
        }
    }
}
