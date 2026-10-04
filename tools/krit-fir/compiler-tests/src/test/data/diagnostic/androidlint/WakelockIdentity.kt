// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 29, 34, 39, 44, 54, 61, 71, 88, 93, 124, 133, 142, 169
// Receiver identity for Wakelock: a release clears an acquire only when it is
// on the same object, the same variable reached through the same receiver
// chain (implicit and explicit `this` are the same), or a local or lambda
// alias of it. An alias or a release on a different object whose last
// segment happens to share the acquire's name does not count.
package test

import android.os.PowerManager
import android.os.PowerManager.WakeLock

class LockHolder {
    lateinit var lock: PowerManager.WakeLock
}

class Svc(private val lock: WakeLock, private val powerManager: PowerManager) {
    private var wakeLock: WakeLock? = null

    // Go reports these: `theirs`, `it`, and the implicit receivers are
    // `other.lock`, a different object, so this.lock is never released.
    fun localAliasOfOther(other: Svc) {
        val theirs = other.lock
        <!Wakelock!>lock.acquire()<!>
        theirs.release()
    }

    fun letOnOther(other: Svc) {
        <!Wakelock!>lock.acquire()<!>
        other.lock.let { it.release() }
    }

    fun withOnOther(other: Svc) {
        <!Wakelock!>lock.acquire()<!>
        with(other.lock) { release() }
    }

    fun applyOnOther(other: Svc) {
        <!Wakelock!>lock.acquire()<!>
        other.lock.apply { release() }
    }

    fun elvisAliasOfOther(other: Svc?) {
        <!Wakelock!>lock.acquire()<!>
        val theirs = other?.lock ?: return
        theirs.release()
    }

    // The lock-swap pattern: the old lock is released and the new one, which
    // was just acquired, is stored. Go reports the acquire, and nothing in the
    // function releases the new lock.
    fun swapThroughLet() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "x")
        <!Wakelock!>wakeLock.acquire()<!>
        this.wakeLock?.let { it.release() }
        this.wakeLock = wakeLock
    }

    fun swapThroughLocal() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "x")
        <!Wakelock!>wakeLock.acquire()<!>
        val old = this.wakeLock
        old?.release()
        this.wakeLock = wakeLock
    }

    // Go cannot type `holder.lock` (a qualified `PowerManager.WakeLock`), so
    // it does not count the release and reports the acquire: this.lock is
    // never released.
    fun releaseOfHolder(holder: LockHolder) {
        <!Wakelock!>lock.acquire()<!>
        holder.lock.release()
    }

    // Go misses this: it cannot type `it`. The two `it`s are different locks,
    // and `a` is never released.
    fun letParametersOfDifferentLocks(a: WakeLock, b: WakeLock) {
        a.let { <!Wakelock!>it.acquire()<!> }
        b.let { it.release() }
    }

    // The same object through an alias, a scope lambda, `this`, or the same
    // receiver chain: released, not reported. Go does not relate `mine` to
    // `lock` and does not count a release through `it`, so it reports the
    // next two acquires; each lock is released.
    fun aliasOfOwnLock() {
        val mine = this.lock
        lock.acquire()
        mine.release()
    }

    fun letOnOwnLock() {
        lock.acquire()
        this.lock.let { it.release() }
    }

    // Go sees no acquire here: it does not type `this.lock`.
    fun withOnOwnLock() {
        this.lock.acquire()
        with(lock) { release() }
    }

    fun sameChain(other: Svc) {
        other.lock.acquire()
        other.lock.release()
    }

    fun chainThroughAlias(other: Svc) {
        val same = other
        same.lock.acquire()
        other.lock.release()
    }

    fun swapReleasingNew() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "x")
        wakeLock.acquire()
        this.wakeLock?.let { it.release() }
        this.wakeLock = wakeLock
        wakeLock.release()
    }

    // Go reports this too (it cannot type `it`); the lock is released.
    fun releasedThroughProperty() {
        wakeLock?.acquire()
        wakeLock?.let { if (it.isHeld) it.release() }
    }

    // `old` holds the lock stored before the assignment; the acquired one is
    // the new lock, which is never released. Go reports it too.
    fun aliasBeforeReassignment(fresh: WakeLock) {
        val old = this.wakeLock
        this.wakeLock = fresh
        <!Wakelock!>wakeLock?.acquire()<!>
        old?.release()
    }

    // With no assignment in the function, `old` is the acquired lock. Go does
    // not relate `old` to `wakeLock` and reports the acquire; the lock is
    // released.
    fun aliasOfUnassignedProperty() {
        val old = this.wakeLock
        wakeLock?.acquire()
        old?.release()
    }
}

class Shared {
    companion object {
        lateinit var shared: WakeLock
    }

    fun implicitThenQualified() {
        shared.acquire()
        Shared.shared.release()
    }

    fun companionThenImplicit() {
        Companion.shared.acquire()
        shared.release()
    }
}

object Global {
    lateinit var lock: WakeLock

    // Go does not count the release through the object qualifier and reports
    // the acquire; the lock is released.
    fun implicitThenQualified() {
        lock.acquire()
        Global.lock.release()
    }
}

lateinit var topLevelLock: WakeLock

fun topLevel() {
    topLevelLock.acquire()
    topLevelLock.release()
}
