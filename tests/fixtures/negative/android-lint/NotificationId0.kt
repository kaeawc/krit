package com.example

import android.app.Notification
import android.app.Service
import android.content.Intent
import android.os.IBinder

class SyncService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        startForeground(NOTIFICATION_ID, Notification())
    }
}

private const val NOTIFICATION_ID = 7

class Tracker {
    fun startForeground(id: Int, notification: Notification) {}

    fun track() {
        startForeground(0, Notification())
    }
}
