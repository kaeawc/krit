// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 19, 24
// `this` and implicit receivers for Wakelock. An acquire needs a written
// receiver, as in Go; a release also counts on an implicit receiver (a
// with / apply / run lambda's, or a WakeLock extension's), which Go does not
// see. `this` names the declaration or lambda it is bound to.
package test

import android.os.PowerManager.WakeLock

// Go does not count a release on an implicit receiver or on `this`, so it
// reports these three acquires; each lock is released.
fun withReleased(lock: WakeLock) {
    lock.acquire()
    with(lock) { release() }
}

fun applyReleased(lock: WakeLock) {
    lock.acquire()
    lock.apply { release() }
}

fun runReleased(lock: WakeLock) {
    lock.acquire()
    lock.run { this.release() }
}

// Go cannot type `this`, so it sees no acquire in these extensions; the two
// unreleased ones below are findings Go misses.
fun WakeLock.holdImplicitRelease() {
    this.acquire()
    release()
}

fun WakeLock.holdExplicitRelease() {
    this.acquire()
    this@holdExplicitRelease.release()
}

fun WakeLock.holdUnreleased() {
    <!Wakelock!>this.acquire()<!>
}

// Inside `with(other)`, the implicit receiver of release() is `other`, not
// the extension receiver.
fun WakeLock.holdReleasingOther(other: WakeLock) {
    <!Wakelock!>this.acquire()<!>
    with(other) { release() }
}

// An acquire on an implicit receiver is not reported, as in Go.
fun implicitAcquire(lock: WakeLock) {
    with(lock) { acquire() }
    lock.apply { acquire() }
}
