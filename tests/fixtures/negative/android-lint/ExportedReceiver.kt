package com.example

import android.content.BroadcastReceiver
import android.content.Context
import android.content.Intent

class SecureReceiver : BroadcastReceiver() {
    override fun onReceive(context: Context, intent: Intent) {
        context.enforceCallingPermission("com.example.BROADCAST", "No permission")
    }
}

class NotAReceiver {
    fun handle(message: String) {
        println(message)
    }
}
