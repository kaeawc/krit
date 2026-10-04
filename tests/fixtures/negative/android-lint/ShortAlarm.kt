package com.example

import android.app.AlarmManager
import android.app.PendingIntent

class SyncScheduler(private val alarmManager: AlarmManager) {
    fun schedule(operation: PendingIntent, triggerAt: Long, interval: Long) {
        alarmManager.setRepeating(AlarmManager.RTC_WAKEUP, triggerAt, 60000L, operation)
        alarmManager.setInexactRepeating(AlarmManager.ELAPSED_REALTIME, triggerAt, AlarmManager.INTERVAL_FIFTEEN_MINUTES, operation)
        alarmManager.setRepeating(AlarmManager.RTC, triggerAt, interval, operation)
        alarmManager.set(AlarmManager.RTC, triggerAt + 5000L, operation)
    }
}
