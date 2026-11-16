package com.example

import android.app.Notification
import android.app.Service
import android.content.Intent
import android.os.IBinder

class SyncService : Service() {
    override fun onBind(intent: Intent?): IBinder? = null

    override fun onCreate() {
        startForeground(0, Notification())
    }
}
