package com.example

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat

class Notifier(private val context: Context) {
    fun show() {
        val intent = Intent(context, OpenReceiver::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        NotificationCompat.Builder(context, "updates")
            .setContentTitle("Update ready")
            .setContentIntent(pending)
            .build()
    }
}
