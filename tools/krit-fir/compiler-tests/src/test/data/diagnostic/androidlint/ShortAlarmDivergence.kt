// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
// A statement that starts with `(`. Kotlin ends the first statement at the
// newline, but tree-sitter joins the parenthesized line to the previous call
// as a call suffix, so Go sees one call chain from line 15 to line 16. Go
// reports the chain's last call, on line 16, which is the line FIR reports for
// the 5 000 ms setRepeating; it reads the chain as one statement, though, and
// FIR reads the two statements as Kotlin does.
package test.divergence

import android.app.AlarmManager
import android.app.PendingIntent

fun misparse(alarmManager: AlarmManager, any: Any, operation: PendingIntent) {
    alarmManager.setRepeating(AlarmManager.RTC, 6000L, 60000L, operation)
    (any as AlarmManager).setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5000L<!>, operation)
}
