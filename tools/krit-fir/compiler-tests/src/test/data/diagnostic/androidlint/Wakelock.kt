// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 31, 35, 39, 44, 48
// Wakelock basics: an acquire on a WakeLock that nothing in the file releases
// is reported on the acquire's first line. A release of the same receiver
// anywhere in the file clears it: in the same function (before or after, in a
// finally, a lambda, a local function, an anonymous object) or in another
// one, as in the acquire in onResume, release in onPause shape Android Lint
// accepts. A lock reached through a parameter is the caller's to release and
// is not reported.
package test

import android.os.PowerManager
import android.os.PowerManager.WakeLock

class Unreleased(private val powerManager: PowerManager) {
    private var held: WakeLock? = null
    private lateinit var lock: WakeLock
    private lateinit var spare: WakeLock

    fun unreleasedLocal() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "sync")
        <!Wakelock!>wakeLock.acquire()<!>
    }

    fun unreleasedTimeout() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "sync")
        wakeLock.acquire(1000L)
    }

    fun unreleasedProperty() {
        <!Wakelock!>lock.acquire()<!>
    }

    fun unreleasedSafeCall() {
        <!Wakelock!>held?.acquire()<!>
    }

    fun unreleasedMultiline() {
        <!Wakelock!>lock<!>
            .acquire()
    }

    fun unrelatedRelease() {
        <!Wakelock!>lock.acquire()<!>
        spare.release()
    }

    fun expressionBody() = <!Wakelock!>lock.acquire()<!>
}

// Android Lint's intended use: acquire in one lifecycle method, release in
// another.
class Lifecycle(private val powerManager: PowerManager) {
    private var wakeLock: WakeLock? = null

    fun onResume() {
        wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "sync")
        wakeLock?.acquire()
    }

    fun onPause() {
        wakeLock?.release()
    }
}

class ReleasedInFinally(private val powerManager: PowerManager) {
    fun run() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "sync")
        wakeLock.acquire()
        try {
            work()
        } finally {
            wakeLock.release()
        }
    }

    private fun work() {}
}

class ReleasedWithFlags(private val powerManager: PowerManager) {
    fun run() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "sync")
        wakeLock.acquire()
        wakeLock.release(PowerManager.RELEASE_FLAG_WAIT_FOR_NO_PROXIMITY)
    }
}

class ReleasedBefore(private val lock: WakeLock) {
    fun run() {
        lock.release()
        lock.acquire()
    }
}

class ReleasedSafeCall {
    private var held: WakeLock? = null

    fun run() {
        held?.acquire()
        held?.release()
    }
}

class ReleasedThroughThis(private val lock: WakeLock) {
    fun run() {
        this.lock.acquire()
        lock.release()
    }
}

class ReleasedInLambda(private val lock: WakeLock) {
    fun run() {
        lock.acquire()
        run { lock.release() }
    }
}

class ReleasedInLocalFunction(private val lock: WakeLock) {
    fun run() {
        lock.acquire()
        fun done() {
            lock.release()
        }
        done()
    }
}

class ReleasedInAnonymousObject(private val lock: WakeLock) {
    fun run() {
        lock.acquire()
        val cleanup = object : Runnable {
            override fun run() {
                lock.release()
            }
        }
        cleanup.run()
    }
}

// Released outside any function, in an init block.
class ReleasedInInit(private val lock: WakeLock) {
    init {
        lock.release()
    }

    fun run() {
        lock.acquire()
    }
}

// A lock passed in, or the receiver of an extension, is the caller's.
class Callers {
    fun hold(wakeLock: WakeLock) {
        wakeLock.acquire()
    }

    fun holdThrough(holder: Unreleased, other: WakeLock?) {
        other?.acquire()
        holder.hashCode()
    }
}

fun WakeLock.hold() {
    this.acquire()
}

class Box {
    lateinit var lock: WakeLock
}

// Go misses this: it matches a release by the receiver's last identifier,
// so `second.lock` clears `first.lock`. They are different objects, and
// `first.lock` is never released.
class Boxes {
    private val first = Box()
    private val second = Box()

    fun run() {
        <!Wakelock!>first.lock.acquire()<!>
        second.lock.release()
    }
}
