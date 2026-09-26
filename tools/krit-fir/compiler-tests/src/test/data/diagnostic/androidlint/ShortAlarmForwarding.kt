// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 80, 81, 82, 83, 84, 85, 86, 87, 117, 118, 119, 120
// Project functions named like the setter that pass the interval on to
// AlarmManager. Go matches the call by name alone and reports every call in
// forwarding() and scopes() whose third argument is a short literal. Each
// marked call sets a platform repeating alarm whose interval comes from that
// literal, so the message is true and FIR reports it too. The two unmarked Go
// lines (87, 120) set no alarm interval from the literal.
package test.forwarding

import android.app.AlarmManager
import android.app.PendingIntent
import android.content.Context
import android.content.Intent

// An extension on AlarmManager that passes its interval to the platform
// member. An extension on AlarmManager (or a subtype, or a nullable one) whose
// third parameter is a Long is reported like the member.
fun AlarmManager.setInexactRepeating(type: Int, triggerAt: Long, intervalMillis: Long, context: Context, intent: Intent) {
    setInexactRepeating(type, triggerAt, intervalMillis, PendingIntent.getBroadcast(context, 0, intent, 0))
}

fun AlarmManager?.setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, context: Context, intent: Intent) {
    this?.setRepeating(type, triggerAt, intervalMillis, PendingIntent.getBroadcast(context, 0, intent, 0))
}

// A wrapper class member that passes the interval to AlarmManager.
class AlarmScheduler(private val am: AlarmManager) {
    fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operation: PendingIntent) {
        am.setRepeating(type, triggerAt, intervalMillis, operation)
    }

    // The interval reaches AlarmManager inside a lambda.
    fun setInexactRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operation: PendingIntent) {
        operation.let { am.setInexactRepeating(type, triggerAt, intervalMillis, it) }
    }
}

// A wrapper of a wrapper, with an expression body and named arguments.
class SchedulerFacade(private val scheduler: AlarmScheduler) {
    fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operation: PendingIntent) =
        scheduler.setRepeating(operation = operation, intervalMillis = intervalMillis, type = type, triggerAt = triggerAt)
}

// A wrapper whose interval is in seconds: the platform interval is computed
// from the parameter, so the literal still decides it.
class SecondsScheduler(private val am: AlarmManager) {
    fun setRepeating(type: Int, triggerAt: Long, intervalSeconds: Long, operation: PendingIntent) {
        am.setRepeating(type, triggerAt, intervalSeconds * 1000L, operation)
    }
}

// An interface member has no body to read, so FIR cannot tell whether the
// interval reaches AlarmManager; it matches Go and reports the call.
interface RepeatingScheduler {
    fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operation: PendingIntent)
}

// A wrapper that ignores its third parameter and schedules a fixed hourly
// alarm. Go reports the call below (line 87); the interval set is an hour, so
// the message is false and FIR does not report it.
class HourlyScheduler(private val am: AlarmManager) {
    fun setRepeating(type: Int, triggerAt: Long, retries: Long, operation: PendingIntent) {
        am.setRepeating(type, triggerAt, AlarmManager.INTERVAL_HOUR, operation)
    }
}

fun forwarding(
    am: AlarmManager,
    nullable: AlarmManager?,
    sched: AlarmScheduler,
    facade: SchedulerFacade,
    seconds: SecondsScheduler,
    repeating: RepeatingScheduler,
    hourly: HourlyScheduler,
    context: Context,
    intent: Intent,
    op: PendingIntent,
) {
    <!ShortAlarm!>am.setInexactRepeating(AlarmManager.RTC, 0L, 5000L, context, intent)<!>
    <!ShortAlarm!>nullable.setRepeating(AlarmManager.RTC, 0L, 5000L, context, intent)<!>
    <!ShortAlarm!>sched.setRepeating(AlarmManager.RTC, 0L, 5000L, op)<!>
    <!ShortAlarm!>sched.setInexactRepeating(AlarmManager.RTC, 0L, 5000L, op)<!>
    <!ShortAlarm!>facade.setRepeating(AlarmManager.RTC, 0L, 5000L, op)<!>
    <!ShortAlarm!>seconds.setRepeating(AlarmManager.RTC, 0L, 30L, op)<!>
    <!ShortAlarm!>repeating.setRepeating(AlarmManager.RTC, 0L, 5000L, op)<!>
    hourly.setRepeating(AlarmManager.RTC, 0L, 3L, op)
    // Not short, in either Go or FIR.
    sched.setRepeating(AlarmManager.RTC, 0L, 60000L, op)
}

// A generic extension bounded by AlarmManager.
fun <T : AlarmManager> T.setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, context: Context, intent: Intent, requestCode: Int) {
    setRepeating(type, triggerAt, intervalMillis, PendingIntent.getBroadcast(context, requestCode, intent, 0))
}

// A function that calls itself and never reaches AlarmManager. Go reports the
// call in scopes() (line 120); nothing sets an alarm, so FIR does not.
class Recursive {
    fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long) {
        if (type > 0) setRepeating(type - 1, triggerAt, intervalMillis)
    }
}

// Local and anonymous wrappers that pass the interval on are reported too.
fun scopes(am: AlarmManager, recursive: Recursive, context: Context, intent: Intent, op: PendingIntent) {
    class LocalScheduler {
        fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long) {
            am.setRepeating(type, triggerAt, intervalMillis, op)
        }
    }
    val anonymous = object {
        fun setInexactRepeating(type: Int, triggerAt: Long, intervalMillis: Long) {
            am.setInexactRepeating(type, triggerAt, intervalMillis, op)
        }
    }
    <!ShortAlarm!>LocalScheduler().setRepeating(AlarmManager.RTC, 0L, 5000L)<!>
    <!ShortAlarm!>anonymous.setInexactRepeating(AlarmManager.RTC, 0L, 5000L)<!>
    <!ShortAlarm!>am.setRepeating(AlarmManager.RTC, 0L, 5000L, context, intent, 1)<!>
    recursive.setRepeating(1, 0L, 5000L)
}
