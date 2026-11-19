// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17x2, 18x2
// Constant arguments Android Lint folds (ConstantEvaluator): a product, a
// sign, a difference, a conversion, arithmetic on a constant, and a local val
// holding one. Go folds `*`, `+`, and a leading `-` on literals, so it reports
// the first two calls too; it does not fold `-` between values, conversions,
// names, or locals. A `var` may be reassigned before the call, so neither Go
// nor FIR follows it.
package test.folded

import android.app.AlarmManager
import android.app.PendingIntent

const val SECOND = 1000L

fun folded(alarmManager: AlarmManager, operation: PendingIntent) {
    alarmManager.setRepeating(AlarmManager.RTC, <!ShortAlarm!>2 * 1000L<!>, <!ShortAlarm!>30 * 1000L<!>, operation)
    alarmManager.setRepeating(AlarmManager.RTC, <!ShortAlarm!>-1L<!>, <!ShortAlarm!>-1L<!>, operation)
    alarmManager.setRepeating(AlarmManager.RTC, <!ShortAlarm!>10000L - 9000L<!>, <!ShortAlarm!>120000L - 100000L<!>, operation)
    alarmManager.setRepeating(AlarmManager.RTC, <!ShortAlarm!>100.toLong()<!>, <!ShortAlarm!>30 * SECOND<!>, operation)
    val trigger = 1000L
    val interval = 30 * SECOND
    alarmManager.setRepeating(AlarmManager.RTC, <!ShortAlarm!>trigger<!>, <!ShortAlarm!>interval<!>, operation)
}

fun notShort(alarmManager: AlarmManager, operation: PendingIntent, base: Long) {
    alarmManager.setRepeating(AlarmManager.RTC, 5 * SECOND, 60 * SECOND, operation)
    alarmManager.setRepeating(AlarmManager.RTC, base * 1000L, base + 1L, operation)
    var interval = 1000L
    interval = 120000L
    alarmManager.setRepeating(AlarmManager.RTC, 6000L, interval, operation)
    val computed = base * 2
    alarmManager.setRepeating(AlarmManager.RTC, 6000L, computed, operation)
}
