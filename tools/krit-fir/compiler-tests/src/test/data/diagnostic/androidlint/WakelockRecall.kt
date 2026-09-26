// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 57
// Recall cases for Wakelock: WakeLock receivers whose type only resolution
// knows. Every acquire below is on a real android.os.PowerManager.WakeLock
// (or a subtype of a class named WakeLock) and its function never releases
// it, so every one is reported. Go's source inference misses them: it types a
// `PowerManager.WakeLock` annotation as its outer class `PowerManager`, and it
// cannot type a call result, a type alias, a smart cast, a property
// initialized from newWakeLock, or a subclass. The import alias is the one
// case here Go also reports.
package test

import android.os.PowerManager
import android.os.PowerManager.WakeLock as PowerLock

typealias Lock = PowerManager.WakeLock

open class WakeLock {
    fun acquire() {}
    fun release() {}
}

class CountedLock : WakeLock()

class Holder(val powerManager: PowerManager) {
    lateinit var lock: PowerManager.WakeLock

    fun make(): PowerManager.WakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "holder")
}

fun qualifiedParameter(wakeLock: PowerManager.WakeLock) {
    <!Wakelock!>wakeLock.acquire()<!>
}

fun qualifiedProperty(holder: Holder) {
    <!Wakelock!>holder.lock.acquire()<!>
}

fun callResult(holder: Holder) {
    <!Wakelock!>holder.make().acquire()<!>
}

fun directNewWakeLock(powerManager: PowerManager) {
    <!Wakelock!>powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "direct").acquire()<!>
}

class Eager(powerManager: PowerManager) {
    private val lock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "eager")

    fun start() {
        <!Wakelock!>lock.acquire()<!>
    }
}

// Go reports this too: it resolves the import alias.
fun importAlias(lock: PowerLock) {
    <!Wakelock!>lock.acquire()<!>
}

fun aliased(lock: Lock) {
    <!Wakelock!>lock.acquire()<!>
}

fun smartCast(candidate: Any) {
    if (candidate is PowerManager.WakeLock) {
        <!Wakelock!>candidate.acquire()<!>
    }
}

fun subclass(lock: CountedLock) {
    <!Wakelock!>lock.acquire()<!>
}

fun inferredLocal(holder: Holder) {
    val lock = holder.make()
    <!Wakelock!>lock.acquire()<!>
}

// Released through a qualified receiver of the same name: not reported.
fun releasedQualified(holder: Holder) {
    holder.lock.acquire()
    holder.lock.release()
}
