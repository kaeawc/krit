package com.example

import android.os.PowerManager

class SyncService {
    fun performSync(powerManager: PowerManager) {
        val wakeLock = powerManager.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "sync")
        wakeLock.acquire()
        try {
            doWork()
        } finally {
            wakeLock.release()
        }
    }
}
