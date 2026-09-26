// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 23, 27, 31, 35, 39, 45, 50, 107
// Wakelock basics: an acquire on a WakeLock whose enclosing
// function never releases the same receiver is reported on the acquire's
// first line; a release anywhere in the function (before or after, in a
// finally, a lambda, or a local function) clears it.
package test

import android.os.PowerManager
import android.os.PowerManager.WakeLock

class SyncService(private val powerManager: PowerManager) {
    private var held: WakeLock? = null
    private lateinit var lock: WakeLock

    fun unreleasedLocal() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "sync")
        <!Wakelock!>wakeLock.acquire()<!>
    }

    fun unreleasedTimeout() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "sync")
        <!Wakelock!>wakeLock.acquire(1000L)<!>
    }

    fun unreleasedParameter(wakeLock: WakeLock) {
        <!Wakelock!>wakeLock.acquire()<!>
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

    fun twoAcquires(other: WakeLock) {
        lock.acquire()
        <!Wakelock!>other.acquire()<!>
        lock.release()
    }

    fun unrelatedRelease(other: WakeLock) {
        <!Wakelock!>lock.acquire()<!>
        other.release()
    }

    fun releasedInFinally() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "sync")
        wakeLock.acquire()
        try {
            work()
        } finally {
            wakeLock.release()
        }
    }

    fun releasedWithFlags(wakeLock: WakeLock) {
        wakeLock.acquire(10L)
        wakeLock.release(PowerManager.RELEASE_FLAG_WAIT_FOR_NO_PROXIMITY)
    }

    fun releasedBefore() {
        lock.release()
        lock.acquire()
    }

    fun releasedSafeCall() {
        held?.acquire()
        held?.release()
    }

    fun releasedThroughThis() {
        this.lock.acquire()
        lock.release()
    }

    fun releasedInLambda() {
        lock.acquire()
        run { lock.release() }
    }

    fun releasedInLocalFunction() {
        lock.acquire()
        fun done() {
            lock.release()
        }
        done()
    }

    fun releasedInAnonymousObject() {
        lock.acquire()
        val cleanup = object : Runnable {
            override fun run() {
                lock.release()
            }
        }
        cleanup.run()
    }

    fun expressionBody() = <!Wakelock!>lock.acquire()<!>

    // Go misses this: it matches a release by the receiver's last identifier,
    // so `second.lock` clears `first.lock`. They are different objects, and
    // `first.lock` is never released.
    fun releasedByName(first: SyncService, second: SyncService) {
        <!Wakelock!>first.lock.acquire()<!>
        second.lock.release()
    }

    private fun work() {}
}
