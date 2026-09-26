// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 23, 37, 57
// Divergences from Go for Wakelock.
package test

import android.os.PowerManager
import android.os.PowerManager.WakeLock

class Worker(private val powerManager: PowerManager) {
    private var held: WakeLock? = null

    // Go types the parenthesized receiver but reads no name through the
    // parentheses, so it cannot match this acquire to the release of the same
    // lock and reports it. The lock is released, so the message is false; not
    // reported here.
    fun parenthesizedReleased(lock: WakeLock) {
        (lock).acquire()
        lock.release()
    }

    // Reported through parentheses when nothing releases the lock, as Go does.
    fun parenthesizedUnreleased(lock: WakeLock) {
        <!Wakelock!>(lock).acquire()<!>
    }

    // Go misses this: its source inference cannot type `held!!`, and its
    // newWakeLock fallback reads no name through `!!`. The WakeLock is
    // acquired and never released.
    fun notNullUnreleased() {
        <!Wakelock!>held!!.acquire()<!>
    }

    // Go types `held` in `held?.acquire()` but not `held!!`, so it does not
    // count this release and reports the acquire. The lock is released; not
    // reported here.
    fun notNullReleaseOfSafeAcquire() {
        held?.acquire()
        held!!.release()
    }

    // Released through `!!`: neither reports it.
    fun notNullReleased() {
        held!!.acquire()
        held!!.release()
    }

    // Go's newWakeLock fallback matches the earlier `lock` declared inside the
    // lambda, which the receiver does not refer to, and reports this acquire.
    // The receiver is the outer `lock`, a `Latch`, not a WakeLock, so the
    // message is false; not reported here.
    fun shadowedName(latch: Latch) {
        run {
            val lock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "inner")
            lock.hashCode()
        }
        val lock = latch
        lock.acquire()
    }
}

class Latch {
    fun acquire() {}
}
