// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Intervals Go misses. Go reads the interval's text and parses it as a decimal
// integer, so it drops a literal with underscores (the parse fails), a hex or
// binary literal, a parenthesized literal, and a named constant. Each call
// below sets a repeating alarm to 5 or 10 seconds, which is the finding the
// message describes, so FIR reports them all.
package test.recall

import android.app.AlarmManager
import android.app.PendingIntent

const val SHORT_INTERVAL = 5_000L
const val ALIASED_INTERVAL = SHORT_INTERVAL
const val LONG_INTERVAL = 120_000L

object Intervals {
    const val SYNC = 10000L
}

fun recall(alarmManager: AlarmManager, operation: PendingIntent) {
    <!ShortAlarm!>alarmManager.setRepeating(AlarmManager.RTC, 0L, 5_000L, operation)<!>
    <!ShortAlarm!>alarmManager.setRepeating(AlarmManager.RTC, 0L, 0x1388L, operation)<!>
    <!ShortAlarm!>alarmManager.setRepeating(AlarmManager.RTC, 0L, 0b1001110001000L, operation)<!>
    <!ShortAlarm!>alarmManager.setRepeating(AlarmManager.RTC, 0L, (5000L), operation)<!>
    <!ShortAlarm!>alarmManager.setInexactRepeating(AlarmManager.RTC, 0L, SHORT_INTERVAL, operation)<!>
    <!ShortAlarm!>alarmManager.setInexactRepeating(AlarmManager.RTC, 0L, ALIASED_INTERVAL, operation)<!>
    <!ShortAlarm!>alarmManager.setInexactRepeating(AlarmManager.RTC, 0L, Intervals.SYNC, operation)<!>
    // A long constant is not short, in either Go or FIR.
    alarmManager.setInexactRepeating(AlarmManager.RTC, 0L, LONG_INTERVAL, operation)
}
