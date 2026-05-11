package com.example

import android.app.Service
import android.content.Intent
import android.os.IBinder

class SecureService : Service() {
    override fun onBind(intent: Intent): IBinder? {
        enforceCallingPermission("com.example.BIND", "No permission")
        return null
    }
}

class NotAService {
    fun doWork() {}
}
