// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives for WakelockTimeout: a timeout and calls on other types.
package test.wakelocktimeoutnegative

import android.os.PowerManager

class Worker(private val lock: PowerManager.WakeLock) {
    fun timeout() {
        lock.acquire(10_000L)
    }

    fun release() {
        lock.release()
    }

    fun held(): Boolean = lock.isHeld
}

// A project type that merely has an acquire() is not a wake lock.
class Semaphore {
    fun acquire() = Unit
}

fun other(semaphore: Semaphore) {
    semaphore.acquire()
}

fun PowerManager.WakeLock.acquireWithTimeout() {
    acquire(1L)
}
