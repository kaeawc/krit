// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 120
// Aliases for Wakelock: a receiver also names the variable it aliases (a
// local variable holding one where it is read, directly or through an elvis,
// and the parameter of a let / also / takeIf / takeUnless lambda on one), so
// a release through either name clears the acquire.
package test

import android.os.PowerManager.WakeLock

class LetAcquireReleased(private val wakeLock: WakeLock?) {
    // Go does not type `it`, so it sees neither call.
    fun run() {
        wakeLock?.let { it.acquire() }
        wakeLock?.release()
    }
}

class LetReleased(private val lock: WakeLock) {
    // Go does not count the release through `it` and reports the acquire;
    // the lock is released.
    fun run() {
        lock.acquire()
        lock.also { it.release() }
    }
}

class LocalAliasReleased(private val lock: WakeLock) {
    fun run() {
        val alias = lock
        alias.acquire()
        lock.release()
    }
}

class ElvisAliasReleased(private val wakeLock: WakeLock?) {
    fun run() {
        val alias = wakeLock ?: return
        alias.acquire()
        wakeLock.release()
    }
}

class NamedLambdaParameterReleased(private val lock: WakeLock) {
    fun run() {
        lock.takeIf { true }?.let { held -> held.acquire() }
        lock.release()
    }
}

class LetAcquireUnreleased(private val wakeLock: WakeLock?) {
    // Go misses this: it does not type `it`. Nothing releases the lock.
    fun run() {
        wakeLock?.let { <!Wakelock!>it.acquire()<!> }
    }
}

// The shape of a helper that acquires in one method and releases in another
// through lambda parameters bound to the same property.
class LetAcrossFunctions(private val wakeLock: WakeLock?) {
    fun acquire() {
        wakeLock?.let { if (!it.isHeld) it.acquire() }
    }

    fun release() {
        wakeLock?.let { lock -> if (lock.isHeld) lock.release() }
    }
}

class VarAliasReleased(private val lock: WakeLock) {
    // A var never assigned again aliases its initializer.
    fun run() {
        var alias = lock
        alias.acquire()
        lock.release()
    }
}

class ReassignedVarReleased(private val other: WakeLock, private val lock: WakeLock) {
    // A reassigned var aliases the value it holds where it is read: here
    // `lock`, which is released.
    fun run() {
        var alias = other
        alias = lock
        alias.acquire()
        lock.release()
        other.hashCode()
    }
}

class ReassignedVarUnreleased(private val other: WakeLock, private val lock: WakeLock) {
    // Go misses this: it cannot type `alias`. When acquired, `alias` holds
    // `other`, which is never released.
    fun run() {
        var alias = lock
        alias = other
        <!Wakelock!>alias.acquire()<!>
        lock.release()
    }
}

class CallAliasUnreleased(private val factory: () -> WakeLock) {
    // Go misses this: it cannot type `alias`. A call result is not an alias,
    // as each call may return a different lock, so the release of another
    // call's result does not clear the acquire (Go's name match does not
    // relate them either).
    fun run() {
        val alias = factory()
        <!Wakelock!>alias.acquire()<!>
        factory().release()
    }
}

// A lambda parameter of a call other than a scope function is the caller's
// lock, like a function parameter: identity cannot follow an element of a
// collection to a release through another iteration. Go reports it: it
// types `it` from the list and reads no lambda parameter as the caller's.
class ForEachParameter(private val locks: List<WakeLock>) {
    fun run() {
        locks.forEach { it.acquire() }
    }
}
