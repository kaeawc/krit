// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21
// Aliases for Wakelock: a receiver also names the variable it aliases (a
// local variable holding one where it is read, directly or through an elvis,
// and the parameter of a let / also / takeIf / takeUnless lambda on one), so
// a release through either name clears the acquire.
package test

import android.os.PowerManager.WakeLock

class Aliases(private val wakeLock: WakeLock?, private val factory: () -> WakeLock) {
    // Go does not type `it`, so it sees neither call.
    fun letAcquireReleased() {
        wakeLock?.let { it.acquire() }
        wakeLock?.release()
    }

    // Go does not count the release through `it` and reports the acquire;
    // the lock is released.
    fun letReleased(lock: WakeLock) {
        lock.acquire()
        lock.also { it.release() }
    }

    fun localAliasReleased(lock: WakeLock) {
        val alias = lock
        alias.acquire()
        lock.release()
    }

    fun elvisAliasReleased() {
        val alias = wakeLock ?: return
        alias.acquire()
        wakeLock.release()
    }

    fun namedLambdaParameterReleased(lock: WakeLock) {
        lock.takeIf { true }?.let { held -> held.acquire() }
        lock.release()
    }

    // Go misses this: it does not type `it`. Nothing releases the lock.
    fun letAcquireUnreleased() {
        wakeLock?.let { <!Wakelock!>it.acquire()<!> }
    }

    // A var never assigned again aliases its initializer.
    fun varAliasReleased(lock: WakeLock) {
        var alias = lock
        alias.acquire()
        lock.release()
    }

    // A reassigned var aliases the value it holds where it is read: here
    // `lock`, which is released.
    fun reassignedVarReleased(other: WakeLock, lock: WakeLock) {
        var alias = other
        alias = lock
        alias.acquire()
        lock.release()
        other.hashCode()
    }

    // Go misses this: it cannot type `alias`. When acquired, `alias` holds
    // `other`, which is never released.
    fun reassignedVarUnreleased(other: WakeLock, lock: WakeLock) {
        var alias = lock
        alias = other
        <!Wakelock!>alias.acquire()<!>
        lock.release()
    }

    // Go misses this: it cannot type `alias`. A call result is not an alias,
    // as each call may return a different lock, so the release of another
    // call's result does not clear the acquire (Go's name match does not
    // relate them either).
    fun callAliasUnreleased() {
        val alias = factory()
        <!Wakelock!>alias.acquire()<!>
        factory().release()
    }
}
