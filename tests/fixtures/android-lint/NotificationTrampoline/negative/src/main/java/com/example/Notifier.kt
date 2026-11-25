package com.example

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat

class Notifier(private val context: Context) {
    fun show() {
        val open = PendingIntent.getActivity(context, 0, Intent(context, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE)
        val log = PendingIntent.getBroadcast(context, 1, Intent(context, LogReceiver::class.java).let { it }, PendingIntent.FLAG_IMMUTABLE)
        val intent = Intent(context, LogReceiver::class.java)
        val quiet = PendingIntent.getBroadcast(context, 2, intent, PendingIntent.FLAG_IMMUTABLE)
        NotificationCompat.Builder(context, "updates")
            .setContentIntent(open)
            .addAction(0, "Log", log)
            .addAction(0, "Quiet", quiet)
            .build()
    }
}
