// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 23, 30
// `this` and implicit receivers for Wakelock. An acquire or release also
// counts on an implicit receiver (a with / apply / run lambda's), which Go
// does not see. `this` is the declaration or lambda receiver it is bound to.
// The receiver of a WakeLock extension is the caller's lock, like a
// parameter, so an acquire on it is not reported.
package test

import android.os.PowerManager.WakeLock

// Go does not count a release on an implicit receiver or on `this`, so it
// reports these three acquires; each lock is released.
class WithReleased(private val lock: WakeLock) {
    fun run() {
        lock.acquire()
        with(lock) { release() }
    }
}

class ApplyReleased(private val lock: WakeLock) {
    fun run() {
        lock.acquire()
        lock.apply { release() }
    }
}

class RunReleased(private val lock: WakeLock) {
    fun run() {
        lock.acquire()
        lock.run { this.release() }
    }
}

// Go cannot type `this`, so it sees no acquire in these extensions.
fun WakeLock.holdImplicitRelease() {
    this.acquire()
    release()
}

fun WakeLock.holdUnreleased() {
    this.acquire()
}

fun WakeLock.holdImplicitUnreleased() {
    acquire()
}

// Go misses these: it needs a written receiver for an acquire (with the
// oracle, it would type the implicit receiver). Nothing releases the lock.
class ImplicitAcquire(private val lock: WakeLock) {
    fun run() {
        with(lock) { <!Wakelock!>acquire()<!> }
        lock.apply { <!Wakelock!>acquire()<!> }
    }
}

// Inside `with(other)`, the implicit receiver of release() is `other`, not
// `lock`, which is never released.
class ReleasingOther(private val lock: WakeLock, private val other: WakeLock) {
    fun run() {
        with(lock) {
            <!Wakelock!>acquire()<!>
            with(other) { release() }
        }
    }
}

// Released through the same implicit or explicit receiver: not reported.
class ImplicitAcquireReleased(private val lock: WakeLock) {
    fun run() {
        with(lock) { acquire() }
        lock.release()
    }
}

class ApplyBoth(private val lock: WakeLock) {
    fun run() {
        lock.apply {
            acquire()
            this.release()
        }
    }
}
