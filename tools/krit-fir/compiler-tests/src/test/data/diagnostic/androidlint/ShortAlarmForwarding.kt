// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 79, 80, 81, 82, 83, 84, 85, 87, 88, 118, 119, 120, 121, 200, 201, 202, 203, 204, 205
// Project functions named like the setter that pass the trigger time or the
// interval on to AlarmManager. Go matches the call by name alone and reports
// every four-argument call in forwarding(), scopes(), and stored() whose
// trigger time or interval is a short literal. Each marked argument reaches a
// platform repeating alarm, so the message is true and FIR reports it too. The
// unmarked calls that Go reports set no alarm value from the literal.
package test.forwarding

import android.app.AlarmManager
import android.app.PendingIntent

// An extension on AlarmManager that passes its interval to the platform
// member. An extension on AlarmManager (or a subtype, or a nullable one) whose
// interval parameter is a Long is reported like the member.
fun AlarmManager.setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operations: Set<PendingIntent>) {
    setRepeating(type, triggerAt, intervalMillis, operations.first())
}

fun AlarmManager?.setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operation: PendingIntent?) {
    this?.setRepeating(type, triggerAt, intervalMillis, operation)
}

// A wrapper class member that passes the interval to AlarmManager.
class AlarmScheduler(private val am: AlarmManager) {
    fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operation: PendingIntent) {
        am.setRepeating(type, triggerAt, intervalMillis, operation)
    }
}

// The interval reaches AlarmManager inside a lambda.
class LambdaScheduler(private val am: AlarmManager) {
    fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operation: PendingIntent) {
        operation.let { am.setRepeating(type, triggerAt, intervalMillis, it) }
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
// alarm. Go reports the call below (line 88); the interval set is an hour, so
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
    lambda: LambdaScheduler,
    facade: SchedulerFacade,
    seconds: SecondsScheduler,
    repeating: RepeatingScheduler,
    hourly: HourlyScheduler,
    op: PendingIntent,
) {
    am.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, setOf(op))
    nullable.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, op)
    sched.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, op)
    lambda.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, op)
    facade.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, op)
    seconds.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>30L<!>, op)
    repeating.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, op)
    // The trigger time goes through a wrapper the same way.
    sched.setRepeating(AlarmManager.RTC, <!ShortAlarm!>1000L<!>, 60000L, op)
    hourly.setRepeating(AlarmManager.RTC, 6000L, 3L, op)
    // Not short, in either Go or FIR.
    sched.setRepeating(AlarmManager.RTC, 6000L, 60000L, op)
}

// A generic extension bounded by AlarmManager.
fun <T : AlarmManager> T.setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operations: List<PendingIntent>) {
    setRepeating(type, triggerAt, intervalMillis, operations.first())
}

// A function that calls itself and never reaches AlarmManager. Go reports the
// call in scopes(); nothing sets an alarm, so FIR does not.
class Recursive {
    fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, retries: Int) {
        if (type > 0) setRepeating(type - 1, triggerAt, intervalMillis, retries)
    }
}

// Local and anonymous wrappers that pass the interval on are reported too.
fun scopes(am: AlarmManager, recursive: Recursive, op: PendingIntent) {
    class LocalScheduler {
        fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, retries: Int) {
            am.setRepeating(type, triggerAt, intervalMillis, op)
        }
    }
    val anonymous = object {
        fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, retries: Int) {
            am.setRepeating(type, triggerAt, intervalMillis, op)
        }
    }
    LocalScheduler().setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, 1)
    anonymous.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, 1)
    am.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, listOf(op))
    recursive.setRepeating(1, 6000L, 5000L, 1)
}

// Wrappers that store the interval parameter in a member or a local before
// passing it to AlarmManager. The value that reaches the platform interval is
// the parameter's, so the literal decides it and FIR reports the calls in
// stored(), like Go.
class MemberScheduler(private val am: AlarmManager) {
    private var interval = 0L
    fun setRepeating(type: Int, at: Long, intervalMillis: Long, op: PendingIntent) {
        interval = intervalMillis
        am.setRepeating(type, at, interval, op)
    }
}

class LocalValScheduler(private val am: AlarmManager) {
    fun setRepeating(type: Int, at: Long, intervalMillis: Long, op: PendingIntent) {
        val millis = intervalMillis
        val scaled = millis * 1L
        am.setRepeating(type, at, scaled, op)
    }
}

// The stored value is replaced by a fixed hour before the platform call. Go
// reports the call in stored(); the interval set is an hour, so the message is
// false and FIR does not report it.
class ReassignedScheduler(private val am: AlarmManager) {
    fun setRepeating(type: Int, at: Long, intervalMillis: Long, op: PendingIntent) {
        var millis = intervalMillis
        millis = 3_600_000L
        am.setRepeating(type, at, millis, op)
    }
}

// A conditional overwrite may leave the parameter's value in place, so the
// literal can still reach the platform interval and FIR reports the call.
class ConditionalScheduler(private val am: AlarmManager) {
    fun setRepeating(type: Int, at: Long, intervalMillis: Long, op: PendingIntent) {
        var millis = intervalMillis
        if (type > 0) millis = 3_600_000L
        am.setRepeating(type, at, millis, op)
    }
}

// The parameter is stored after the platform call, inside a loop: the next
// iteration sets the parameter's interval, so FIR reports the call.
class LoopScheduler(private val am: AlarmManager) {
    fun setRepeating(type: Int, at: Long, intervalMillis: Long, op: PendingIntent) {
        var millis = 3_600_000L
        var attempt = 0
        while (attempt < 2) {
            am.setRepeating(type, at, millis, op)
            millis = intervalMillis
            attempt++
        }
    }
}

// The platform call runs in a lambda, which may run after any write in the
// body, so a write of the parameter anywhere in it counts and FIR reports the
// call.
class DeferredScheduler(private val am: AlarmManager) {
    fun setRepeating(type: Int, at: Long, intervalMillis: Long, op: PendingIntent) {
        var millis = 3_600_000L
        val schedule = { am.setRepeating(type, at, millis, op) }
        millis = intervalMillis
        schedule()
    }
}

fun stored(
    member: MemberScheduler,
    local: LocalValScheduler,
    reassigned: ReassignedScheduler,
    conditional: ConditionalScheduler,
    loop: LoopScheduler,
    deferred: DeferredScheduler,
    op: PendingIntent,
) {
    member.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, op)
    local.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, op)
    reassigned.setRepeating(AlarmManager.RTC, 6000L, 5000L, op)
    conditional.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, op)
    loop.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, op)
    deferred.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, op)
}
