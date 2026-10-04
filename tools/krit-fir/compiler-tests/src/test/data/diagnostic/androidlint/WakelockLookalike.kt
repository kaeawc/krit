// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 54, 64
// Lookalikes for Wakelock: `acquire` on receivers that are not WakeLocks is
// not reported. A third-party class named WakeLock, and a variable a call
// written `newWakeLock` initializes, count as WakeLocks, as in Go.
package test

import java.util.concurrent.Semaphore

class Lock {
    fun acquire() {}
    fun release() {}
}

class WakeLock {
    fun acquire() {}
    fun release() {}
}

class LockFactory {
    fun newWakeLock(tag: String): Lock = Lock()
}

fun acquire() {}

fun semaphore(permits: Semaphore) {
    permits.acquire()
}

fun projectLock(lock: Lock) {
    lock.acquire()
}

fun bareCall() {
    acquire()
}

// Go misses this: it needs a written receiver. The implicit receiver is a
// WakeLock that is never released.
fun implicitReceiver(lock: android.os.PowerManager.WakeLock) {
    with(lock) {
        <!Wakelock!>acquire()<!>
    }
}

// The implicit receiver of `acquire()` here is the Semaphore.
fun implicitSemaphore(permits: Semaphore) {
    with(permits) {
        acquire()
    }
}

fun thirdPartyWakeLock(lock: WakeLock) {
    <!Wakelock!>lock.acquire()<!>
}

fun thirdPartyReleased(lock: WakeLock) {
    lock.acquire()
    lock.release()
}

fun factoryLock(factory: LockFactory) {
    val lock = factory.newWakeLock("tag")
    <!Wakelock!>lock.acquire()<!>
}

fun factoryReleased(factory: LockFactory) {
    val lock = factory.newWakeLock("tag")
    lock.acquire()
    lock.release()
}
