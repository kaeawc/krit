// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 37, 49
// Divergences from Go, which types the receiver from declarations it can read in the file.
package test.wakelocktimeoutdivergence

import android.os.PowerManager
import android.os.PowerManager.WakeLock as Lock

typealias Held = PowerManager.WakeLock

class Source {
    val shared: PowerManager.WakeLock get() = error("stub")
    fun make(manager: PowerManager) = manager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
}

// Go resolves an import alias through the file's imports, so it reports this too.
fun aliasedImport(lock: Lock) {
    <!WakelockTimeout!>lock.acquire()<!>
}

// Go misses a type alias of the class.
fun aliasedType(lock: Held) {
    <!WakelockTimeout!>lock.acquire()<!>
}

// Go cannot type a property of a project class from another declaration or a call result.
fun fromProperty(source: Source) {
    <!WakelockTimeout!>source.shared.acquire()<!>
}

fun fromCall(source: Source, manager: PowerManager) {
    <!WakelockTimeout!>source.make(manager).acquire()<!>
}

// Go reads through `!!` (it reports this) but cannot type a smart-cast receiver.
fun notNull(lock: PowerManager.WakeLock?) {
    <!WakelockTimeout!>lock!!.acquire()<!>
}

fun smartCast(lock: Any) {
    if (lock is PowerManager.WakeLock) {
        <!WakelockTimeout!>lock.acquire()<!>
    }
}

// Go reports a local initialized by newWakeLock; the compiler types it by its initializer.
fun inferred(manager: PowerManager) {
    val lock = manager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
    <!WakelockTimeout!>lock.acquire()<!>
}

// `this` inside a WakeLock extension is a written receiver whose type is the WakeLock.
fun PowerManager.WakeLock.acquireTwice() {
    <!WakelockTimeout!>this.acquire()<!>
}

// An implicit receiver is not a written receiver, so Go skips it, but the call still takes no timeout.
fun PowerManager.WakeLock.acquireImplicit() {
    <!WakelockTimeout!>acquire()<!>
}

fun scoped(lock: PowerManager.WakeLock) {
    with(lock) {
        <!WakelockTimeout!>acquire()<!>
    }
}
