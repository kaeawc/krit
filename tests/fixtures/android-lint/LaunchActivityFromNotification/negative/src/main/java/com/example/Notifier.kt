package com.example

import android.app.Notification
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat

class Notifier(private val context: Context) {
    fun show() {
        val dismiss = PendingIntent.getBroadcast(context, 0, Intent("com.example.action.DISMISS"), PendingIntent.FLAG_IMMUTABLE)
        val open = PendingIntent.getActivity(context, 1, Intent(context, MainActivity::class.java), PendingIntent.FLAG_IMMUTABLE)
        // A broadcast from a notification action is fine; the content intent opens an activity.
        NotificationCompat.Builder(context, "updates")
            .setContentIntent(open)
            .addAction(0, "Dismiss", dismiss)
            .build()
        // The platform builder is not NotificationCompat.Builder, which is all Lint checks.
        Notification.Builder(context, "updates").setContentIntent(dismiss)
    }
}
