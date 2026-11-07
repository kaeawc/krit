// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 16, 17, 18, 20, 22, 23, 24x2, 25x2, 29, 30, 32, 34, 37, 41, 45, 67, 72, 80, 87
// Positives and negatives for ShortAlarm on android.app.AlarmManager. Go and
// FIR agree on every call here, as Android Lint does: only the four-parameter
// `setRepeating` is checked; an integer literal trigger time below 5 000 ms or
// interval below 60 000 ms, zero and negative values included, is reported on
// that argument. `setInexactRepeating` is not checked.
package test

import android.app.AlarmManager
import android.app.PendingIntent

class Scheduler(private val alarmManager: AlarmManager, private val nullable: AlarmManager?) {
    fun literals(operation: PendingIntent, triggerAt: Long) {
        alarmManager.setRepeating(AlarmManager.RTC_WAKEUP, triggerAt, <!ShortAlarm!>5000L<!>, operation)
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, <!ShortAlarm!>1<!>, operation)
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, <!ShortAlarm!>59999L<!>, operation)
        this.alarmManager.setRepeating(AlarmManager.RTC, triggerAt, <!ShortAlarm!>1000L<!>, operation)
        // A comment before the literal hides it from neither Go nor FIR.
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, /* five seconds */ <!ShortAlarm!>5000L<!>, operation)
        // Zero and negative values are below the minimum too.
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, <!ShortAlarm!>0L<!>, operation)
        alarmManager.setRepeating(AlarmManager.RTC, <!ShortAlarm!>1000L<!>, 60000L, operation)
        alarmManager.setRepeating(AlarmManager.RTC, <!ShortAlarm!>0L<!>, <!ShortAlarm!>0L<!>, operation)
        alarmManager.setRepeating(AlarmManager.RTC, <!ShortAlarm!>4999<!>, <!ShortAlarm!>59999<!>, operation)
    }

    fun receivers(operation: PendingIntent, triggerAt: Long) {
        nullable?.setRepeating(AlarmManager.RTC, triggerAt, <!ShortAlarm!>5000L<!>, operation)
        nullable?.let { it.setRepeating(AlarmManager.RTC, triggerAt, <!ShortAlarm!>5000L<!>, operation) }
        with(alarmManager) {
            setRepeating(AlarmManager.RTC, triggerAt, <!ShortAlarm!>5000L<!>, operation)
        }
        alarmManager.apply { setRepeating(AlarmManager.RTC, triggerAt, <!ShortAlarm!>10000L<!>, operation) }
        // A call split over lines reports on the argument's line.
        alarmManager
            .setRepeating(AlarmManager.RTC, triggerAt, <!ShortAlarm!>5000L<!>, operation)
        alarmManager.setRepeating(
            AlarmManager.RTC,
            triggerAt,
            <!ShortAlarm!>5000L<!>,
            operation,
        )
        nullable
            ?.setRepeating(AlarmManager.RTC, triggerAt, <!ShortAlarm!>5000L<!>, operation)
    }

    fun negatives(operation: PendingIntent, triggerAt: Long, interval: Long) {
        alarmManager.setRepeating(AlarmManager.RTC_WAKEUP, triggerAt, 60000L, operation)
        alarmManager.setRepeating(AlarmManager.RTC_WAKEUP, triggerAt, 120000, operation)
        alarmManager.setRepeating(AlarmManager.RTC, 5000L, 60000L, operation)
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, interval, operation)
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, 5000L.coerceAtLeast(interval), operation)
        // Lint does not check these methods.
        alarmManager.setInexactRepeating(AlarmManager.ELAPSED_REALTIME, 0L, 30000, operation)
        alarmManager.setInexactRepeating(AlarmManager.RTC, triggerAt, AlarmManager.INTERVAL_FIFTEEN_MINUTES, operation)
        alarmManager.set(AlarmManager.RTC, triggerAt + 5000L, operation)
        alarmManager.setWindow(AlarmManager.RTC, triggerAt, 5000L, operation)
        alarmManager.setExact(AlarmManager.RTC, 5000L, operation)
    }
}

// Scope shapes: top-level functions, lambdas, objects, companions, and
// anonymous objects all report.
fun topLevel(alarmManager: AlarmManager, operation: PendingIntent) {
    val schedule = {
        alarmManager.setRepeating(AlarmManager.RTC, 5000L, <!ShortAlarm!>5000L<!>, operation)
    }
    schedule()
    val job = object : Runnable {
        override fun run() {
            alarmManager.setRepeating(AlarmManager.RTC, 5000L, <!ShortAlarm!>5000L<!>, operation)
        }
    }
    job.run()
}

object AlarmHolder {
    fun schedule(alarmManager: AlarmManager, operation: PendingIntent) {
        alarmManager.setRepeating(AlarmManager.RTC, 5000L, <!ShortAlarm!>5000L<!>, operation)
    }
}

class WithCompanion {
    companion object {
        fun schedule(alarmManager: AlarmManager, operation: PendingIntent) {
            alarmManager.setRepeating(AlarmManager.RTC, 5000L, <!ShortAlarm!>5000L<!>, operation)
        }
    }
}
