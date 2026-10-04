// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 29, 36, 43, 60, 70
// Scopes for Wakelock: the acquire belongs to its nearest enclosing named
// function, looking through lambdas, anonymous functions, and local classes
// and objects; a member function of a local class is its own function. An
// acquire outside any named function is not reported, as in Go.
package test

import android.os.PowerManager.WakeLock

class Scopes(private val lock: WakeLock) {
    init {
        lock.acquire()
    }

    val eager: Unit = lock.acquire()

    val lazyAcquire: Unit
        get() = lock.acquire()

    fun inLambda() {
        run {
            <!Wakelock!>lock.acquire()<!>
        }
    }

    fun inAnonymousFunction() {
        val block = fun() {
            <!Wakelock!>lock.acquire()<!>
        }
        block()
    }

    fun inLocalFunction() {
        fun start() {
            <!Wakelock!>lock.acquire()<!>
        }
        start()
    }

    fun localFunctionReleasedOutside() {
        fun start() {
            <!Wakelock!>lock.acquire()<!>
        }
        start()
        lock.release()
    }

    fun localFunctionReleasedInside() {
        fun start() {
            lock.acquire()
            lock.release()
        }
        start()
    }

    fun anonymousObjectMember() {
        val task = object : Runnable {
            override fun run() {
                <!Wakelock!>lock.acquire()<!>
            }
        }
        task.run()
        lock.release()
    }

    fun anonymousObjectInit() {
        val task = object {
            init {
                <!Wakelock!>lock.acquire()<!>
            }
        }
        task.hashCode()
    }

    fun anonymousObjectInitReleased() {
        val task = object {
            init {
                lock.acquire()
            }
        }
        task.hashCode()
        lock.release()
    }

    fun releasedInLocalClassMember() {
        lock.acquire()
        class Cleanup {
            fun done() {
                lock.release()
            }
        }
        Cleanup().done()
    }
}
