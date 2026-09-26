// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 14, 15, 16, 17, 19, 23, 24, 26, 28, 30, 32, 38, 61, 66, 74, 81
// Positives and negatives for ShortAlarm on android.app.AlarmManager. Go and
// FIR agree on every call here: a repeating alarm whose interval is an
// integer literal between 1 and 59 999 is reported on the call's first line.
package test

import android.app.AlarmManager
import android.app.PendingIntent

class Scheduler(private val alarmManager: AlarmManager, private val nullable: AlarmManager?) {
    fun literals(operation: PendingIntent, triggerAt: Long) {
        <!ShortAlarm!>alarmManager.setRepeating(AlarmManager.RTC_WAKEUP, triggerAt, 5000L, operation)<!>
        <!ShortAlarm!>alarmManager.setInexactRepeating(AlarmManager.ELAPSED_REALTIME, triggerAt, 30000, operation)<!>
        <!ShortAlarm!>alarmManager.setRepeating(AlarmManager.RTC, triggerAt, 1, operation)<!>
        <!ShortAlarm!>alarmManager.setRepeating(AlarmManager.RTC, triggerAt, 59999L, operation)<!>
        <!ShortAlarm!>this.alarmManager.setRepeating(AlarmManager.RTC, triggerAt, 1000L, operation)<!>
        // A comment before the literal hides it from neither Go nor FIR.
        <!ShortAlarm!>alarmManager.setRepeating(AlarmManager.RTC, triggerAt, /* five seconds */ 5000L, operation)<!>
    }

    fun receivers(operation: PendingIntent, triggerAt: Long) {
        <!ShortAlarm!>nullable?.setRepeating(AlarmManager.RTC, triggerAt, 5000L, operation)<!>
        nullable?.let { <!ShortAlarm!>it.setInexactRepeating(AlarmManager.RTC, triggerAt, 5000L, operation)<!> }
        with(alarmManager) {
            <!ShortAlarm!>setRepeating(AlarmManager.RTC, triggerAt, 5000L, operation)<!>
        }
        alarmManager.apply { <!ShortAlarm!>setInexactRepeating(AlarmManager.RTC, triggerAt, 10000L, operation)<!> }
        // A call split over lines reports on its first line.
        <!ShortAlarm!>alarmManager<!>
            .setRepeating(AlarmManager.RTC, triggerAt, 5000L, operation)
        <!ShortAlarm!>alarmManager<!>.setRepeating(
            AlarmManager.RTC,
            triggerAt,
            5000L,
            operation,
        )
        <!ShortAlarm!>nullable<!>
            ?.setInexactRepeating(AlarmManager.RTC, triggerAt, 5000L, operation)
    }

    fun negatives(operation: PendingIntent, triggerAt: Long, interval: Long) {
        alarmManager.setRepeating(AlarmManager.RTC_WAKEUP, triggerAt, 60000L, operation)
        alarmManager.setRepeating(AlarmManager.RTC_WAKEUP, triggerAt, 120000, operation)
        alarmManager.setInexactRepeating(AlarmManager.RTC, triggerAt, AlarmManager.INTERVAL_FIFTEEN_MINUTES, operation)
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, interval, operation)
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, 0L, operation)
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, -5000L, operation)
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, 5 * 1000L, operation)
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, 5000L.coerceAtLeast(interval), operation)
        alarmManager.set(AlarmManager.RTC, triggerAt + 5000L, operation)
        alarmManager.setWindow(AlarmManager.RTC, triggerAt, 5000L, operation)
        alarmManager.setExact(AlarmManager.RTC, 5000L, operation)
    }
}

// Scope shapes: top-level functions, lambdas, objects, companions, and
// anonymous objects all report.
fun topLevel(alarmManager: AlarmManager, operation: PendingIntent) {
    val schedule = {
        <!ShortAlarm!>alarmManager.setRepeating(AlarmManager.RTC, 0L, 5000L, operation)<!>
    }
    schedule()
    val job = object : Runnable {
        override fun run() {
            <!ShortAlarm!>alarmManager.setInexactRepeating(AlarmManager.RTC, 0L, 5000L, operation)<!>
        }
    }
    job.run()
}

object AlarmHolder {
    fun schedule(alarmManager: AlarmManager, operation: PendingIntent) {
        <!ShortAlarm!>alarmManager.setRepeating(AlarmManager.RTC, 0L, 5000L, operation)<!>
    }
}

class WithCompanion {
    companion object {
        fun schedule(alarmManager: AlarmManager, operation: PendingIntent) {
            <!ShortAlarm!>alarmManager.setRepeating(AlarmManager.RTC, 0L, 5000L, operation)<!>
        }
    }
}
