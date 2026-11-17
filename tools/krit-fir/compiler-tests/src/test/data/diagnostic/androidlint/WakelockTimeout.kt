// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 15, 19, 24, 28, 32, 36, 40, 45
// Positives for WakelockTimeout: `acquire()` with no timeout on a PowerManager.WakeLock. The finding sits on the
// first line of the call expression, receiver included, where Go reports it.
package test.wakelocktimeout

import android.os.PowerManager

class Worker(private val lock: PowerManager.WakeLock, private val manager: PowerManager) {
    fun field() {
        <!WakelockTimeout!>lock.acquire()<!>
    }

    fun thisQualified() {
        <!WakelockTimeout!>this.lock.acquire()<!>
    }

    fun parameter(held: PowerManager.WakeLock) {
        <!WakelockTimeout!>held.acquire()<!>
    }

    fun local() {
        val created = manager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
        <!WakelockTimeout!>created.acquire()<!>
    }

    fun chain() {
        <!WakelockTimeout!>manager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag").acquire()<!>
    }

    fun safeCall(maybe: PowerManager.WakeLock?) {
        <!WakelockTimeout!>maybe?.acquire()<!>
    }

    fun inLambda() {
        run { <!WakelockTimeout!>lock.acquire()<!> }
    }

    fun multiLine() {
        <!WakelockTimeout!>manager<!>
            .newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "tag")
            .acquire()
    }

    fun asExpression(): Unit = <!WakelockTimeout!>lock.acquire()<!>
}
