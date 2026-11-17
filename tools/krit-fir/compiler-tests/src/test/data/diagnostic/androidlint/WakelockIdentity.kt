// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 29, 36, 43, 50, 64, 75, 87, 108, 115, 160, 173, 186, 213
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

// Go reports these: `theirs`, `it`, and the implicit receivers are
// `other.lock`, a different object, so this.lock is never released.
class LocalAliasOfOther(private val lock: WakeLock, private val other: LockHolder) {
    fun run() {
        val theirs = other.lock
        <!Wakelock!>lock.acquire()<!>
        theirs.release()
    }
}

class LetOnOther(private val lock: WakeLock, private val other: LockHolder) {
    fun run() {
        <!Wakelock!>lock.acquire()<!>
        other.lock.let { it.release() }
    }
}

class WithOnOther(private val lock: WakeLock, private val other: LockHolder) {
    fun run() {
        <!Wakelock!>lock.acquire()<!>
        with(other.lock) { release() }
    }
}

class ApplyOnOther(private val lock: WakeLock, private val other: LockHolder) {
    fun run() {
        <!Wakelock!>lock.acquire()<!>
        other.lock.apply { release() }
    }
}

class ElvisAliasOfOther(private val lock: WakeLock, private val other: LockHolder?) {
    fun run() {
        <!Wakelock!>lock.acquire()<!>
        val theirs = other?.lock ?: return
        theirs.release()
    }
}

// The lock-swap pattern: the old lock is released and the new one, which
// was just acquired, is stored. Go reports the acquire, and nothing releases
// the new lock.
class SwapThroughLet(private val powerManager: PowerManager) {
    private var wakeLock: WakeLock? = null

    fun run() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "x")
        <!Wakelock!>wakeLock.acquire()<!>
        this.wakeLock?.let { it.release() }
        this.wakeLock = wakeLock
    }
}

class SwapThroughLocal(private val powerManager: PowerManager) {
    private var wakeLock: WakeLock? = null

    fun run() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "x")
        <!Wakelock!>wakeLock.acquire()<!>
        val old = this.wakeLock
        old?.release()
        this.wakeLock = wakeLock
    }
}

// Go cannot type `holder.lock` (a qualified `PowerManager.WakeLock`), so
// it does not count the release and reports the acquire: this.lock is
// never released.
class ReleaseOfHolder(private val lock: WakeLock, private val holder: LockHolder) {
    fun run() {
        <!Wakelock!>lock.acquire()<!>
        holder.lock.release()
    }
}

// Go misses this: it cannot type `it`. The two `it`s are different locks,
// and `a` is never released.
class LetOfDifferentLocks(private val a: WakeLock, private val b: WakeLock) {
    fun run() {
        a.let { <!Wakelock!>it.acquire()<!> }
        b.let { it.release() }
    }
}

// The same object through an alias, a scope lambda, `this`, or the same
// receiver chain: released, not reported. Go does not relate `mine` to
// `lock` and does not count a release through `it`, so it reports the
// next two acquires; each lock is released.
class AliasOfOwnLock(private val lock: WakeLock) {
    fun run() {
        val mine = this.lock
        lock.acquire()
        mine.release()
    }
}

class LetOnOwnLock(private val lock: WakeLock) {
    fun run() {
        lock.acquire()
        this.lock.let { it.release() }
    }
}

// Go sees no acquire here: it does not type `this.lock`.
class WithOnOwnLock(private val lock: WakeLock) {
    fun run() {
        this.lock.acquire()
        with(lock) { release() }
    }
}

class SameChain(private val other: LockHolder) {
    fun run() {
        other.lock.acquire()
        other.lock.release()
    }
}

class ChainThroughAlias(private val other: LockHolder) {
    fun run() {
        val same = other
        same.lock.acquire()
        other.lock.release()
    }
}

class SwapReleasingNew(private val powerManager: PowerManager) {
    private var wakeLock: WakeLock? = null

    fun run() {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "x")
        wakeLock.acquire()
        this.wakeLock?.let { it.release() }
        this.wakeLock = wakeLock
        wakeLock.release()
    }
}

// Go reports this too (it cannot type `it`); the lock is released.
class ReleasedThroughProperty {
    private var wakeLock: WakeLock? = null

    fun run() {
        wakeLock?.acquire()
        wakeLock?.let { if (it.isHeld) it.release() }
    }
}

// `old` holds the lock stored before the assignment; the acquired one is
// the new lock, which is never released. Go reports it too.
class AliasBeforeReassignment(private val fresh: WakeLock) {
    private var wakeLock: WakeLock? = null

    fun run() {
        val old = this.wakeLock
        this.wakeLock = fresh
        <!Wakelock!>wakeLock?.acquire()<!>
        old?.release()
    }
}

// With no assignment in the function, `old` is the acquired lock. Go does
// not relate `old` to `wakeLock` and reports the acquire; the lock is
// released.
class AliasOfUnassignedProperty {
    private var wakeLock: WakeLock? = null

    fun run() {
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
