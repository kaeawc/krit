// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 35, 36, 37, 38, 43, 48
// Lookalikes of AlarmManager.setRepeating / setInexactRepeating. Go matches the
// call by name alone, whatever its receiver, and reports each call below whose
// third positional argument is an integer literal under 60 000. None of them
// passes that argument to AlarmManager, so none sets an alarm's repeat interval
// and the message ("Short alarm interval") is not true of them: FIR does not
// report them.
package test.lookalike

import android.app.AlarmManager
import android.app.PendingIntent

// A project scheduler with the same method names and parameter list.
class JobScheduler {
    fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operation: Any) {
    }

    fun setInexactRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operation: Any) {
    }
}

// A top-level function named like the setter.
fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operation: Any) {
}

// A project extension on AlarmManager with a different parameter list (the
// call below resolves to it, not to the platform member): its third parameter
// is an Int retry count that reaches no alarm, so no interval is set.
fun AlarmManager.setRepeating(triggerAt: Long, delay: Long, retries: Int) {
}

// Go reports each call (the lines in the header); FIR reports none.
fun lookalikes(scheduler: JobScheduler, alarmManager: AlarmManager, operation: PendingIntent) {
    scheduler.setRepeating(0, 0L, 5000L, operation)
    scheduler.setInexactRepeating(0, 0L, 5000L, operation)
    setRepeating(0, 0L, 5000L, operation)
    alarmManager.setRepeating(0L, 0L, 3)
    val anonymous = object {
        fun setInexactRepeating(type: Int, triggerAt: Long, intervalMillis: Long) {
        }
    }
    anonymous.setInexactRepeating(0, 0L, 5000L)
    class Local {
        fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long) {
        }
    }
    Local().setRepeating(0, 0L, 5000L)
}
