package com.example

import android.os.PowerManager

class Worker(private val pm: PowerManager) {
    fun start() {
        pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "myapp:worker")
    }
}
