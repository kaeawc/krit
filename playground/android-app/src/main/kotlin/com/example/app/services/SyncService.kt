package com.example.app.services

import android.app.Service
import android.content.Intent
import android.os.Handler
import android.os.IBinder
import android.os.Looper
import android.util.Log

// Service without proper onBind override returning null
class SyncService : Service() {

    private val handler = Handler(Looper.getMainLooper())
    private val TAG = "SyncService"

    override fun onBind(intent: Intent?): IBinder? {
        return null
    }

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        Log.d(TAG, "SyncService started")

        // HandlerLeak: anonymous Runnable posted to Handler
        handler.postDelayed(object : Runnable {
            override fun run() {
                performSync()
                handler.postDelayed(this, 60000)
            }
        }, 5000)

        return START_STICKY
    }

    // LongMethod
    private fun performSync() {
        Log.d(TAG, "Starting sync...")
        val startTime = System.currentTimeMillis()
        val items = mutableListOf<String>()

        items.add("item1")
        items.add("item2")
        items.add("item3")
        items.add("item4")
        items.add("item5")
        items.add("item6")
        items.add("item7")
        items.add("item8")
        items.add("item9")
        items.add("item10")

        for (item in items) {
            Log.d(TAG, "Syncing: $item")
            try {
                Thread.sleep(100)
            } catch (e: InterruptedException) {
                // EmptyCatchBlock
            }
        }

        val elapsed = System.currentTimeMillis() - startTime
        Log.d(TAG, "Sync completed in ${elapsed}ms")

        if (elapsed > 5000) {
            Log.w(TAG, "Sync took too long")
        }

        val resultCode = 200
        val message = "Sync OK"
        val itemCount = items.size
        val timestamp = System.currentTimeMillis()
        val formattedTime = java.text.SimpleDateFormat("yyyy-MM-dd").format(java.util.Date(timestamp))

        Log.d(TAG, "Result: $resultCode, $message, $itemCount items at $formattedTime")

        val retryCount = 0
        val maxRetries = 3
        val backoffMs = 1000L

        if (resultCode != 200) {
            for (i in 0 until maxRetries) {
                Log.d(TAG, "Retry $i after ${backoffMs * (i + 1)}ms")
                Thread.sleep(backoffMs * (i + 1))
            }
        }

        Log.d(TAG, "Sync finalized")
    }

    // Wakelock: acquiring without releasing
    fun acquireWakeLock() {
        val pm = getSystemService(POWER_SERVICE) as android.os.PowerManager
        val wakeLock = pm.newWakeLock(android.os.PowerManager.PARTIAL_WAKE_LOCK, "app:sync")
        wakeLock.acquire()
    }

    override fun onDestroy() {
        super.onDestroy()
        handler.removeCallbacksAndMessages(null)
    }
}
