// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 23, 25
// Interval spellings. Go parses a literal with underscores, a hex literal, and
// a parenthesized literal, but drops a binary literal and every named
// constant. Each call below sets a repeating alarm to 5 or 10 seconds, which
// is the finding the message describes, so FIR reports the interval argument
// of each.
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
    alarmManager.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>5_000L<!>, operation)
    alarmManager.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>0x1388L<!>, operation)
    alarmManager.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>0b1001110001000L<!>, operation)
    alarmManager.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>(5000L)<!>, operation)
    alarmManager.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>SHORT_INTERVAL<!>, operation)
    alarmManager.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>ALIASED_INTERVAL<!>, operation)
    alarmManager.setRepeating(AlarmManager.RTC, 6000L, <!ShortAlarm!>Intervals.SYNC<!>, operation)
    // A long constant is not short, in either Go or FIR.
    alarmManager.setRepeating(AlarmManager.RTC, 6000L, LONG_INTERVAL, operation)
}
