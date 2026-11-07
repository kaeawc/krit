// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15x2
// A project class that reuses the simple name AlarmManager. Go matches the
// call by name alone and reports both arguments of the call. Neither calls
// android.app.AlarmManager: this class schedules no platform alarm, so FIR
// does not report them.
package test.thirdparty

class AlarmManager {
    fun setRepeating(type: Int, triggerAt: Long, intervalMillis: Long, operation: Runnable) {
    }
}

fun schedule(alarmManager: AlarmManager, operation: Runnable) {
    alarmManager.setRepeating(0, 0L, 5000L, operation)
}
