// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
// A statement that starts with `(`. Kotlin ends the first statement at the
// newline, but tree-sitter joins the parenthesized line to the previous call
// as a call suffix, so Go sees one call chain that starts on line 16 and ends
// with the 5 000 ms setRepeating of line 17. Go reports line 16, whose own
// interval is 60 000 ms (not short, so the message is false there), and
// nothing on line 17, whose 5 000 ms interval is short. FIR reads the two
// statements as Kotlin does and reports only line 17.
package test.divergence

import android.app.AlarmManager
import android.app.PendingIntent

fun misparse(alarmManager: AlarmManager, any: Any, operation: PendingIntent) {
    alarmManager.setRepeating(AlarmManager.RTC, 0L, 60000L, operation)
    <!ShortAlarm!>(any as AlarmManager).setRepeating(AlarmManager.RTC, 0L, 5000L, operation)<!>
}
