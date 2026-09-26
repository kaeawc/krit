package com.example

import android.app.AlarmManager
import android.app.PendingIntent

class SyncScheduler(private val alarmManager: AlarmManager) {
    fun schedule(operation: PendingIntent, triggerAt: Long) {
        alarmManager.setRepeating(AlarmManager.RTC_WAKEUP, triggerAt, 5000L, operation)
        alarmManager.setInexactRepeating(AlarmManager.ELAPSED_REALTIME, triggerAt, 30000, operation)
    }
}
