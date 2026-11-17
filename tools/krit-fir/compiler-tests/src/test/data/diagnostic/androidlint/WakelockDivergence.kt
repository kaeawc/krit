// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 23, 45, 71, 85
// Divergences from Go for Wakelock.
package test

import android.os.PowerManager
import android.os.PowerManager.WakeLock

// Go types the parenthesized receiver but reads no name through the
// parentheses, so it cannot match this acquire to the release of the same
// lock and reports it. The lock is released, so the message is false; not
// reported here.
class ParenthesizedReleased(private val lock: WakeLock) {
    fun run() {
        (lock).acquire()
        lock.release()
    }
}

// Reported through parentheses when nothing releases the lock, as Go does.
class ParenthesizedUnreleased(private val held: WakeLock) {
    fun run() {
        <!Wakelock!>(held).acquire()<!>
    }
}

// Go misses this: its source inference cannot type `pending!!`, and its
// newWakeLock fallback reads no name through `!!`. The WakeLock is
// acquired and never released.
class NotNullUnreleased {
    private var pending: WakeLock? = null

    fun run() {
        <!Wakelock!>pending!!.acquire()<!>
    }
}

// Go types `held` in `held?.acquire()` but not `held!!`, so it does not
// count this release and reports the acquire. The lock is released; not
// reported here.
class NotNullReleaseOfSafeAcquire {
    private var held: WakeLock? = null

    fun run() {
        held?.acquire()
        held!!.release()
    }
}

// Released through `!!`: neither reports it.
class NotNullReleased {
    private var held: WakeLock? = null

    fun run() {
        held!!.acquire()
        held!!.release()
    }
}

// Go's newWakeLock fallback matches the earlier `lock` declared inside the
// lambda, which the receiver does not refer to, and reports this acquire.
// The receiver is the outer `lock`, a `Latch`, not a WakeLock, so the
// message is false; not reported here.
class ShadowedName(private val powerManager: PowerManager) {
    fun run() {
        run {
            val lock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "inner")
            lock.hashCode()
        }
        val lock = Latch()
        lock.acquire()
    }
}

class Latch {
    fun acquire() {}
}

// Locals of different functions are different variables, and start()'s
// lock is never released. Go reports it too: for a local it looks for the
// release only in the acquiring function.
class LocalsByName(private val powerManager: PowerManager) {
    fun start() {
        val lock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "a")
        <!Wakelock!>lock.acquire()<!>
    }

    fun stop() {
        val lock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "b")
        lock.release()
    }
}
