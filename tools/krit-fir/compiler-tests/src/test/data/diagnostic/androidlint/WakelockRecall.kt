// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 69
// Recall cases for Wakelock: WakeLock receivers whose type only resolution
// knows. Every acquire below is on a real android.os.PowerManager.WakeLock
// (or a subtype of a class named WakeLock) that nothing in the file
// releases, so every one is reported. Go's source inference misses them: it
// types a `PowerManager.WakeLock` annotation as its outer class
// `PowerManager`, and it cannot type a call result, a type alias, a smart
// cast, a property initialized from newWakeLock, or a subclass. The import
// alias is the one case here Go also reports.
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
    lateinit var spare: PowerManager.WakeLock

    fun make(): PowerManager.WakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "holder")
}

class QualifiedProperty {
    lateinit var wakeLock: PowerManager.WakeLock

    fun run() {
        <!Wakelock!>wakeLock.acquire()<!>
    }
}

class HolderProperty(private val holder: Holder) {
    fun run() {
        <!Wakelock!>holder.lock.acquire()<!>
    }
}

class CallResult(private val holder: Holder) {
    fun run() {
        <!Wakelock!>holder.make().acquire()<!>
    }
}

class DirectNewWakeLock(private val powerManager: PowerManager) {
    fun run() {
        <!Wakelock!>powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "direct").acquire()<!>
    }
}

class Eager(powerManager: PowerManager) {
    private val lock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "eager")

    fun start() {
        <!Wakelock!>lock.acquire()<!>
    }
}

// Go reports this too: it resolves the import alias.
class ImportAlias(private val imported: PowerLock) {
    fun run() {
        <!Wakelock!>imported.acquire()<!>
    }
}

class Aliased(private val aliased: Lock) {
    fun run() {
        <!Wakelock!>aliased.acquire()<!>
    }
}

class SmartCast(private val candidate: Any) {
    fun run() {
        if (candidate is PowerManager.WakeLock) {
            <!Wakelock!>candidate.acquire()<!>
        }
    }
}

class Subclass(private val counted: CountedLock) {
    fun run() {
        <!Wakelock!>counted.acquire()<!>
    }
}

class InferredLocal(private val holder: Holder) {
    fun run() {
        val lock = holder.make()
        <!Wakelock!>lock.acquire()<!>
    }
}

// Released through a qualified receiver of the same name: not reported.
class ReleasedQualified(private val holder: Holder) {
    fun run() {
        holder.spare.acquire()
        holder.spare.release()
    }
}

// Go misses these: it cannot type a constructor call or an object expression.
// Each is a subtype of a local class named WakeLock, acquired and never
// released.
fun localClassInstances() {
    open class WakeLock {
        fun acquire() {}
        fun release() {}
    }
    class Child : WakeLock()
    <!Wakelock!>Child().acquire()<!>
    <!Wakelock!>object : WakeLock() {}.acquire()<!>
}
