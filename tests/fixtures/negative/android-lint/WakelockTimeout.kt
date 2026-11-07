package com.example

import android.os.PowerManager

class Worker(private val lock: PowerManager.WakeLock) {
    fun start() {
        lock.acquire(10 * 60 * 1000L)
    }
}
