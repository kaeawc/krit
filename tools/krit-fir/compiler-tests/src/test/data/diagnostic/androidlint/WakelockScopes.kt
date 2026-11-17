// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 34, 43, 73, 96
// Scopes for Wakelock: an acquire is reported only inside a named function,
// looking through lambdas, anonymous functions, and local classes and
// objects; a member function of a local class is its own function. An acquire
// outside any named function is not reported, as in Go. A release anywhere in
// the file counts, inside or outside a nested scope.
package test

import android.os.PowerManager.WakeLock

class OutsideFunctions(private val lock: WakeLock) {
    init {
        lock.acquire()
    }

    val eager: Unit = lock.acquire()

    val lazyAcquire: Unit
        get() = lock.acquire()
}

class InLambda(private val lock: WakeLock) {
    fun run() {
        run {
            <!Wakelock!>lock.acquire()<!>
        }
    }
}

class InAnonymousFunction(private val lock: WakeLock) {
    fun run() {
        val block = fun() {
            <!Wakelock!>lock.acquire()<!>
        }
        block()
    }
}

class InLocalFunction(private val lock: WakeLock) {
    fun run() {
        fun start() {
            <!Wakelock!>lock.acquire()<!>
        }
        start()
    }
}

class LocalFunctionReleasedOutside(private val lock: WakeLock) {
    fun run() {
        fun start() {
            lock.acquire()
        }
        start()
        lock.release()
    }
}

class LocalFunctionReleasedInside(private val lock: WakeLock) {
    fun run() {
        fun start() {
            lock.acquire()
            lock.release()
        }
        start()
    }
}

class AnonymousObjectMember(private val lock: WakeLock) {
    fun run() {
        val task = object : Runnable {
            override fun run() {
                <!Wakelock!>lock.acquire()<!>
            }
        }
        task.run()
    }
}

class AnonymousObjectMemberReleased(private val lock: WakeLock) {
    fun run() {
        val task = object : Runnable {
            override fun run() {
                lock.acquire()
            }
        }
        task.run()
        lock.release()
    }
}

class AnonymousObjectInit(private val lock: WakeLock) {
    fun run() {
        val task = object {
            init {
                <!Wakelock!>lock.acquire()<!>
            }
        }
        task.hashCode()
    }
}

class AnonymousObjectInitReleased(private val lock: WakeLock) {
    fun run() {
        val task = object {
            init {
                lock.acquire()
            }
        }
        task.hashCode()
        lock.release()
    }
}

class ReleasedInLocalClassMember(private val lock: WakeLock) {
    fun run() {
        lock.acquire()
        class Cleanup {
            fun done() {
                lock.release()
            }
        }
        Cleanup().done()
    }
}

// A local in one function is not the same variable as a local of the same
// name in another, so the release in stop() does not clear it. Go misses
// this: it cannot type a local initialized by calling a function-typed
// property.
class LocalsByFunction(private val make: () -> WakeLock) {
    fun start() {
        val lock = make()
        <!Wakelock!>lock.acquire()<!>
    }

    fun stop() {
        val lock = make()
        lock.release()
    }
}
